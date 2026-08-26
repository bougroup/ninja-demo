package db

import (
	"database/sql"
)

type Vendor struct {
	ID                string
	BusinessName      string
	RCNumber          string
	ContactName       string
	ContactEmail      string
	Status            string
	PayoutWalletID    sql.NullString
	EscrowBalanceKobo int64
	EscrowStatus      string

	CompanyRegistrationNumber sql.NullString
	CompanyStatus             sql.NullString
	CompanyType               sql.NullString
	CompanyNatureOfBusiness   sql.NullString
	CompanyEmail              sql.NullString
	CompanyAddress            sql.NullString
	CompanyLookupRaw          sql.NullString
	CompanyAdvancedRaw        sql.NullString

	KYBFlowID          sql.NullString
	KYBVerificationID  sql.NullString
	KYBVerificationURL sql.NullString
	KYBOutcome         sql.NullString
	KYBRaw             sql.NullString

	CreatedAt string
	UpdatedAt string
}

const vendorColumns = `
	id, business_name, rc_number, contact_name, contact_email, status, payout_wallet_id,
	escrow_balance_kobo, escrow_status,
	company_registration_number, company_status, company_type, company_nature_of_business,
	company_email, company_address, company_lookup_raw, company_advanced_raw,
	kyb_flow_id, kyb_verification_id, kyb_verification_url, kyb_outcome, kyb_raw,
	created_at, updated_at
`

func scanVendor(row interface {
	Scan(dest ...any) error
}) (*Vendor, error) {
	var v Vendor
	err := row.Scan(
		&v.ID, &v.BusinessName, &v.RCNumber, &v.ContactName, &v.ContactEmail, &v.Status, &v.PayoutWalletID,
		&v.EscrowBalanceKobo, &v.EscrowStatus,
		&v.CompanyRegistrationNumber, &v.CompanyStatus, &v.CompanyType, &v.CompanyNatureOfBusiness,
		&v.CompanyEmail, &v.CompanyAddress, &v.CompanyLookupRaw, &v.CompanyAdvancedRaw,
		&v.KYBFlowID, &v.KYBVerificationID, &v.KYBVerificationURL, &v.KYBOutcome, &v.KYBRaw,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func InsertVendor(conn *sql.DB, id, businessName, rcNumber, contactName, contactEmail string) error {
	_, err := conn.Exec(`
		INSERT INTO vendors (id, business_name, rc_number, contact_name, contact_email, status, escrow_balance_kobo, escrow_status)
		VALUES (?, ?, ?, ?, ?, 'applied', 1250000000, 'locked')
	`, id, businessName, rcNumber, contactName, contactEmail)
	return err
}

func GetVendor(conn *sql.DB, id string) (*Vendor, error) {
	row := conn.QueryRow(`SELECT `+vendorColumns+` FROM vendors WHERE id = ?`, id)
	return scanVendor(row)
}

func ListVendors(conn *sql.DB) ([]*Vendor, error) {
	rows, err := conn.Query(`SELECT ` + vendorColumns + ` FROM vendors ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Vendor
	for rows.Next() {
		v, err := scanVendor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func UpdateVendorCompanyLookup(conn *sql.DB, id, regNumber, status, companyType, nature, email, address, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE vendors SET
			company_registration_number = ?, company_status = ?, company_type = ?,
			company_nature_of_business = ?, company_email = ?, company_address = ?,
			company_lookup_raw = ?, status = 'company_verified', updated_at = datetime('now')
		WHERE id = ?
	`, regNumber, status, companyType, nature, email, address, rawJSON, id)
	return err
}

func UpdateVendorCompanyAdvanced(conn *sql.DB, id, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE vendors SET company_advanced_raw = ?, updated_at = datetime('now') WHERE id = ?
	`, rawJSON, id)
	return err
}

func UpdateVendorKYBFlowLink(conn *sql.DB, id, flowID, verificationID, verificationURL string) error {
	_, err := conn.Exec(`
		UPDATE vendors SET
			kyb_flow_id = ?, kyb_verification_id = ?, kyb_verification_url = ?,
			status = 'kyb_pending', updated_at = datetime('now')
		WHERE id = ?
	`, flowID, verificationID, verificationURL, id)
	return err
}

func UpdateVendorKYBOutcome(conn *sql.DB, verificationID, outcome, status, rawJSON string) error {
	_, err := conn.Exec(`
		UPDATE vendors SET kyb_outcome = ?, status = ?, kyb_raw = ?, updated_at = datetime('now')
		WHERE kyb_verification_id = ?
	`, outcome, status, rawJSON, verificationID)
	return err
}

func UpdateVendorPayoutWallet(conn *sql.DB, id, walletID string) error {
	_, err := conn.Exec(`
		UPDATE vendors SET payout_wallet_id = ?, escrow_status = 'released', status = 'payout_eligible', updated_at = datetime('now')
		WHERE id = ?
	`, walletID, id)
	return err
}

func UpdateVendorEscrowStatus(conn *sql.DB, id, escrowStatus string) error {
	_, err := conn.Exec(`UPDATE vendors SET escrow_status = ?, updated_at = datetime('now') WHERE id = ?`, escrowStatus, id)
	return err
}

func UpdateVendorStatus(conn *sql.DB, id, status string) error {
	_, err := conn.Exec(`UPDATE vendors SET status = ?, updated_at = datetime('now') WHERE id = ?`, status, id)
	return err
}

func GetVendorByKYBVerificationID(conn *sql.DB, verificationID string) (*Vendor, error) {
	row := conn.QueryRow(`SELECT `+vendorColumns+` FROM vendors WHERE kyb_verification_id = ?`, verificationID)
	return scanVendor(row)
}
