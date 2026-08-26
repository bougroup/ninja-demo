package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// EmployeeApplyForm renders the candidate check form — HR can run it as an
// instant dashboard lookup or send a hosted selfie/liveness link for remote
// candidates. GET /app/employees/apply
var employeeSteps = []string{"Apply", "Verify identity", "Cleared"}

func (e *Env) EmployeeApplyForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "employee_apply.html", map[string]any{
		"Stepper": StepperData{Steps: employeeSteps, Current: 0},
	})
}

// EmployeeApplySubmit either calls POST /api/identity/identify directly
// (dashboard_lookup — server-side, against the sandbox) or generates a
// one-time hosted KYC link with a selfie + liveness check (hosted_link).
// POST /app/employees/apply
func (e *Env) EmployeeApplySubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	fullName := r.FormValue("full_name")
	role := r.FormValue("role")
	email := r.FormValue("email")
	idType := r.FormValue("id_type")
	idNumber := r.FormValue("id_number")
	dob := r.FormValue("date_of_birth")
	checkMethod := r.FormValue("check_method")
	if fullName == "" || idType == "" || checkMethod == "" {
		flash(w, r, "/app/employees/apply", "name, ID type, and check method are required")
		return
	}

	candidateID := uuid.NewString()
	if err := db.InsertCandidate(e.DB, candidateID, fullName, role, idType, idNumber, checkMethod); err != nil {
		log.Printf("insert candidate: %v", err)
		http.Error(w, "could not save candidate", http.StatusInternalServerError)
		return
	}

	if checkMethod == "dashboard_lookup" {
		first, last := splitName(fullName)
		result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
			IDType:      idType,
			Mode:        "verify",
			IDNumber:    idNumber,
			FirstName:   first,
			LastName:    last,
			DateOfBirth: dob,
			Reference:   candidateID,
		})
		if err != nil {
			flash(w, r, "/app/employees", "identify failed: "+err.Error())
			return
		}
		raw, _ := json.Marshal(result)
		status := "mismatch"
		if result.Found && result.Verified {
			status = "cleared"
		}
		_ = db.UpdateCandidateDashboardResult(e.DB, candidateID, status, string(raw))
		flash(w, r, "/app/employees", fullName+": "+status)
		return
	}

	// hosted_link
	kycFlowID, err := e.EnsureEmployeeKYCFlow(ctx)
	if err != nil {
		log.Printf("ensure employee kyc flow: %v", err)
		flash(w, r, "/app/employees", "ninja sandbox unavailable: "+err.Error())
		return
	}
	link, err := e.Ninja.CreateFlowLink(ctx, kycFlowID, ninja.CreateFlowLinkRequest{
		CustomerName:  fullName,
		CustomerEmail: email,
		CustomerRef:   candidateID,
	})
	if err != nil {
		log.Printf("create employee kyc link: %v", err)
		flash(w, r, "/app/employees", "kyc link creation failed: "+err.Error())
		return
	}
	_ = db.UpdateCandidateKYCLink(e.DB, candidateID, kycFlowID, link.ID, link.URL)
	http.Redirect(w, r, "/app/employees/"+candidateID, http.StatusSeeOther)
}

// EmployeesDashboard lists every candidate checked so far.
// GET /app/employees
func (e *Env) EmployeesDashboard(w http.ResponseWriter, r *http.Request) {
	candidates, err := db.ListCandidates(e.DB)
	if err != nil {
		log.Printf("list candidates: %v", err)
	}
	render(w, r, "employees_dashboard.html", map[string]any{
		"Candidates": candidates,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

// EmployeeDetail shows one candidate, including the live hosted KYC
// verification detail when applicable.
// GET /app/employees/{id}
func (e *Env) EmployeeDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	candidate, err := db.GetCandidate(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var live any
	if candidate.KYCVerificationID.Valid {
		if v, err := e.Ninja.GetVerification(ctx, candidate.KYCVerificationID.String); err != nil {
			log.Printf("get verification: %v", err)
		} else {
			live = v
		}
	}

	step := 1
	if candidate.Status == "cleared" || candidate.Status == "mismatch" || candidate.Status == "failed" {
		step = 2
	}
	e.render(w, r, "employee_detail.html", map[string]any{
		"Candidate": candidate,
		"Live":      live,
		"Stepper":   StepperData{Steps: employeeSteps, Current: step},
		"Flash":     r.URL.Query().Get("flash"),
	})
}

// EmployeeApprovePayroll unlocks monthly salary disbursement when candidate is cleared.
// POST /app/employees/{id}/approve-payroll
func (e *Env) EmployeeApprovePayroll(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	candidate, err := db.GetCandidate(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if candidate.Status != "cleared" {
		flash(w, r, "/app/employees/"+id, "Payroll Locked: Candidate must clear identity & liveness check before payroll enrollment.")
		return
	}

	if err := db.UpdateCandidatePayrollStatus(e.DB, id, "active"); err != nil {
		log.Printf("update payroll: %v", err)
	}

	flash(w, r, "/app/employees/"+id, "Payroll Enrolled: Monthly salary of ₦850,000/mo activated for direct deposit.")
}

// EmployeeSimulateLiveness simulates a successful selfie/liveness webhook event.
// POST /app/employees/{id}/simulate-liveness
func (e *Env) EmployeeSimulateLiveness(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	candidate, err := db.GetCandidate(e.DB, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	verID := id
	if candidate.KYCVerificationID.Valid {
		verID = candidate.KYCVerificationID.String
	}
	_ = db.UpdateCandidateKYCOutcome(e.DB, verID, "passed", 0.99, `{"verified":true,"liveness_score":0.99,"facematch":true,"anti_spoofing":"passed"}`)

	flash(w, r, "/app/employees/"+id, "Simulated Liveness Cleared: 99% biometric match confirmed.")
}

// EmployeeFlowComplete is the redirect_url target after a hosted KYC session finishes.
func (e *Env) EmployeeFlowComplete(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/app/employees/apply?flash=verification+submitted%2C+check+your+email+for+updates", http.StatusSeeOther)
}
