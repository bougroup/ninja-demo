package db

import (
	"database/sql"
)

type FintechCustomer struct {
	ID             string
	FullName       string
	DateOfBirth    string
	IDType         string
	IDNumber       string
	Score          sql.NullFloat64
	Recommendation sql.NullString
	MismatchesRaw  sql.NullString
	FieldsRaw      sql.NullString
	Tier           int
	DailyLimitKobo int64
	BalanceKobo    int64
	Status         string
	LastCheckedAt  string
	CreatedAt      string
}

const fintechCustomerColumns = `
	id, full_name, date_of_birth, id_type, id_number, score, recommendation,
	mismatches_raw, fields_raw, tier, daily_limit_kobo, balance_kobo, status, last_checked_at, created_at
`

func scanFintechCustomer(row interface{ Scan(dest ...any) error }) (*FintechCustomer, error) {
	var c FintechCustomer
	err := row.Scan(
		&c.ID, &c.FullName, &c.DateOfBirth, &c.IDType, &c.IDNumber, &c.Score, &c.Recommendation,
		&c.MismatchesRaw, &c.FieldsRaw, &c.Tier, &c.DailyLimitKobo, &c.BalanceKobo, &c.Status, &c.LastCheckedAt, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func InsertFintechCustomer(conn *sql.DB, id, fullName, dob, idType, idNumber string, score float64, recommendation, mismatchesJSON, fieldsJSON, status string) error {
	tier := 1
	dailyLimit := int64(5000000) // ₦50,000 for Tier 1
	if score >= 0.90 {
		tier = 2
		dailyLimit = 50000000 // ₦500,000 for Tier 2
	}
	_, err := conn.Exec(`
		INSERT INTO fintech_customers (id, full_name, date_of_birth, id_type, id_number, score, recommendation, mismatches_raw, fields_raw, tier, daily_limit_kobo, balance_kobo, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 125000000, ?)
	`, id, fullName, dob, idType, idNumber, score, recommendation, mismatchesJSON, fieldsJSON, tier, dailyLimit, status)
	return err
}

func GetFintechCustomer(conn *sql.DB, id string) (*FintechCustomer, error) {
	row := conn.QueryRow(`SELECT `+fintechCustomerColumns+` FROM fintech_customers WHERE id = ?`, id)
	return scanFintechCustomer(row)
}

func ListFintechCustomers(conn *sql.DB) ([]*FintechCustomer, error) {
	rows, err := conn.Query(`SELECT ` + fintechCustomerColumns + ` FROM fintech_customers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FintechCustomer
	for rows.Next() {
		c, err := scanFintechCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func UpdateFintechCustomerRecheck(conn *sql.DB, id string, score float64, recommendation, mismatchesJSON, fieldsJSON, status string) error {
	tier := 1
	dailyLimit := int64(5000000)
	if score >= 0.80 {
		tier = 2
		dailyLimit = 50000000
	}
	if score >= 0.95 {
		tier = 3
		dailyLimit = 500000000
	}
	_, err := conn.Exec(`
		UPDATE fintech_customers SET
			score = ?, recommendation = ?, mismatches_raw = ?, fields_raw = ?, tier = ?, daily_limit_kobo = ?, status = ?, last_checked_at = datetime('now')
		WHERE id = ?
	`, score, recommendation, mismatchesJSON, fieldsJSON, tier, dailyLimit, status, id)
	return err
}

func UpdateFintechCustomerTier(conn *sql.DB, id string, tier int, dailyLimitKobo int64) error {
	_, err := conn.Exec(`UPDATE fintech_customers SET tier = ?, daily_limit_kobo = ? WHERE id = ?`, tier, dailyLimitKobo, id)
	return err
}

type FintechTransfer struct {
	ID            string
	CustomerID    string
	RecipientName string
	RecipientBank string
	RecipientAcct string
	AmountKobo    int64
	Status        string
	Reason        sql.NullString
	CreatedAt     string
}

func InsertFintechTransfer(conn *sql.DB, id, customerID, recipientName, recipientBank, recipientAcct string, amountKobo int64, status, reason string) error {
	_, err := conn.Exec(`
		INSERT INTO fintech_transfers (id, customer_id, recipient_name, recipient_bank, recipient_acct, amount_kobo, status, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, customerID, recipientName, recipientBank, recipientAcct, amountKobo, status, reason)
	return err
}

func ListFintechTransfersByCustomer(conn *sql.DB, customerID string) ([]*FintechTransfer, error) {
	rows, err := conn.Query(`
		SELECT id, customer_id, recipient_name, recipient_bank, recipient_acct, amount_kobo, status, reason, created_at
		FROM fintech_transfers WHERE customer_id = ? ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FintechTransfer
	for rows.Next() {
		var t FintechTransfer
		if err := rows.Scan(&t.ID, &t.CustomerID, &t.RecipientName, &t.RecipientBank, &t.RecipientAcct, &t.AmountKobo, &t.Status, &t.Reason, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}
