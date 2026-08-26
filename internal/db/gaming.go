package db

import (
	"database/sql"
)

type GamingPlayer struct {
	ID           string
	FullName     string
	DateOfBirth  string
	IDType       string
	IDNumber     string
	IdentifyRaw  sql.NullString
	SelfExcluded bool
	Status       string
	BalanceKobo  int64
	WinningsKobo int64
	CreatedAt    string
}

const gamingPlayerColumns = `id, full_name, date_of_birth, id_type, id_number, identify_raw, self_excluded, status, balance_kobo, winnings_kobo, created_at`

func scanGamingPlayer(row interface{ Scan(dest ...any) error }) (*GamingPlayer, error) {
	var p GamingPlayer
	err := row.Scan(&p.ID, &p.FullName, &p.DateOfBirth, &p.IDType, &p.IDNumber, &p.IdentifyRaw, &p.SelfExcluded, &p.Status, &p.BalanceKobo, &p.WinningsKobo, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindGamingPlayerByIdentity looks up an existing player by their id_type +
// id_number — the one-identity-per-account dedupe check run at signup.
func FindGamingPlayerByIdentity(conn *sql.DB, idType, idNumber string) (*GamingPlayer, error) {
	row := conn.QueryRow(`SELECT `+gamingPlayerColumns+` FROM gaming_players WHERE id_type = ? AND id_number = ?`, idType, idNumber)
	p, err := scanGamingPlayer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func InsertGamingPlayer(conn *sql.DB, id, fullName, dob, idType, idNumber, status, identifyRawJSON string) error {
	_, err := conn.Exec(`
		INSERT INTO gaming_players (id, full_name, date_of_birth, id_type, id_number, status, identify_raw, balance_kobo, winnings_kobo)
		VALUES (?, ?, ?, ?, ?, ?, ?, 35000000, 18000000)
	`, id, fullName, dob, idType, idNumber, status, identifyRawJSON)
	return err
}

func GetGamingPlayer(conn *sql.DB, id string) (*GamingPlayer, error) {
	row := conn.QueryRow(`SELECT `+gamingPlayerColumns+` FROM gaming_players WHERE id = ?`, id)
	return scanGamingPlayer(row)
}

func ListGamingPlayers(conn *sql.DB) ([]*GamingPlayer, error) {
	rows, err := conn.Query(`SELECT ` + gamingPlayerColumns + ` FROM gaming_players ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GamingPlayer
	for rows.Next() {
		p, err := scanGamingPlayer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func SetGamingPlayerSelfExcluded(conn *sql.DB, id string) error {
	_, err := conn.Exec(`UPDATE gaming_players SET self_excluded = 1, status = 'blocked_self_excluded' WHERE id = ?`, id)
	return err
}

func UpdateGamingPlayerBalance(conn *sql.DB, id string, balanceKobo, winningsKobo int64) error {
	_, err := conn.Exec(`UPDATE gaming_players SET balance_kobo = ?, winnings_kobo = ? WHERE id = ?`, balanceKobo, winningsKobo, id)
	return err
}

type GamingPayout struct {
	ID          string
	PlayerID    string
	AmountKobo  int64
	IdentifyRaw sql.NullString
	Status      string
	Reason      sql.NullString
	CreatedAt   string
}

func InsertGamingPayout(conn *sql.DB, id, playerID string, amountKobo int64, status, reason, identifyRawJSON string) error {
	_, err := conn.Exec(`
		INSERT INTO gaming_payouts (id, player_id, amount_kobo, status, reason, identify_raw)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, playerID, amountKobo, status, reason, identifyRawJSON)
	return err
}

func ListGamingPayoutsByPlayer(conn *sql.DB, playerID string) ([]*GamingPayout, error) {
	rows, err := conn.Query(`
		SELECT id, player_id, amount_kobo, identify_raw, status, reason, created_at
		FROM gaming_payouts WHERE player_id = ? ORDER BY created_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GamingPayout
	for rows.Next() {
		var p GamingPayout
		if err := rows.Scan(&p.ID, &p.PlayerID, &p.AmountKobo, &p.IdentifyRaw, &p.Status, &p.Reason, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

type GamingBet struct {
	ID         string
	PlayerID   string
	MatchEvent string
	Selection  string
	Odds       float64
	StakeKobo  int64
	Status     string
	CreatedAt  string
}

func InsertGamingBet(conn *sql.DB, id, playerID, matchEvent, selection string, odds float64, stakeKobo int64, status string) error {
	_, err := conn.Exec(`
		INSERT INTO gaming_bets (id, player_id, match_event, selection, odds, stake_kobo, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, playerID, matchEvent, selection, odds, stakeKobo, status)
	return err
}

func ListGamingBetsByPlayer(conn *sql.DB, playerID string) ([]*GamingBet, error) {
	rows, err := conn.Query(`
		SELECT id, player_id, match_event, selection, odds, stake_kobo, status, created_at
		FROM gaming_bets WHERE player_id = ? ORDER BY created_at DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*GamingBet
	for rows.Next() {
		var b GamingBet
		if err := rows.Scan(&b.ID, &b.PlayerID, &b.MatchEvent, &b.Selection, &b.Odds, &b.StakeKobo, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}
