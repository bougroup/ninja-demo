package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// AgentRecruitForm renders the field-recruitment form.
// GET /app/agents/recruit
func (e *Env) AgentRecruitForm(w http.ResponseWriter, r *http.Request) {
	e.render(w, r, "agent_recruit.html", nil)
}

// AgentRecruitSubmit calls POST /api/identity/identify (verify mode) —
// server-side, against the sandbox — verifying a prospective agent in real
// time, and flags rather than silently rejects the "one BVN behind fifty
// terminals" pattern: an identity already tied to another terminal.
// POST /app/agents/recruit
func (e *Env) AgentRecruitSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	fullName := r.FormValue("full_name")
	dob := r.FormValue("date_of_birth")
	idType := r.FormValue("id_type")
	idNumber := r.FormValue("id_number")
	terminalID := r.FormValue("terminal_id")
	if fullName == "" || idType == "" || idNumber == "" || terminalID == "" {
		flash(w, r, "/app/agents/recruit", "name, ID, and terminal ID are required")
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
		flash(w, r, "/app/agents/recruit", "identity check failed: "+err.Error())
		return
	}
	raw, _ := json.Marshal(result)

	status := "active"
	if !result.Found || !result.Verified {
		status = "blocked_mismatch"
	} else if dupes, err := db.CountAgentsByIdentity(e.DB, idType, idNumber); err == nil && dupes > 0 {
		status = "flagged_duplicate_bvn"
	}

	agentID := uuid.NewString()
	if err := db.InsertAgent(e.DB, agentID, fullName, dob, idType, idNumber, terminalID, status, string(raw)); err != nil {
		log.Printf("insert agent: %v", err)
		flash(w, r, "/app/agents/recruit", "could not save agent")
		return
	}

	flash(w, r, "/app/agents", fullName+" recruited: "+status)
}

// AgentsDashboard lists agents, POS fleet status, and aggregators together.
// GET /app/agents
func (e *Env) AgentsDashboard(w http.ResponseWriter, r *http.Request) {
	agents, err := db.ListAgents(e.DB)
	if err != nil {
		log.Printf("list agents: %v", err)
	}
	aggregators, err := db.ListAggregators(e.DB)
	if err != nil {
		log.Printf("list aggregators: %v", err)
	}

	dupesById := map[string][]*db.Agent{}
	for _, a := range agents {
		if dupeList, err := db.ListAgentsByIdentity(e.DB, a.IDType, a.IDNumber); err == nil && len(dupeList) > 1 {
			dupesById[a.IDNumber] = dupeList
		}
	}

	e.render(w, r, "agents_dashboard.html", map[string]any{
		"Agents":      agents,
		"Aggregators": aggregators,
		"DupesById":   dupesById,
		"Flash":       r.URL.Query().Get("flash"),
	})
}

// AgentSimulateFraudRing simulates deploying 3 POS terminals to the same BVN to demonstrate fraud detection.
// POST /app/agents/simulate-fraud-ring
func (e *Env) AgentSimulateFraudRing(w http.ResponseWriter, r *http.Request) {
	bvn := "77777777777"
	name := "James Bond"
	dob := "1975-01-01"

	terminals := []string{"POS-LAG-901", "POS-IBD-902", "POS-ABJ-903"}
	for _, term := range terminals {
		agentID := uuid.NewString()
		_ = db.InsertAgent(e.DB, agentID, name, dob, "bvn", bvn, term, "flagged_duplicate_bvn", `{"verified":true,"fraud_alert":"multi_terminal_duplicate_bvn"}`)
	}

	flash(w, r, "/app/agents", "Fraud Ring Simulation Triggered: Same BVN (77777777777) deployed across 3 POS terminals — Multi-Terminal Fraud Alarm Active!")
}

// AgentReactivate re-runs identity verification before reactivating a
// dormant/deactivated agent — "re-run the same check before reactivating a
// dormant agent" from the solution page. POST /app/agents/{id}/reactivate
func (e *Env) AgentReactivate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	agent, err := db.GetAgent(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	first, last := splitName(agent.FullName)
	dob := ""
	if agent.DateOfBirth.Valid {
		dob = agent.DateOfBirth.String
	}
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      agent.IDType,
		Mode:        "verify",
		IDNumber:    agent.IDNumber,
		FirstName:   first,
		LastName:    last,
		DateOfBirth: dob,
		Reference:   uuid.NewString(),
	})
	if err != nil {
		flash(w, r, "/app/agents", "Reactivation check failed upstream: "+err.Error())
		return
	}
	raw, _ := json.Marshal(result)

	status := "blocked_mismatch"
	if result.Found && result.Verified {
		status = "active"
	}
	if err := db.UpdateAgentReverification(e.DB, id, status, string(raw)); err != nil {
		log.Printf("update agent reverification: %v", err)
	}
	flash(w, r, "/app/agents", fmt.Sprintf("%s Reactivation Check Cleared: Cash Float of ₦2,000,000 Restored.", agent.FullName))
}

// AgentDeactivate marks an agent deactivated/dormant.
// POST /app/agents/{id}/deactivate
func (e *Env) AgentDeactivate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := db.UpdateAgentStatus(e.DB, id, "dormant"); err != nil {
		log.Printf("deactivate agent: %v", err)
	}
	flash(w, r, "/app/agents", "POS Terminal Marked Dormant: Float suspended until re-verification.")
}

var aggregatorSteps = []string{"Apply", "Verify business", "Active"}

// AggregatorApplyForm renders the aggregator business application form.
// GET /app/agents/aggregators/apply
func (e *Env) AggregatorApplyForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "aggregator_apply.html", map[string]any{
		"Stepper": StepperData{Steps: aggregatorSteps, Current: 0},
	})
}

// AggregatorApplySubmit pulls the aggregator's CAC record (POST
// /api/company/lookup) and sends a one-time hosted KYB link (POST
// /api/kyb-flows/:id/links) — the "send aggregators hosted KYB links" step
// from the Agent Network KYC solution. POST /app/agents/aggregators/apply
func (e *Env) AggregatorApplySubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	businessName := r.FormValue("business_name")
	rcNumber := r.FormValue("rc_number")
	contactName := r.FormValue("contact_name")
	contactEmail := r.FormValue("contact_email")
	if businessName == "" || rcNumber == "" || contactName == "" || contactEmail == "" {
		flash(w, r, "/app/agents/aggregators/apply", "all fields are required")
		return
	}

	aggregatorID := uuid.NewString()
	if err := db.InsertAggregator(e.DB, aggregatorID, businessName, rcNumber, contactName, contactEmail); err != nil {
		log.Printf("insert aggregator: %v", err)
		http.Error(w, "could not save aggregator", http.StatusInternalServerError)
		return
	}

	lookup, err := e.Ninja.CompanyLookup(ctx, ninja.CompanyLookupRequest{RCNumber: rcNumber, Reference: aggregatorID})
	if err != nil {
		log.Printf("company lookup: %v", err)
		flash(w, r, "/app/agents/aggregators/"+aggregatorID, "company lookup failed: "+err.Error())
		return
	}
	lookupRaw, _ := json.Marshal(lookup)
	_ = db.UpdateAggregatorCompanyLookup(e.DB, aggregatorID, string(lookupRaw))

	kybFlowID, err := e.EnsureAggregatorKYBFlow(ctx)
	if err != nil {
		log.Printf("ensure aggregator kyb flow: %v", err)
		flash(w, r, "/app/agents/aggregators/"+aggregatorID, "ninja sandbox unavailable: "+err.Error())
		return
	}

	link, err := e.Ninja.CreateKYBFlowLink(ctx, kybFlowID, ninja.CreateKYBFlowLinkRequest{
		ExpectedName: businessName,
		ContactName:  contactName,
		ContactEmail: contactEmail,
		CustomerRef:  aggregatorID,
	})
	if err != nil {
		log.Printf("create aggregator kyb link: %v", err)
		flash(w, r, "/app/agents/aggregators/"+aggregatorID, "kyb link creation failed: "+err.Error())
		return
	}
	_ = db.UpdateAggregatorKYBFlowLink(e.DB, aggregatorID, kybFlowID, link.ID, link.URL)

	http.Redirect(w, r, "/app/agents/aggregators/"+aggregatorID, http.StatusSeeOther)
}

// AggregatorStatus shows an aggregator's verification status.
// GET /app/agents/aggregators/{id}
func (e *Env) AggregatorStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	aggregator, err := db.GetAggregator(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	step := 1
	if aggregator.Status == "payout_eligible" || aggregator.Status == "active" {
		step = 2
	}
	render(w, r, "aggregator_status.html", map[string]any{
		"Aggregator": aggregator,
		"Stepper":    StepperData{Steps: aggregatorSteps, Current: step},
	})
}

// AggregatorFlowComplete is the redirect_url target after a hosted KYB
// session finishes. GET /app/agents/aggregators/kyb-complete
func (e *Env) AggregatorFlowComplete(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/app/agents/aggregators/apply?flash=verification+submitted%2C+check+your+email+for+updates", http.StatusSeeOther)
}
