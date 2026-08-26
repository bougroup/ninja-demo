package db

import (
	"database/sql"
)

type Director struct {
	ID            string
	VendorID      string
	CACDirectorID sql.NullString
	FullName      sql.NullString
	IDType        sql.NullString
	IDNumber      sql.NullString

	BulkIdentifyStatus sql.NullString
	BulkIdentifyRaw    sql.NullString

	KYCFlowID          sql.NullString
	KYCVerificationID  sql.NullString
	KYCVerificationURL sql.NullString
	KYCOutcome         sql.NullString
	KYCScore           sql.NullFloat64
	KYCRaw             sql.NullString

	CreatedAt string
	UpdatedAt string
}

const directorColumns = `
	id, vendor_id, cac_director_id, full_name, id_type, id_number,
	bulk_identify_status, bulk_identify_raw,
	kyc_flow_id, kyc_verification_id, kyc_verification_url, kyc_outcome, kyc_score, kyc_raw,
	created_at, updated_at
`

func scanDirector(row interface{ Scan(dest ...any) error }) (*Director, error) {
	var d Director
	err := row.Scan(
		&d.ID, &d.VendorID, &d.CACDirectorID, &d.FullName, &d.IDType, &d.IDNumber,
		&d.BulkIdentifyStatus, &d.BulkIdentifyRaw,
		&d.KYCFlowID, &d.KYCVerificationID, &d.KYCVerificationURL, &d.KYCOutcome, &d.KYCScore, &d.KYCRaw,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func InsertDirector(conn *sql.DB, id, vendorID, cacDirectorID, fullName, idType, idNumber string) error {
	_, err := conn.Exec(`
		INSERT INTO directors (id, vendor_id, cac_director_id, full_name, id_type, id_number)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, vendorID, cacDirectorID, fullName, idType, idNumber)
	return err
}

func ListDirectorsByVendor(conn *sql.DB, vendorID string) ([]*Director, error) {
	rows, err := conn.Query(`SELECT `+directorColumns+` FROM directors WHERE vendor_id = ? ORDER BY created_at ASC`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Director
	for rows.Next() {
		d, err := scanDirector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func GetDirector(conn *sql.DB, id string) (*Director, error) {
	row := conn.QueryRow(`SELECT `+directorColumns+` FROM directors WHERE id = ?`, id)
	return scanDirector(row)
}

func UpdateDirectorBulkIdentify(conn *sql.DB, id, status, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE directors SET bulk_identify_status = ?, bulk_identify_raw = ?, updated_at = datetime('now')
		WHERE id = ?
	`, status, rawJSON, id)
	return err
}

func UpdateDirectorKYCLink(conn *sql.DB, id, flowID, verificationID, verificationURL string) error {
	_, err := conn.Exec(`
		UPDATE directors SET kyc_flow_id = ?, kyc_verification_id = ?, kyc_verification_url = ?, updated_at = datetime('now')
		WHERE id = ?
	`, flowID, verificationID, verificationURL, id)
	return err
}

func UpdateDirectorKYCOutcome(conn *sql.DB, verificationID, outcome string, score float64, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE directors SET kyc_outcome = ?, kyc_score = ?, kyc_raw = ?, updated_at = datetime('now')
		WHERE kyc_verification_id = ?
	`, outcome, score, rawJSON, verificationID)
	return err
}

func GetDirectorByKYCVerificationID(conn *sql.DB, verificationID string) (*Director, error) {
	row := conn.QueryRow(`SELECT `+directorColumns+` FROM directors WHERE kyc_verification_id = ?`, verificationID)
	return scanDirector(row)
}

// AllDirectorsPassed reports whether every director for a vendor has a
// passing KYC outcome (used to decide payout eligibility alongside the KYB
// outcome).
func AllDirectorsPassed(conn *sql.DB, vendorID string) (bool, error) {
	var total, passed int
	err := conn.QueryRow(`SELECT COUNT(*) FROM directors WHERE vendor_id = ?`, vendorID).Scan(&total)
	if err != nil {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	err = conn.QueryRow(`SELECT COUNT(*) FROM directors WHERE vendor_id = ? AND kyc_outcome = 'pass'`, vendorID).Scan(&passed)
	if err != nil {
		return false, err
	}
	return passed == total, nil
}
