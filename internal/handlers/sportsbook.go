package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/email"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// RegisterForm renders the clean player registration page.
// GET /register, GET /signup
func (e *Env) RegisterForm(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, "sportsbook_signup.html", map[string]any{
		"Title": "ApexBet — Player Registration & Welcome Bonus",
	})
}

// RegisterSubmit creates the user, dispatches a welcome email via Resend,
// credits the ₦100,000 welcome balance, sets session cookie, and redirects to sportsbook.
// POST /register, POST /signup
func (e *Env) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	userEmail := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	mobile := strings.TrimSpace(r.FormValue("mobile"))

	if firstName == "" || lastName == "" || userEmail == "" || password == "" {
		flash(w, r, "/register", "Please fill in First Name, Last Name, Email, and Password.")
		return
	}

	// Check if user already exists
	existing, err := db.GetUserByEmail(e.DB, userEmail)
	if err != nil {
		log.Printf("check existing user: %v", err)
	}
	if existing != nil {
		// Log in as existing user
		http.SetCookie(w, &http.Cookie{
			Name:     "apexbet_user_id",
			Value:    existing.ID,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400 * 7,
		})
		flash(w, r, "/sportsbook", fmt.Sprintf("Welcome back, %s! Logged in successfully.", existing.FirstName))
		return
	}

	userID := uuid.NewString()
	passwordHash := "hashed_" + uuid.NewString()[:8]

	if err := db.CreateUser(e.DB, userID, firstName, lastName, userEmail, passwordHash, mobile); err != nil {
		log.Printf("create user: %v", err)
		flash(w, r, "/register", "Could not create account. Please try again.")
		return
	}

	// Dispatch Welcome Email via Resend
	go func() {
		status, htmlBody, err := email.SendWelcomeEmail(userEmail, firstName)
		if err != nil {
			log.Printf("resend email error for %s: %v", userEmail, err)
		}
		_ = db.InsertEmailLog(e.DB, uuid.NewString(), userEmail, "🎰 Welcome to ApexBet — Complete 18+ Verification", htmlBody, status)
	}()

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "apexbet_user_id",
		Value:    userID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7,
	})

	flash(w, r, "/sportsbook", fmt.Sprintf("Welcome to ApexBet, %s! ₦100,000 welcome credit added. Please complete 18+ ID verification to activate betting.", firstName))
}

// SportsbookDashboard renders the SportyBet-style Live Football & Betting Dashboard.
// GET /, GET /sportsbook
func (e *Env) SportsbookDashboard(w http.ResponseWriter, r *http.Request) {
	user := e.getCurrentUser(r)
	if user == nil {
		demoID := uuid.NewString()
		_ = db.CreateUser(e.DB, demoID, "James", "Bond", "james.bond@apexbet.ng", "pass123", "08012345678")
		user, _ = db.GetUser(e.DB, demoID)
		if user == nil {
			user = &db.User{
				ID:          demoID,
				FirstName:   "James",
				LastName:    "Bond",
				Email:       "james.bond@apexbet.ng",
				KYCStatus:   "unverified",
				BalanceKobo: 10000000,
			}
		}
	}

	// Get user's recent bets and payouts
	playerBets, _ := db.ListGamingBetsByPlayer(e.DB, user.ID)
	playerPayouts, _ := db.ListGamingPayoutsByPlayer(e.DB, user.ID)
	emailLogs, _ := db.ListEmailLogs(e.DB, 5)

	e.render(w, r, "sportsbook.html", map[string]any{
		"User":          user,
		"PlayerBets":    playerBets,
		"PlayerPayouts": playerPayouts,
		"EmailLogs":     emailLogs,
		"CurrentTime":   time.Now().Format("15:04:05"),
	})
}

// VerifyIdentitySubmit calls Ninja POST /api/identity/identify in verify mode
// to enforce the NLRC §34 18+ statute check and name confidence scoring.
// POST /api/kyc/verify
func (e *Env) VerifyIdentitySubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := e.getCurrentUser(r)
	if user == nil {
		flash(w, r, "/register", "Please register or log in first.")
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	idType := r.FormValue("id_type")
	idNumber := strings.TrimSpace(r.FormValue("id_number"))
	dob := r.FormValue("date_of_birth")

	if idType == "" || idNumber == "" || dob == "" {
		flash(w, r, "/sportsbook", "Please provide ID Type, ID Number, and Date of Birth.")
		return
	}

	// Rule 1: Self-exclusion check
	if user.SelfExcluded {
		flash(w, r, "/sportsbook", "Identity Blocked: Account is listed on the National Self-Exclusion Registry.")
		return
	}

	// Rule 2: Server-to-Server Ninja Identity & Age Check
	start := time.Now()
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      idType,
		Mode:        "verify",
		IDNumber:    idNumber,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		DateOfBirth: dob,
		Reference:   "kyc_" + uuid.NewString()[:8],
	})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		flash(w, r, "/sportsbook", "Ninja API check failed: "+err.Error())
		return
	}

	rawJSON, _ := json.Marshal(result)
	age := ageFromDOB(dob)
	status := "verified"
	var flashMsg string

	switch {
	case age < 18:
		status = "blocked_underage"
		flashMsg = fmt.Sprintf("⛔ NLRC 18+ VIOLATION: Date of Birth indicates age %d (Under 18). Account and betting wallet permanently locked.", age)
	case !result.Found || !result.Verified:
		status = "blocked_mismatch"
		flashMsg = fmt.Sprintf("Identity Verification Failed: Details did not match the national %s registry record (Latency: %dms).", strings.ToUpper(idType), duration)
	default:
		status = "verified"
		flashMsg = fmt.Sprintf("✓ Identity Verified (Score %.0f%% &middot; Age %d &middot; %dms): 18+ NLRC compliance cleared. Full betting and payout features activated!", result.Score*100, age, duration)
	}

	_ = db.UpdateUserKYC(e.DB, user.ID, status, idType, idNumber, dob, age, result.Score, string(rawJSON))
	flash(w, r, "/sportsbook", flashMsg)
}

// PlaceBetSubmit processes a live match wager.
// POST /api/bets/place
func (e *Env) PlaceBetSubmit(w http.ResponseWriter, r *http.Request) {
	user := e.getCurrentUser(r)
	if user == nil {
		flash(w, r, "/register", "Please register or log in first.")
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	// Regulatory Gate: Check if user is verified 18+
	if user.KYCStatus == "blocked_underage" {
		flash(w, r, "/sportsbook", "⛔ Bet Blocked: Underage account (Age < 18). Regulated by NLRC Act §34.")
		return
	}
	if user.KYCStatus != "verified" {
		flash(w, r, "/sportsbook", "⚠️ Action Blocked: You must complete 18+ Identity Verification before placing real-money wagers.")
		return
	}
	if user.SelfExcluded {
		flash(w, r, "/sportsbook", "⛔ Bet Blocked: Account is self-excluded.")
		return
	}

	matchEvent := r.FormValue("match_event")
	selection := r.FormValue("selection")
	odds, _ := strconv.ParseFloat(r.FormValue("odds"), 64)
	if odds <= 1.0 {
		odds = 2.10
	}
	stakeNaira, _ := strconv.ParseFloat(r.FormValue("stake"), 64)
	if stakeNaira <= 0 {
		stakeNaira = 5000 // default ₦5,000
	}
	stakeKobo := int64(stakeNaira * 100)

	if user.BalanceKobo < stakeKobo {
		flash(w, r, "/sportsbook", fmt.Sprintf("Insufficient balance: Available balance is %s, required stake is %s.", formatKobo(user.BalanceKobo), formatKobo(stakeKobo)))
		return
	}

	// Deduct balance & create bet
	newBalance := user.BalanceKobo - stakeKobo
	_ = db.UpdateUserBalance(e.DB, user.ID, newBalance, user.WinningsKobo)
	_ = db.InsertGamingBet(e.DB, uuid.NewString(), user.ID, matchEvent, selection, odds, stakeKobo, "placed")

	potentialWin := int64(float64(stakeKobo) * odds)
	flash(w, r, "/sportsbook", fmt.Sprintf("🎲 Bet Placed Successfully! %s &middot; Selection: %s @ %.2f (Potential Win: %s)", matchEvent, selection, odds, formatKobo(potentialWin)))
}

// SimulateMatchWin credits ₦250,000 in winnings to test withdrawal gates.
// POST /api/bets/simulate-win
func (e *Env) SimulateMatchWin(w http.ResponseWriter, r *http.Request) {
	user := e.getCurrentUser(r)
	if user == nil {
		flash(w, r, "/register", "Please register or log in first.")
		return
	}

	winKobo := int64(25000000) // ₦250,000
	newWinnings := user.WinningsKobo + winKobo
	_ = db.UpdateUserBalance(e.DB, user.ID, user.BalanceKobo, newWinnings)
	_ = db.InsertGamingBet(e.DB, uuid.NewString(), user.ID, "Arsenal 2 - 1 Chelsea (Premier League)", "Arsenal to Win (WON)", 2.45, 5000000, "won")

	flash(w, r, "/sportsbook", fmt.Sprintf("🏆 MATCH WON! Arsenal defeated Chelsea 2-1. Credited %s to your winnings balance. Ready for withdrawal!", formatKobo(winKobo)))
}

// RequestPayoutCheck re-runs identity verification via Ninja before approving withdrawal.
// POST /api/payout/request
func (e *Env) RequestPayoutCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := e.getCurrentUser(r)
	if user == nil {
		flash(w, r, "/register", "Please register or log in first.")
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	amountNaira, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	if amountNaira <= 0 {
		amountNaira = 250000
	}
	amountKobo := int64(amountNaira * 100)

	if user.WinningsKobo < amountKobo {
		flash(w, r, "/sportsbook", fmt.Sprintf("Insufficient winnings: Available winnings balance is %s.", formatKobo(user.WinningsKobo)))
		return
	}

	// Compliance Gate 1: Self-Exclusion
	if user.SelfExcluded {
		_ = db.InsertGamingPayout(e.DB, uuid.NewString(), user.ID, amountKobo, "blocked_self_excluded", "Account is on Self-Exclusion registry; payouts frozen", "")
		flash(w, r, "/sportsbook", "⛔ Payout Blocked: Player account is listed on the Self-Exclusion Registry.")
		return
	}

	// Compliance Gate 2: Server-side Identity & Age Re-Verification with Ninja
	idType := user.IDType
	if idType == "" {
		idType = "bvn"
	}
	idNumber := user.IDNumber
	if idNumber == "" {
		idNumber = "77777777777"
	}
	dob := user.DateOfBirth
	if dob == "" {
		dob = "1975-01-01"
	}

	start := time.Now()
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      idType,
		Mode:        "verify",
		IDNumber:    idNumber,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		DateOfBirth: dob,
		Reference:   "payout_" + uuid.NewString()[:8],
	})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		flash(w, r, "/sportsbook", "Payout check failed upstream: "+err.Error())
		return
	}

	raw, _ := json.Marshal(result)
	age := ageFromDOB(dob)
	status := "approved"
	reason := fmt.Sprintf("Identity verified (Score %.0f%%); age %d cleared in %dms", result.Score*100, age, duration)
	var flashMsg string

	switch {
	case age < 18:
		status = "blocked_underage"
		reason = fmt.Sprintf("Player age %d is under 18", age)
		flashMsg = fmt.Sprintf("⛔ CRITICAL AML/AGE ALERT: Payout of %s blocked — Player is %d years old (Underage).", formatKobo(amountKobo), age)
	case !result.Found || !result.Verified:
		status = "blocked_mismatch"
		reason = "National registry name/DOB mismatch on payout rail"
		flashMsg = fmt.Sprintf("⛔ Payout Denied: Identity mismatch against national %s registry.", strings.ToUpper(idType))
	default:
		status = "approved"
		flashMsg = fmt.Sprintf("✓ Payout of %s Approved & Disbursed in %dms! Real-time Ninja KYC verified recipient matches BVN (%s).", formatKobo(amountKobo), duration, user.FullName())
		newWinnings := user.WinningsKobo - amountKobo
		if newWinnings < 0 {
			newWinnings = 0
		}
		_ = db.UpdateUserBalance(e.DB, user.ID, user.BalanceKobo, newWinnings)
	}

	payoutID := uuid.NewString()
	_ = db.InsertGamingPayout(e.DB, payoutID, user.ID, amountKobo, status, reason, string(raw))
	flash(w, r, "/sportsbook", flashMsg)
}

// UserSelfExclude locks the player from all betting and payouts.
// POST /api/responsible-gaming/self-exclude
func (e *Env) UserSelfExclude(w http.ResponseWriter, r *http.Request) {
	user := e.getCurrentUser(r)
	if user == nil {
		flash(w, r, "/register", "Please register or log in first.")
		return
	}

	_ = db.SetUserSelfExcluded(e.DB, user.ID)
	flash(w, r, "/sportsbook", "⛔ Responsible Gaming: You have been added to the National Self-Exclusion Registry. All betting, deposits, and withdrawals have been frozen.")
}

// ResetDemoState restores clean balances and clears flags for easy presentations.
// POST /api/demo/reset
func (e *Env) ResetDemoState(w http.ResponseWriter, r *http.Request) {
	user := e.getCurrentUser(r)
	if user != nil {
		_ = db.UpdateUserBalance(e.DB, user.ID, 10000000, 0)
		_ = db.UpdateUserKYC(e.DB, user.ID, "unverified", "", "", "", 0, 0, "")
		_, _ = e.DB.Exec(`UPDATE users SET self_excluded = 0 WHERE id = ?`, user.ID)
	}
	flash(w, r, "/sportsbook", "⚡ Demo State Reset: Restored initial ₦100,000 balance and unverified status.")
}

// Helper: getCurrentUser reads user from cookie or returns latest active user.
func (e *Env) getCurrentUser(r *http.Request) *db.User {
	cookie, err := r.Cookie("apexbet_user_id")
	if err == nil && cookie.Value != "" {
		if u, err := db.GetUser(e.DB, cookie.Value); err == nil && u != nil {
			return u
		}
	}
	u, _ := db.GetLatestUser(e.DB)
	if u != nil {
		return u
	}
	demoID := uuid.NewString()
	_ = db.CreateUser(e.DB, demoID, "James", "Bond", "james.bond@apexbet.ng", "pass123", "08012345678")
	u, _ = db.GetUser(e.DB, demoID)
	return u
}
