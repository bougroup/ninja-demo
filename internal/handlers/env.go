package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
)

// Env is the shared dependency bag every handler closes over.
type Env struct {
	DB        *sql.DB
	Ninja     *ninja.Client
	PublicURL string // base URL other services (webhooks, redirects) use to reach this app
}

const (
	cfgKYCFlowVendorDirectors = "kyc_flow_vendor_directors"
	cfgKYBFlowVendorBusiness  = "kyb_flow_vendor_business"
	cfgKYCFlowEmployees       = "kyc_flow_employees"
	cfgKYBFlowAggregators     = "kyb_flow_aggregators"
)

// EnsureVendorFlows creates the Marketplace Vendor Payouts hosted flows once
// (idempotent — IDs are cached in the config table).
func (e *Env) EnsureVendorFlows(ctx context.Context) (kycFlowID, kybFlowID string, err error) {
	kycFlowID, err = e.ensureKYCFlow(ctx, cfgKYCFlowVendorDirectors, "Marketplace Director KYC",
		"/app/vendor/kyc-complete")
	if err != nil {
		return "", "", fmt.Errorf("ensure vendor kyc flow: %w", err)
	}
	kybFlowID, err = e.ensureKYBFlow(ctx, cfgKYBFlowVendorBusiness, "Marketplace Vendor KYB",
		"/app/vendor/kyb-complete")
	if err != nil {
		return "", "", fmt.Errorf("ensure vendor kyb flow: %w", err)
	}
	return kycFlowID, kybFlowID, nil
}

// EnsureEmployeeKYCFlow creates the Employee Verification hosted KYC flow
// once (idempotent).
func (e *Env) EnsureEmployeeKYCFlow(ctx context.Context) (string, error) {
	return e.ensureKYCFlow(ctx, cfgKYCFlowEmployees, "Employee Verification KYC",
		"/app/employees/kyc-complete")
}

// EnsureAggregatorKYBFlow creates the Agent Network aggregator hosted KYB
// flow once (idempotent).
func (e *Env) EnsureAggregatorKYBFlow(ctx context.Context) (string, error) {
	return e.ensureKYBFlow(ctx, cfgKYBFlowAggregators, "Agent Network Aggregator KYB",
		"/app/agents/aggregators/kyb-complete")
}

func (e *Env) ensureKYCFlow(ctx context.Context, cfgKey, name, redirectPath string) (string, error) {
	if id, _ := getConfig(e.DB, cfgKey); id != "" {
		return id, nil
	}
	resp, err := e.Ninja.CreateFlow(ctx, ninja.CreateFlowRequest{
		Name:        name,
		IDTypes:     []string{"nin", "bvn"},
		RedirectURL: e.PublicURL + redirectPath,
		WebhookURL:  e.PublicURL + "/webhooks/ninja",
	})
	if err != nil {
		return "", err
	}
	if err := setConfig(e.DB, cfgKey, resp.ID); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (e *Env) ensureKYBFlow(ctx context.Context, cfgKey, name, redirectPath string) (string, error) {
	if id, _ := getConfig(e.DB, cfgKey); id != "" {
		return id, nil
	}
	resp, err := e.Ninja.CreateKYBFlow(ctx, ninja.CreateKYBFlowRequest{
		Name:                  name,
		RequireDocuments:      true,
		RequireProofOfAddress: true,
		RequireDirectorIDs:    true,
		GlobalAML:             true,
		RedirectURL:           e.PublicURL + redirectPath,
		WebhookURL:            e.PublicURL + "/webhooks/ninja",
	})
	if err != nil {
		return "", err
	}
	if err := setConfig(e.DB, cfgKey, resp.ID); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// ToggleMockMode switches between live Sandbox API and high-fidelity offline simulator.
// POST /app/toggle-mock-mode
func (e *Env) ToggleMockMode(w http.ResponseWriter, r *http.Request) {
	newVal := !e.Ninja.IsMockMode()
	e.Ninja.SetMockMode(newVal)
	returnTo := r.FormValue("return_to")
	if returnTo == "" {
		returnTo = r.Header.Get("Referer")
	}
	if returnTo == "" {
		returnTo = "/"
	}
	modeStr := "⚡ Offline Simulator Active"
	if !newVal {
		modeStr = "🟢 Live Ninja Sandbox Active"
	}
	http.Redirect(w, r, returnTo+"?flash="+modeStr, http.StatusSeeOther)
}

// ActivityStreamAPI returns recent API calls for the live stream widget.
// GET /app/api/activity-stream
func (e *Env) ActivityStreamAPI(w http.ResponseWriter, r *http.Request) {
	logs, err := db.ListRecentAPILogs(e.DB, 20)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch logs"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}
