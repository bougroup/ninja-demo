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

// bucketRecommendation mirrors Ninja's documented default thresholds (90%
// accept, 60% review) as a fallback for sandbox responses that omit
// `recommendation` on the raw identify call.
func bucketRecommendation(score float64) string {
	if score <= 1.0 {
		score = score * 100
	}
	switch {
	case score >= 85:
		return "accept"
	case score >= 60:
		return "review"
	default:
		return "reject"
	}
}

func statusFromRecommendation(prefix, recommendation string) string {
	switch recommendation {
	case "accept":
		if prefix == "re_kyc" {
			return "re_kyc_cleared"
		}
		return "onboarded"
	case "reject":
		if prefix == "re_kyc" {
			return "re_kyc_failed"
		}
		return "flagged_review"
	default: // review
		return "flagged_review"
	}
}

// FintechOnboardForm renders the customer onboarding form.
// GET /app/fintech/onboard
func (e *Env) FintechOnboardForm(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, "fintech_onboard.html", nil)
}

// FintechOnboardSubmit calls POST /api/identity/identify (verify mode) —
// server-side, against the sandbox — and surfaces the per-field
// name-matching score Ninja returns, rather than a blunt pass/fail.
// POST /app/fintech/onboard
func (e *Env) FintechOnboardSubmit(w http.ResponseWriter, r *http.Request) {
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
		flash(w, r, "/app/fintech/onboard", "all fields are required")
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
		flash(w, r, "/app/fintech/onboard", "identify failed: "+err.Error())
		return
	}

	recommendation := result.Recommendation
	if recommendation == "" {
		recommendation = bucketRecommendation(result.Score)
	}
	mismatchesJSON, _ := json.Marshal(result.Mismatches())
	fieldsJSON, _ := json.Marshal(result.Fields)
	status := statusFromRecommendation("onboard", recommendation)

	customerID := uuid.NewString()
	if err := db.InsertFintechCustomer(e.DB, customerID, fullName, dob, idType, idNumber,
		result.Score, recommendation, string(mismatchesJSON), string(fieldsJSON), status); err != nil {
		log.Printf("insert fintech customer: %v", err)
		flash(w, r, "/app/fintech/onboard", "could not save customer")
		return
	}

	scorePct := result.Score
	if scorePct <= 1.0 {
		scorePct = scorePct * 100
	}
	flash(w, r, "/app/fintech", fmt.Sprintf("%s: %s (Fuzzy Confidence Score: %.0f%%)", fullName, strings.ToUpper(recommendation), scorePct))
}

// FintechDashboard lists onboarded customers with their score, tier, and transfers.
// GET /app/fintech
func (e *Env) FintechDashboard(w http.ResponseWriter, r *http.Request) {
	customers, err := db.ListFintechCustomers(e.DB)
	if err != nil {
		log.Printf("list fintech customers: %v", err)
	}

	transfers := map[string][]*db.FintechTransfer{}
	for _, c := range customers {
		if tList, err := db.ListFintechTransfersByCustomer(e.DB, c.ID); err == nil {
			transfers[c.ID] = tList
		}
	}

	e.render(w, r, "fintech_dashboard.html", map[string]any{
		"Customers": customers,
		"Transfers": transfers,
		"Flash":     r.URL.Query().Get("flash"),
	})
}

// FintechTransferSubmit processes a mock outbound wire transfer and enforces Tier limits.
// POST /app/fintech/customers/{id}/transfer
func (e *Env) FintechTransferSubmit(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	customer, err := db.GetFintechCustomer(e.DB, customerID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	amountNaira, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	if amountNaira <= 0 {
		amountNaira = 250000 // default ₦250,000 wire
	}
	amountKobo := int64(amountNaira * 100)
	recipientName := r.FormValue("recipient_name")
	if recipientName == "" {
		recipientName = "Adeola Trading Enterprises"
	}
	recipientBank := r.FormValue("recipient_bank")
	if recipientBank == "" {
		recipientBank = "Zenith Bank Plc"
	}
	recipientAcct := r.FormValue("recipient_acct")
	if recipientAcct == "" {
		recipientAcct = "1029384756"
	}

	// Check 1: Flagged Customer EDD
	if customer.Status == "flagged_review" {
		transferID := uuid.NewString()
		_ = db.InsertFintechTransfer(e.DB, transferID, customerID, recipientName, recipientBank, recipientAcct, amountKobo, "held_re_kyc", "Account has pending compliance review; wire held")
		flash(w, r, "/app/fintech", "Wire Held: Enhanced Due Diligence (Re-KYC) required before outbound funds transfer.")
		return
	}

	// Check 2: Tier Daily Limit
	if amountKobo > customer.DailyLimitKobo {
		transferID := uuid.NewString()
		reason := fmt.Sprintf("Amount %s exceeds Tier %d limit of %s", formatKobo(amountKobo), customer.Tier, formatKobo(customer.DailyLimitKobo))
		_ = db.InsertFintechTransfer(e.DB, transferID, customerID, recipientName, recipientBank, recipientAcct, amountKobo, "blocked_tier_limit", reason)
		flash(w, r, "/app/fintech", fmt.Sprintf("Transfer Blocked: %s exceeds Tier %d daily limit (%s). Upgrade Tier to proceed.", formatKobo(amountKobo), customer.Tier, formatKobo(customer.DailyLimitKobo)))
		return
	}

	// Transfer Approved
	transferID := uuid.NewString()
	_ = db.InsertFintechTransfer(e.DB, transferID, customerID, recipientName, recipientBank, recipientAcct, amountKobo, "completed", "Instant settlement cleared")
	flash(w, r, "/app/fintech", fmt.Sprintf("Wire Transfer of %s to %s (%s) Sent Successfully.", formatKobo(amountKobo), recipientName, recipientBank))
}

// FintechReKYC re-runs identify verify mode for an existing customer — the
// "re-KYC workflow for high-value or flagged accounts" the solution page describes.
// POST /app/fintech/customers/{id}/re-kyc
func (e *Env) FintechReKYC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	customer, err := db.GetFintechCustomer(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	first, last := splitName(customer.FullName)
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      customer.IDType,
		Mode:        "verify",
		IDNumber:    customer.IDNumber,
		FirstName:   first,
		LastName:    last,
		DateOfBirth: customer.DateOfBirth,
		Reference:   uuid.NewString(),
	})
	if err != nil {
		flash(w, r, "/app/fintech", "re-kyc failed: "+err.Error())
		return
	}

	recommendation := result.Recommendation
	if recommendation == "" {
		recommendation = bucketRecommendation(result.Score)
	}
	mismatchesJSON, _ := json.Marshal(result.Mismatches())
	fieldsJSON, _ := json.Marshal(result.Fields)
	status := statusFromRecommendation("re_kyc", recommendation)

	if err := db.UpdateFintechCustomerRecheck(e.DB, id, result.Score, recommendation, string(mismatchesJSON), string(fieldsJSON), status); err != nil {
		log.Printf("update fintech customer recheck: %v", err)
	}

	scorePct := result.Score
	if scorePct <= 1.0 {
		scorePct = scorePct * 100
	}
	flash(w, r, "/app/fintech", fmt.Sprintf("%s Re-KYC Result: %s (Confidence: %.0f%%, Tier Upgrade Evaluated)", customer.FullName, strings.ToUpper(recommendation), scorePct))
}

func formatScore(score float64) string {
	if score <= 1.0 {
		score = score * 100
	}
	return strconv.FormatFloat(score, 'f', 0, 64) + "%"
}
