package db

import (
	"database/sql"
)

type IdentityCheck struct {
	ID        string
	Scenario  string
	Operator  string
	BatchID   sql.NullString
	Mode      string
	IDType    string
	IDNumber  string
	Reference sql.NullString
	ResultRaw sql.NullString
	CreatedAt string
}

const identityCheckColumns = `id, scenario, operator, batch_id, mode, id_type, id_number, reference, result_raw, created_at`

func scanIdentityCheck(row interface{ Scan(dest ...any) error }) (*IdentityCheck, error) {
	var ic IdentityCheck
	err := row.Scan(&ic.ID, &ic.Scenario, &ic.Operator, &ic.BatchID, &ic.Mode, &ic.IDType, &ic.IDNumber, &ic.Reference, &ic.ResultRaw, &ic.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &ic, nil
}

// InsertIdentityCheck logs one POST /api/identity/identify call. scenario
// tags which demo surface triggered it (admin_spot_check,
// international_console); batchID groups rows from a single bulk-identify
// call and may be empty.
func InsertIdentityCheck(conn *sql.DB, id, scenario, operator, batchID, mode, idType, idNumber, reference, resultJSON string) error {
	_, err := conn.Exec(`
		INSERT INTO identity_checks (id, scenario, operator, batch_id, mode, id_type, id_number, reference, result_raw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, scenario, operator, nullIfEmpty(batchID), mode, idType, idNumber, reference, resultJSON)
	return err
}

func ListIdentityChecks(conn *sql.DB, limit int) ([]*IdentityCheck, error) {
	rows, err := conn.Query(`SELECT `+identityCheckColumns+` FROM identity_checks ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*IdentityCheck
	for rows.Next() {
		ic, err := scanIdentityCheck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ic)
	}
	return out, rows.Err()
}

func ListIdentityChecksByScenario(conn *sql.DB, scenario string, limit int) ([]*IdentityCheck, error) {
	rows, err := conn.Query(`
		SELECT `+identityCheckColumns+` FROM identity_checks WHERE scenario = ? ORDER BY created_at DESC LIMIT ?
	`, scenario, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*IdentityCheck
	for rows.Next() {
		ic, err := scanIdentityCheck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ic)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

type WebhookEvent struct {
	ID          string
	Event       string
	DeliveryID  sql.NullString
	PayloadRaw  string
	SignatureOK bool
	Processed   bool
	Error       sql.NullString
	ReceivedAt  string
}

func InsertWebhookEvent(conn *sql.DB, id, event, deliveryID, payloadRaw string, signatureOK bool) error {
	_, err := conn.Exec(`
		INSERT INTO webhook_events (id, event, delivery_id, payload_raw, signature_ok)
		VALUES (?, ?, ?, ?, ?)
	`, id, event, deliveryID, payloadRaw, signatureOK)
	return err
}

func MarkWebhookEventProcessed(conn *sql.DB, id string, processErr error) error {
	var errMsg sql.NullString
	if processErr != nil {
		errMsg = sql.NullString{String: processErr.Error(), Valid: true}
	}
	_, err := conn.Exec(`
		UPDATE webhook_events SET processed = 1, error = ? WHERE id = ?
	`, errMsg, id)
	return err
}

func ListWebhookEvents(conn *sql.DB, limit int) ([]*WebhookEvent, error) {
	rows, err := conn.Query(`
		SELECT id, event, delivery_id, payload_raw, signature_ok, processed, error, received_at
		FROM webhook_events ORDER BY received_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*WebhookEvent
	for rows.Next() {
		var we WebhookEvent
		if err := rows.Scan(&we.ID, &we.Event, &we.DeliveryID, &we.PayloadRaw, &we.SignatureOK, &we.Processed, &we.Error, &we.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, &we)
	}
	return out, rows.Err()
}
