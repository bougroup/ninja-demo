package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open opens (creating if needed) the SQLite file at path and applies the schema.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	// Apply column additions if existing database was from previous migration
	migrateColumns := []string{
		"ALTER TABLE vendors ADD COLUMN escrow_balance_kobo INTEGER NOT NULL DEFAULT 1250000000",
		"ALTER TABLE vendors ADD COLUMN escrow_status TEXT NOT NULL DEFAULT 'locked'",
		"ALTER TABLE gaming_players ADD COLUMN balance_kobo INTEGER NOT NULL DEFAULT 35000000",
		"ALTER TABLE gaming_players ADD COLUMN winnings_kobo INTEGER NOT NULL DEFAULT 18000000",
		"ALTER TABLE gaming_payouts ADD COLUMN reason TEXT",
		"ALTER TABLE fintech_customers ADD COLUMN tier INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE fintech_customers ADD COLUMN daily_limit_kobo INTEGER NOT NULL DEFAULT 5000000",
		"ALTER TABLE fintech_customers ADD COLUMN balance_kobo INTEGER NOT NULL DEFAULT 125000000",
		"ALTER TABLE agents ADD COLUMN pos_terminal_count INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE agents ADD COLUMN cash_float_kobo INTEGER NOT NULL DEFAULT 200000000",
		"ALTER TABLE candidates ADD COLUMN monthly_salary_kobo INTEGER NOT NULL DEFAULT 85000000",
		"ALTER TABLE candidates ADD COLUMN payroll_status TEXT NOT NULL DEFAULT 'locked'",
	}
	for _, stmt := range migrateColumns {
		_, _ = conn.Exec(stmt) // ignore error if column already exists
	}

	return conn, nil
}

// GetConfig reads a single key from the config table, returning "" if absent.
func GetConfig(conn *sql.DB, key string) (string, error) {
	var value string
	err := conn.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig upserts a key in the config table.
func SetConfig(conn *sql.DB, key, value string) error {
	_, err := conn.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

type APILog struct {
	ID              string
	Endpoint        string
	Method          string
	StatusCode      int
	DurationMs      int
	RequestPayload  sql.NullString
	ResponsePayload sql.NullString
	IsMock          bool
	CreatedAt       string
}

func InsertAPILog(conn *sql.DB, id, endpoint, method string, statusCode, durationMs int, reqPayload, respPayload string, isMock bool) error {
	mockVal := 0
	if isMock {
		mockVal = 1
	}
	_, err := conn.Exec(`
		INSERT INTO api_logs (id, endpoint, method, status_code, duration_ms, request_payload, response_payload, is_mock)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, endpoint, strings.ToUpper(method), statusCode, durationMs, reqPayload, respPayload, mockVal)
	return err
}

func ListRecentAPILogs(conn *sql.DB, limit int) ([]*APILog, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := conn.Query(`
		SELECT id, endpoint, method, status_code, duration_ms, request_payload, response_payload, is_mock, created_at
		FROM api_logs ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*APILog
	for rows.Next() {
		var l APILog
		var isMockInt int
		if err := rows.Scan(&l.ID, &l.Endpoint, &l.Method, &l.StatusCode, &l.DurationMs, &l.RequestPayload, &l.ResponsePayload, &isMockInt, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.IsMock = isMockInt == 1
		out = append(out, &l)
	}
	return out, rows.Err()
}
