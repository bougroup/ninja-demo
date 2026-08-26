package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// GamingSignupForm renders the punter signup form. GET /app/gaming/signup
func (e *Env) GamingSignupForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "gaming_signup.html", nil)
}

// GamingSignupSubmit runs the age/identity gate every new gaming account has
// to clear: block if this identity already has an account or is
// self-excluded, otherwise call POST /api/identity/identify (verify mode)
// — server-side, against the sandbox — and reject anyone under 18 or whose
// details don't match the registry. POST /app/gaming/signup
func (e *Env) GamingSignupSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	fullName := r.FormValue("full_name")
	dob := r.FormValue("date_of_birth")
	idType := r.FormValue("id_type")
	idNumber := r.FormValue("id_number")
	if fullName == "" || dob == "" || idType == "" || idNumber == "" {
		flash(w, r, "/app/gaming/signup", "all fields are required")
		return
	}

	existing, err := db.FindGamingPlayerByIdentity(e.DB, idType, idNumber)
	if err != nil {
		log.Printf("find gaming player: %v", err)
	}
	if existing != nil {
		if existing.SelfExcluded {
			flash(w, r, "/app/gaming/signup", "Identity Blocked: This ID is listed on the National Self-Exclusion Registry")
			return
		}
		flash(w, r, "/app/gaming/signup", "Account Exists: One verified BVN/NIN per gaming account constraint enforced")
		return
	}

	first, last := splitName(fullName)
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      idType,
		Mode:        "verify",
		IDNumber:    idNumber,
		FirstName:   first,
		LastName:    last,
		DateOfBirth: dob,
		Reference:   uuid.NewString(),
	})
	if err != nil {
		flash(w, r, "/app/gaming/signup", "identity check failed: "+err.Error())
		return
	}
	raw, _ := json.Marshal(result)

	age := ageFromDOB(dob)
	status := "active"
	var flashMsg string

	switch {
	case !result.Found || !result.Verified:
		status = "blocked_mismatch"
		flashMsg = fmt.Sprintf("Signup Blocked: Identity details could not be verified against national %s registry.", strings.ToUpper(idType))
	case age < 18:
		status = "blocked_underage"
		flashMsg = fmt.Sprintf("Underage Registration Blocked: Age calculated from DOB is %d (Must be 18+ per Gambling Act).", age)
	default:
		flashMsg = fmt.Sprintf("Player Onboarded Successfully: %s verified (Age %d) with ₦350,000 wallet activated.", fullName, age)
	}

	playerID := uuid.NewString()
	if err := db.InsertGamingPlayer(e.DB, playerID, fullName, dob, idType, idNumber, status, string(raw)); err != nil {
		log.Printf("insert gaming player: %v", err)
		flash(w, r, "/app/gaming/signup", "could not save signup")
		return
	}

	flash(w, r, "/app/gaming", flashMsg)
}

// GamingDashboard lists every player with their gate outcome, wallet balances, and payouts.
// GET /app/gaming
func (e *Env) GamingDashboard(w http.ResponseWriter, r *http.Request) {
	players, err := db.ListGamingPlayers(e.DB)
	if err != nil {
		log.Printf("list gaming players: %v", err)
	}

	playerPayouts := map[string][]*db.GamingPayout{}
	playerBets := map[string][]*db.GamingBet{}
	for _, p := range players {
		if payouts, err := db.ListGamingPayoutsByPlayer(e.DB, p.ID); err == nil {
			playerPayouts[p.ID] = payouts
		}
		if bets, err := db.ListGamingBetsByPlayer(e.DB, p.ID); err == nil {
			playerBets[p.ID] = bets
		}
	}

	render(w, r, "gaming_dashboard.html", map[string]any{
		"Players":       players,
		"PlayerPayouts": playerPayouts,
		"PlayerBets":    playerBets,
	})
}

// GamingPayoutCheck re-runs identity verification before releasing a
// withdrawal — the "payout validation" rule from the gaming solution page.
// POST /app/gaming/players/{id}/payout
func (e *Env) GamingPayoutCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	playerID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	amountNaira, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	if amountNaira <= 0 {
		amountNaira = 250000 // default ₦250,000 payout
	}
	amountKobo := int64(amountNaira * 100)

	player, err := db.GetGamingPlayer(e.DB, playerID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Rule 1: Self-Exclusion Gate
	if player.SelfExcluded {
		payoutID := uuid.NewString()
		_ = db.InsertGamingPayout(e.DB, payoutID, playerID, amountKobo, "blocked_self_excluded", "Player is on Self-Exclusion registry; payouts frozen", "")
		flash(w, r, "/app/gaming", "Payout Blocked: Player account is self-excluded.")
		return
	}

	// Rule 2: Server-side Identity & Age Re-Verification
	first, last := splitName(player.FullName)
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      player.IDType,
		Mode:        "verify",
		IDNumber:    player.IDNumber,
		FirstName:   first,
		LastName:    last,
		DateOfBirth: player.DateOfBirth,
		Reference:   uuid.NewString(),
	})
	if err != nil {
		flash(w, r, "/app/gaming", "Payout check failed upstream: "+err.Error())
		return
	}
	raw, _ := json.Marshal(result)

	age := ageFromDOB(player.DateOfBirth)
	status := "approved"
	reason := fmt.Sprintf("Identity verified; age %d cleared", age)
	var flashMsg string

	switch {
	case !result.Found || !result.Verified:
		status = "blocked_mismatch"
		reason = "National registry name/DOB mismatch"
		flashMsg = fmt.Sprintf("Payout Denied for %s: Identity mismatch against sandbox registry.", player.FullName)
	case age < 18:
		status = "blocked_underage"
		reason = fmt.Sprintf("Player age %d is under 18", age)
		flashMsg = fmt.Sprintf("CRITICAL AML/AGE ALERT: Payout of %s blocked — Player is %d years old (Underage).", formatKobo(amountKobo), age)
	default:
		flashMsg = fmt.Sprintf("Payout of %s Approved for %s: Real-time Ninja check cleared (Age %d).", formatKobo(amountKobo), player.FullName, age)
		// Deduct from winnings
		newWinnings := player.WinningsKobo - amountKobo
		if newWinnings < 0 {
			newWinnings = 0
		}
		_ = db.UpdateGamingPlayerBalance(e.DB, playerID, player.BalanceKobo, newWinnings)
	}

	payoutID := uuid.NewString()
	if err := db.InsertGamingPayout(e.DB, payoutID, playerID, amountKobo, status, reason, string(raw)); err != nil {
		log.Printf("insert gaming payout: %v", err)
	}

	flash(w, r, "/app/gaming", flashMsg)
}

// GamingBetSubmit places an interactive wager and tests AML thresholds.
// POST /app/gaming/players/{id}/bet
func (e *Env) GamingBetSubmit(w http.ResponseWriter, r *http.Request) {
	playerID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	player, err := db.GetGamingPlayer(e.DB, playerID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if player.SelfExcluded || player.Status == "blocked_underage" {
		flash(w, r, "/app/gaming", "Wager Rejected: Player account is restricted.")
		return
	}

	stakeNaira, _ := strconv.ParseFloat(r.FormValue("stake"), 64)
	if stakeNaira <= 0 {
		stakeNaira = 50000
	}
	stakeKobo := int64(stakeNaira * 100)
	matchEvent := r.FormValue("match_event")
	if matchEvent == "" {
		matchEvent = "Arsenal vs Chelsea (Premier League)"
	}
	selection := r.FormValue("selection")
	if selection == "" {
		selection = "Arsenal to Win"
	}
	odds, _ := strconv.ParseFloat(r.FormValue("odds"), 64)
	if odds <= 1.0 {
		odds = 2.15
	}

	status := "placed"
	flashMsg := fmt.Sprintf("Wager Placed: %s on %s @ %.2f odds.", formatKobo(stakeKobo), selection, odds)

	// High stakes AML flag trigger (> ₦500k)
	if stakeKobo >= 50000000 {
		status = "flagged_aml"
		flashMsg = fmt.Sprintf("High-Stakes AML Alert: Wager of %s triggered enhanced due diligence review.", formatKobo(stakeKobo))
	}

	betID := uuid.NewString()
	_ = db.InsertGamingBet(e.DB, betID, playerID, matchEvent, selection, odds, stakeKobo, status)

	flash(w, r, "/app/gaming", flashMsg)
}

// GamingSelfExclude places the player on the self-exclusion list.
// POST /app/gaming/players/{id}/self-exclude
func (e *Env) GamingSelfExclude(w http.ResponseWriter, r *http.Request) {
	playerID := r.PathValue("id")
	if err := db.SetGamingPlayerSelfExcluded(e.DB, playerID); err != nil {
		log.Printf("self-exclude: %v", err)
	}
	flash(w, r, "/app/gaming", "Player added to Self-Exclusion registry: All payouts and wagering immediately suspended.")
}
