package db

import (
	"database/sql"
)

type User struct {
	ID             string
	FirstName      string
	LastName       string
	Email          string
	PasswordHash   string
	Mobile         string
	EmailVerified  bool
	KYCStatus      string // unverified, verified, blocked_underage, blocked_mismatch, blocked_duplicate, blocked_self_excluded
	IDType         string // bvn, nin
	IDNumber       string
	DateOfBirth    string
	Age            int
	KYCScore       float64
	IdentifyRaw    sql.NullString
	LivenessStatus string
	BalanceKobo    int64
	WinningsKobo   int64
	SelfExcluded   bool
	CreatedAt      string
}

func (u *User) FullName() string {
	if u == nil {
		return "Guest Player"
	}
	return u.FirstName + " " + u.LastName
}

const userColumns = `id, first_name, last_name, email, password_hash, COALESCE(mobile, ''), email_verified, kyc_status, COALESCE(id_type, ''), COALESCE(id_number, ''), COALESCE(date_of_birth, ''), age, kyc_score, identify_raw, COALESCE(liveness_status, 'none'), balance_kobo, winnings_kobo, self_excluded, created_at`

func scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	var u User
	var ev, se int
	err := row.Scan(
		&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.PasswordHash, &u.Mobile,
		&ev, &u.KYCStatus, &u.IDType, &u.IDNumber, &u.DateOfBirth, &u.Age,
		&u.KYCScore, &u.IdentifyRaw, &u.LivenessStatus, &u.BalanceKobo,
		&u.WinningsKobo, &se, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.EmailVerified = ev == 1
	u.SelfExcluded = se == 1
	return &u, nil
}

func CreateUser(conn *sql.DB, id, firstName, lastName, email, passwordHash, mobile string) error {
	_, err := conn.Exec(`
		INSERT INTO users (id, first_name, last_name, email, password_hash, mobile, email_verified, kyc_status, balance_kobo, winnings_kobo)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'unverified', 10000000, 0)
	`, id, firstName, lastName, email, passwordHash, mobile)
	return err
}

func GetUserByEmail(conn *sql.DB, email string) (*User, error) {
	row := conn.QueryRow(`SELECT `+userColumns+` FROM users WHERE LOWER(email) = LOWER(?)`, email)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func GetUser(conn *sql.DB, id string) (*User, error) {
	row := conn.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func GetLatestUser(conn *sql.DB) (*User, error) {
	row := conn.QueryRow(`SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC LIMIT 1`)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func UpdateUserKYC(conn *sql.DB, id, status, idType, idNumber, dob string, age int, score float64, identifyRaw string) error {
	_, err := conn.Exec(`
		UPDATE users SET
			kyc_status = ?,
			id_type = ?,
			id_number = ?,
			date_of_birth = ?,
			age = ?,
			kyc_score = ?,
			identify_raw = ?
		WHERE id = ?
	`, status, idType, idNumber, dob, age, score, identifyRaw, id)
	return err
}

func UpdateUserBalance(conn *sql.DB, id string, balanceKobo, winningsKobo int64) error {
	_, err := conn.Exec(`UPDATE users SET balance_kobo = ?, winnings_kobo = ? WHERE id = ?`, balanceKobo, winningsKobo, id)
	return err
}

func SetUserSelfExcluded(conn *sql.DB, id string) error {
	_, err := conn.Exec(`UPDATE users SET self_excluded = 1, kyc_status = 'blocked_self_excluded' WHERE id = ?`, id)
	return err
}

func InsertEmailLog(conn *sql.DB, id, toEmail, subject, bodyHTML, status string) error {
	_, err := conn.Exec(`
		INSERT INTO email_logs (id, to_email, subject, body_html, status)
		VALUES (?, ?, ?, ?, ?)
	`, id, toEmail, subject, bodyHTML, status)
	return err
}

func ListEmailLogs(conn *sql.DB, limit int) ([]map[string]any, error) {
	rows, err := conn.Query(`SELECT id, to_email, subject, status, created_at FROM email_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []map[string]any
	for rows.Next() {
		var id, toEmail, subject, status, createdAt string
		if err := rows.Scan(&id, &toEmail, &subject, &status, &createdAt); err == nil {
			logs = append(logs, map[string]any{
				"ID":        id,
				"ToEmail":   toEmail,
				"Subject":   subject,
				"Status":    status,
				"CreatedAt": createdAt,
			})
		}
	}
	return logs, nil
}
