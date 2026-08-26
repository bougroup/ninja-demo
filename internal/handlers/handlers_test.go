package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
)

func setupTestEnv(t *testing.T) *Env {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	LoadTemplates("../../templates/*.html")
	client := ninja.NewClient("https://api.sandbox.ninja.boucloud.io", "key", "secret")
	client.SetMockMode(true)
	return &Env{
		DB:        conn,
		Ninja:     client,
		PublicURL: "http://localhost:4000",
	}
}

func TestGamingUnderageAndPayoutLogic(t *testing.T) {
	env := setupTestEnv(t)
	defer env.DB.Close()

	// 1. Underage Registration Test (Tobi Minor, DOB 2010)
	form := url.Values{
		"full_name":     {"Tobi Minor"},
		"date_of_birth": {"2010-06-15"},
		"id_type":       {"bvn"},
		"id_number":     {"88888888888"},
	}
	req := httptest.NewRequest("POST", "/app/gaming/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	env.GamingSignupSubmit(rec, req)

	player, err := db.FindGamingPlayerByIdentity(env.DB, "bvn", "88888888888")
	if err != nil || player == nil {
		t.Fatalf("expected player to be saved, err: %v", err)
	}
	if player.Status != "blocked_underage" {
		t.Errorf("expected player status blocked_underage, got %s", player.Status)
	}

	// 2. Verified Adult Registration (James Bond, DOB 1975)
	formAdult := url.Values{
		"full_name":     {"James Bond"},
		"date_of_birth": {"1975-01-01"},
		"id_type":       {"bvn"},
		"id_number":     {"77777777777"},
	}
	reqAdult := httptest.NewRequest("POST", "/app/gaming/signup", strings.NewReader(formAdult.Encode()))
	reqAdult.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recAdult := httptest.NewRecorder()

	env.GamingSignupSubmit(recAdult, reqAdult)

	adultPlayer, err := db.FindGamingPlayerByIdentity(env.DB, "bvn", "77777777777")
	if err != nil || adultPlayer == nil {
		t.Fatalf("expected adult player to be saved, err: %v", err)
	}
	if adultPlayer.Status != "active" {
		t.Errorf("expected adult player status active, got %s", adultPlayer.Status)
	}

	// 3. Payout Check for Adult Player
	payoutForm := url.Values{"amount": {"250000"}}
	payoutReq := httptest.NewRequest("POST", "/app/gaming/players/"+adultPlayer.ID+"/payout", strings.NewReader(payoutForm.Encode()))
	payoutReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	payoutReq.SetPathValue("id", adultPlayer.ID)
	payoutRec := httptest.NewRecorder()

	env.GamingPayoutCheck(payoutRec, payoutReq)

	payouts, err := db.ListGamingPayoutsByPlayer(env.DB, adultPlayer.ID)
	if err != nil || len(payouts) == 0 {
		t.Fatalf("expected payout to be recorded, err: %v", err)
	}
	if payouts[0].Status != "approved" {
		t.Errorf("expected payout approved, got %s", payouts[0].Status)
	}
}

func TestFintechTieringAndTransferLimits(t *testing.T) {
	env := setupTestEnv(t)
	defer env.DB.Close()

	// 1. Onboard Customer (James Bond, High confidence -> Tier 3)
	form := url.Values{
		"full_name":     {"James Bond"},
		"date_of_birth": {"1975-01-01"},
		"id_type":       {"bvn"},
		"id_number":     {"77777777777"},
	}
	req := httptest.NewRequest("POST", "/app/fintech/onboard", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	env.FintechOnboardSubmit(rec, req)

	customers, err := db.ListFintechCustomers(env.DB)
	if err != nil || len(customers) == 0 {
		t.Fatalf("expected customer to be created, err: %v", err)
	}
	customer := customers[0]
	if customer.Tier < 2 {
		t.Errorf("expected customer to have Tier 2 or 3, got Tier %d", customer.Tier)
	}

	// 2. Send ₦45,000 Transfer (Under limit)
	tForm := url.Values{
		"amount":         {"45000"},
		"recipient_name": {"Adeola Trading"},
	}
	tReq := httptest.NewRequest("POST", "/app/fintech/customers/"+customer.ID+"/transfer", strings.NewReader(tForm.Encode()))
	tReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tReq.SetPathValue("id", customer.ID)
	tRec := httptest.NewRecorder()

	env.FintechTransferSubmit(tRec, tReq)

	transfers, err := db.ListFintechTransfersByCustomer(env.DB, customer.ID)
	if err != nil || len(transfers) == 0 {
		t.Fatalf("expected transfer to be recorded")
	}
	if transfers[0].Status != "completed" {
		t.Errorf("expected transfer status completed, got %s", transfers[0].Status)
	}
}

func TestAgentFraudRingSimulation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.DB.Close()

	// Simulate Fraud Ring (3 terminals under same BVN)
	req := httptest.NewRequest("POST", "/app/agents/simulate-fraud-ring", nil)
	rec := httptest.NewRecorder()

	env.AgentSimulateFraudRing(rec, req)

	agents, err := db.ListAgents(env.DB)
	if err != nil || len(agents) < 3 {
		t.Fatalf("expected at least 3 simulated agents, got %d", len(agents))
	}
	if agents[0].Status != "flagged_duplicate_bvn" {
		t.Errorf("expected flagged_duplicate_bvn status, got %s", agents[0].Status)
	}
}
