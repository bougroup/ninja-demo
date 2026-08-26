package db

import (
	"database/sql"
)

type Aggregator struct {
	ID                 string
	BusinessName       string
	RCNumber           string
	ContactName        string
	ContactEmail       string
	CompanyLookupRaw   sql.NullString
	KYBFlowID          sql.NullString
	KYBVerificationID  sql.NullString
	KYBVerificationURL sql.NullString
	KYBOutcome         sql.NullString
	KYBRaw             sql.NullString
	Status             string
	CreatedAt          string
	UpdatedAt          string
}

const aggregatorColumns = `
	id, business_name, rc_number, contact_name, contact_email, company_lookup_raw,
	kyb_flow_id, kyb_verification_id, kyb_verification_url, kyb_outcome, kyb_raw,
	status, created_at, updated_at
`

func scanAggregator(row interface{ Scan(dest ...any) error }) (*Aggregator, error) {
	var a Aggregator
	err := row.Scan(
		&a.ID, &a.BusinessName, &a.RCNumber, &a.ContactName, &a.ContactEmail, &a.CompanyLookupRaw,
		&a.KYBFlowID, &a.KYBVerificationID, &a.KYBVerificationURL, &a.KYBOutcome, &a.KYBRaw,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func InsertAggregator(conn *sql.DB, id, businessName, rcNumber, contactName, contactEmail string) error {
	_, err := conn.Exec(`
		INSERT INTO aggregators (id, business_name, rc_number, contact_name, contact_email, status)
		VALUES (?, ?, ?, ?, ?, 'applied')
	`, id, businessName, rcNumber, contactName, contactEmail)
	return err
}

func GetAggregator(conn *sql.DB, id string) (*Aggregator, error) {
	row := conn.QueryRow(`SELECT `+aggregatorColumns+` FROM aggregators WHERE id = ?`, id)
	return scanAggregator(row)
}

func ListAggregators(conn *sql.DB) ([]*Aggregator, error) {
	rows, err := conn.Query(`SELECT ` + aggregatorColumns + ` FROM aggregators ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Aggregator
	for rows.Next() {
		a, err := scanAggregator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func UpdateAggregatorCompanyLookup(conn *sql.DB, id, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE aggregators SET company_lookup_raw = ?, status = 'company_verified', updated_at = datetime('now') WHERE id = ?
	`, rawJSON, id)
	return err
}

func UpdateAggregatorKYBFlowLink(conn *sql.DB, id, flowID, verificationID, verificationURL string) error {
	_, err := conn.Exec(`
		UPDATE aggregators SET kyb_flow_id = ?, kyb_verification_id = ?, kyb_verification_url = ?, status = 'kyb_pending', updated_at = datetime('now')
		WHERE id = ?
	`, flowID, verificationID, verificationURL, id)
	return err
}

func UpdateAggregatorKYBOutcome(conn *sql.DB, verificationID, outcome, status, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE aggregators SET kyb_outcome = ?, status = ?, kyb_raw = ?, updated_at = datetime('now') WHERE kyb_verification_id = ?
	`, outcome, status, rawJSON, verificationID)
	return err
}

func GetAggregatorByKYBVerificationID(conn *sql.DB, verificationID string) (*Aggregator, error) {
	row := conn.QueryRow(`SELECT `+aggregatorColumns+` FROM aggregators WHERE kyb_verification_id = ?`, verificationID)
	return scanAggregator(row)
}

type Agent struct {
	ID               string
	FullName         string
	DateOfBirth      sql.NullString
	IDType           string
	IDNumber         string
	TerminalID       string
	AggregatorID     sql.NullString
	IdentifyRaw      sql.NullString
	Status           string
	POSTerminalCount int
	CashFloatKobo    int64
	LastReverifiedAt sql.NullString
	CreatedAt        string
}

const agentColumns = `
	id, full_name, date_of_birth, id_type, id_number, terminal_id, aggregator_id,
	identify_raw, status, pos_terminal_count, cash_float_kobo, last_reverified_at, created_at
`

func scanAgent(row interface{ Scan(dest ...any) error }) (*Agent, error) {
	var a Agent
	err := row.Scan(
		&a.ID, &a.FullName, &a.DateOfBirth, &a.IDType, &a.IDNumber, &a.TerminalID, &a.AggregatorID,
		&a.IdentifyRaw, &a.Status, &a.POSTerminalCount, &a.CashFloatKobo, &a.LastReverifiedAt, &a.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CountAgentsByIdentity powers the "one BVN behind fifty terminals" dedupe
// check: how many terminals are already tied to this id_number.
func CountAgentsByIdentity(conn *sql.DB, idType, idNumber string) (int, error) {
	var n int
	err := conn.QueryRow(`SELECT COUNT(*) FROM agents WHERE id_type = ? AND id_number = ?`, idType, idNumber).Scan(&n)
	return n, err
}

func ListAgentsByIdentity(conn *sql.DB, idType, idNumber string) ([]*Agent, error) {
	rows, err := conn.Query(`SELECT `+agentColumns+` FROM agents WHERE id_type = ? AND id_number = ? ORDER BY created_at DESC`, idType, idNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func InsertAgent(conn *sql.DB, id, fullName, dob, idType, idNumber, terminalID, status, identifyRawJSON string) error {
	count, _ := CountAgentsByIdentity(conn, idType, idNumber)
	terminalCount := count + 1
	_, err := conn.Exec(`
		INSERT INTO agents (id, full_name, date_of_birth, id_type, id_number, terminal_id, status, identify_raw, pos_terminal_count, cash_float_kobo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 200000000)
	`, id, fullName, dob, idType, idNumber, terminalID, status, identifyRawJSON, terminalCount)
	return err
}

func GetAgent(conn *sql.DB, id string) (*Agent, error) {
	row := conn.QueryRow(`SELECT `+agentColumns+` FROM agents WHERE id = ?`, id)
	return scanAgent(row)
}

func ListAgents(conn *sql.DB) ([]*Agent, error) {
	rows, err := conn.Query(`SELECT ` + agentColumns + ` FROM agents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func UpdateAgentReverification(conn *sql.DB, id, status, identifyRawJSON string) error {
	_, err := conn.Exec(`
		UPDATE agents SET status = ?, identify_raw = ?, last_reverified_at = datetime('now') WHERE id = ?
	`, status, identifyRawJSON, id)
	return err
}

func UpdateAgentStatus(conn *sql.DB, id, status string) error {
	_, err := conn.Exec(`UPDATE agents SET status = ? WHERE id = ?`, status, id)
	return err
}
