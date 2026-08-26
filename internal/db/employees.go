package db

import (
	"database/sql"
)

type Candidate struct {
	ID                 string
	FullName           string
	Role               sql.NullString
	IDType             string
	IDNumber           sql.NullString
	CheckMethod        string // dashboard_lookup | hosted_link
	IdentifyRaw        sql.NullString
	KYCFlowID          sql.NullString
	KYCVerificationID  sql.NullString
	KYCVerificationURL sql.NullString
	KYCOutcome         sql.NullString
	KYCScore           sql.NullFloat64
	KYCRaw             sql.NullString
	MonthlySalaryKobo  int64
	PayrollStatus      string
	Status             string
	CreatedAt          string
	UpdatedAt          string
}

const candidateColumns = `
	id, full_name, role, id_type, id_number, check_method, identify_raw,
	kyc_flow_id, kyc_verification_id, kyc_verification_url, kyc_outcome, kyc_score, kyc_raw,
	monthly_salary_kobo, payroll_status, status, created_at, updated_at
`

func scanCandidate(row interface{ Scan(dest ...any) error }) (*Candidate, error) {
	var c Candidate
	err := row.Scan(
		&c.ID, &c.FullName, &c.Role, &c.IDType, &c.IDNumber, &c.CheckMethod, &c.IdentifyRaw,
		&c.KYCFlowID, &c.KYCVerificationID, &c.KYCVerificationURL, &c.KYCOutcome, &c.KYCScore, &c.KYCRaw,
		&c.MonthlySalaryKobo, &c.PayrollStatus, &c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func InsertCandidate(conn *sql.DB, id, fullName, role, idType, idNumber, checkMethod string) error {
	_, err := conn.Exec(`
		INSERT INTO candidates (id, full_name, role, id_type, id_number, check_method, monthly_salary_kobo, payroll_status, status)
		VALUES (?, ?, ?, ?, ?, ?, 85000000, 'locked', 'pending')
	`, id, fullName, role, idType, idNumber, checkMethod)
	return err
}

func GetCandidate(conn *sql.DB, id string) (*Candidate, error) {
	row := conn.QueryRow(`SELECT `+candidateColumns+` FROM candidates WHERE id = ?`, id)
	return scanCandidate(row)
}

func ListCandidates(conn *sql.DB) ([]*Candidate, error) {
	rows, err := conn.Query(`SELECT ` + candidateColumns + ` FROM candidates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Candidate
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func UpdateCandidateDashboardResult(conn *sql.DB, id, status, identifyRawJSON string) error {
	payroll := "locked"
	if status == "cleared" || status == "active" {
		payroll = "active"
	}
	_, err := conn.Exec(`
		UPDATE candidates SET status = ?, identify_raw = ?, payroll_status = ?, updated_at = datetime('now') WHERE id = ?
	`, status, identifyRawJSON, payroll, id)
	return err
}

func UpdateCandidateKYCLink(conn *sql.DB, id, flowID, verificationID, verificationURL string) error {
	_, err := conn.Exec(`
		UPDATE candidates SET kyc_flow_id = ?, kyc_verification_id = ?, kyc_verification_url = ?, updated_at = datetime('now')
		WHERE id = ?
	`, flowID, verificationID, verificationURL, id)
	return err
}

func UpdateCandidateKYCOutcome(conn *sql.DB, verificationID, outcome string, score float64, rawJSON string) error {
	status := "mismatch"
	payroll := "locked"
	if outcome == "pass" || outcome == "passed" || outcome == "approved" || outcome == "verified" || outcome == "success" {
		status = "cleared"
		payroll = "active"
	}
	_, err := conn.Exec(`
		UPDATE candidates SET kyc_outcome = ?, kyc_score = ?, kyc_raw = ?, status = ?, payroll_status = ?, updated_at = datetime('now')
		WHERE kyc_verification_id = ?
	`, outcome, score, rawJSON, status, payroll, verificationID)
	return err
}

func UpdateCandidatePayrollStatus(conn *sql.DB, id, status string) error {
	_, err := conn.Exec(`UPDATE candidates SET payroll_status = ?, updated_at = datetime('now') WHERE id = ?`, status, id)
	return err
}

func GetCandidateByKYCVerificationID(conn *sql.DB, verificationID string) (*Candidate, error) {
	row := conn.QueryRow(`SELECT `+candidateColumns+` FROM candidates WHERE kyc_verification_id = ?`, verificationID)
	return scanCandidate(row)
}
