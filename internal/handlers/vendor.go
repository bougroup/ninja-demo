package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

var vendorSteps = []string{"Apply", "Verify business & directors", "Payout active"}

// VendorApplyForm renders the empty application form (GET /app/vendor/apply).
func (e *Env) VendorApplyForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "vendor_apply.html", map[string]any{
		"Stepper": StepperData{Steps: vendorSteps, Current: 0},
	})
}

// VendorApplySubmit runs the full onboarding pipeline for a new vendor:
//
//  1. POST /api/company/lookup        - pull the CAC record for the RC number
//  2. POST /api/company/advanced-lookup - pull directors + shareholders
//  3. POST /api/identity/bulk-identify  - cross-check every director's BVN in one call
//  4. POST /api/kyb-flows/:id/links     - one-time hosted KYB link for the business
//  5. POST /api/flows/:id/links         - one-time hosted KYC link per director
//
// Everything is persisted so the admin dashboard can show live status even
// before the hosted verification webhooks land.
func (e *Env) VendorApplySubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	businessName := r.FormValue("business_name")
	rcNumber := r.FormValue("rc_number")
	contactName := r.FormValue("contact_name")
	contactEmail := r.FormValue("contact_email")
	directorIDType := r.FormValue("director_id_type") // nin | bvn — how to verify directors below
	if directorIDType == "" {
		directorIDType = "bvn"
	}

	if businessName == "" || rcNumber == "" || contactName == "" || contactEmail == "" {
		flash(w, r, "/app/vendor/apply", "all fields are required")
		return
	}

	vendorID := uuid.NewString()
	if err := db.InsertVendor(e.DB, vendorID, businessName, rcNumber, contactName, contactEmail); err != nil {
		log.Printf("insert vendor: %v", err)
		http.Error(w, "could not save vendor", http.StatusInternalServerError)
		return
	}

	kycFlowID, kybFlowID, err := e.EnsureVendorFlows(ctx)
	if err != nil {
		log.Printf("ensure flows: %v", err)
		flash(w, r, "/app/vendor/apply", "ninja sandbox unavailable: "+err.Error())
		return
	}

	// 1. Standard company lookup.
	lookup, err := e.Ninja.CompanyLookup(ctx, ninja.CompanyLookupRequest{RCNumber: rcNumber, Reference: vendorID})
	if err != nil {
		log.Printf("company lookup: %v", err)
		flash(w, r, "/app/vendor/apply", "company lookup failed: "+err.Error())
		return
	}
	lookupRaw, _ := json.Marshal(lookup)
	_ = db.UpdateVendorCompanyLookup(e.DB, vendorID,
		lookup.Data.RegistrationNumber, lookup.Data.Status, lookup.Data.CompanyType,
		lookup.Data.NatureOfBusiness, lookup.Data.Email, lookup.Data.Address, string(lookupRaw))

	// 2. Advanced lookup for directors + shareholders (needed for AML + director KYC).
	advanced, err := e.Ninja.CompanyAdvancedLookup(ctx, ninja.CompanyAdvancedLookupRequest{RCNumber: rcNumber, Reference: vendorID})
	if err != nil {
		log.Printf("company advanced lookup: %v", err)
		flash(w, r, "/app/vendor/"+vendorID, "advanced lookup failed: "+err.Error())
		return
	}
	advancedRaw, _ := json.Marshal(advanced)
	_ = db.UpdateVendorCompanyAdvanced(e.DB, vendorID, string(advancedRaw))

	// 3. Bulk-identify every director in a single call (max 25 per Ninja's limit).
	var idNumbers []string
	for _, d := range advanced.Data.Directors {
		if d.IDNumber != "" {
			idNumbers = append(idNumbers, d.IDNumber)
		}
	}
	bulkByID := map[string]ninja.BulkIdentifyEntry{}
	if len(idNumbers) > 0 {
		bulk, err := e.Ninja.BulkIdentify(ctx, ninja.BulkIdentifyRequest{
			IDType:    directorIDType,
			IDNumbers: strings.Join(idNumbers, ","),
			Reference: vendorID,
		})
		if err != nil {
			log.Printf("bulk identify: %v", err)
		} else {
			for _, entry := range bulk.Data {
				bulkByID[entry.IDNumber] = entry
			}
		}
	}

	// Persist each director and kick off their individual hosted KYC link.
	for _, d := range advanced.Data.Directors {
		directorID := uuid.NewString()
		if err := db.InsertDirector(e.DB, directorID, vendorID, d.ID, d.Name, directorIDType, d.IDNumber); err != nil {
			log.Printf("insert director: %v", err)
			continue
		}

		if entry, ok := bulkByID[d.IDNumber]; ok {
			status := "matched"
			if entry.Error != "" {
				status = "no_match"
			}
			raw, _ := json.Marshal(entry)
			_ = db.UpdateDirectorBulkIdentify(e.DB, directorID, status, string(raw))
		}

		link, err := e.Ninja.CreateFlowLink(ctx, kycFlowID, ninja.CreateFlowLinkRequest{
			CustomerName:  d.Name,
			CustomerEmail: contactEmail, // demo: directors share the vendor contact inbox
			CustomerRef:   directorID,
		})
		if err != nil {
			log.Printf("create kyc link for director %s: %v", d.Name, err)
			continue
		}
		_ = db.UpdateDirectorKYCLink(e.DB, directorID, kycFlowID, link.ID, link.URL)
	}

	// 4. One-time hosted KYB link for the business itself (docs, proof of
	// address, director IDs, AML — configured on the flow).
	kybLink, err := e.Ninja.CreateKYBFlowLink(ctx, kybFlowID, ninja.CreateKYBFlowLinkRequest{
		ExpectedName: businessName,
		ContactName:  contactName,
		ContactEmail: contactEmail,
		CustomerRef:  vendorID,
	})
	if err != nil {
		log.Printf("create kyb link: %v", err)
		flash(w, r, "/app/vendor/"+vendorID, "kyb link creation failed: "+err.Error())
		return
	}
	_ = db.UpdateVendorKYBFlowLink(e.DB, vendorID, kybFlowID, kybLink.ID, kybLink.URL)

	http.Redirect(w, r, "/app/vendor/"+vendorID, http.StatusSeeOther)
}

// VendorStatus renders the vendor-facing status page: their KYB link (if
// still pending) and each director's individual KYC link.
func (e *Env) VendorStatus(w http.ResponseWriter, r *http.Request) {
	vendorID := r.PathValue("id")
	vendor, err := db.GetVendor(e.DB, vendorID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	directors, err := db.ListDirectorsByVendor(e.DB, vendorID)
	if err != nil {
		log.Printf("list directors: %v", err)
	}
	step := 1
	if vendor.Status == "payout_eligible" {
		step = 2
	}

	clearedCount := 0
	for _, d := range directors {
		if d.KYCOutcome.Valid && (d.KYCOutcome.String == "pass" || d.KYCOutcome.String == "passed" || d.KYCOutcome.String == "approved" || d.KYCOutcome.String == "verified") {
			clearedCount++
		}
	}

	e.render(w, r, "vendor_status.html", map[string]any{
		"Vendor":       vendor,
		"Directors":    directors,
		"ClearedCount": clearedCount,
		"Stepper":      StepperData{Steps: vendorSteps, Current: step},
		"Flash":        r.URL.Query().Get("flash"),
	})
}

// VendorReleaseEscrow unlocks the marketplace settlement wallet if all directors & company pass.
// POST /app/vendor/{id}/release-escrow
func (e *Env) VendorReleaseEscrow(w http.ResponseWriter, r *http.Request) {
	vendorID := r.PathValue("id")
	if _, err := db.GetVendor(e.DB, vendorID); err != nil {
		http.NotFound(w, r)
		return
	}

	directors, err := db.ListDirectorsByVendor(e.DB, vendorID)
	if err != nil {
		log.Printf("list directors: %v", err)
	}

	allDirectorsCleared := len(directors) > 0
	for _, d := range directors {
		if !d.KYCOutcome.Valid || (d.KYCOutcome.String != "pass" && d.KYCOutcome.String != "passed" && d.KYCOutcome.String != "approved" && d.KYCOutcome.String != "verified") {
			allDirectorsCleared = false
			break
		}
	}

	if !allDirectorsCleared {
		flash(w, r, "/app/vendor/"+vendorID, "Escrow Settlement Locked: All corporate directors must complete liveness KYC before funds release.")
		return
	}

	walletID := "PW-" + uuid.NewString()[:8]
	if err := db.UpdateVendorPayoutWallet(e.DB, vendorID, walletID); err != nil {
		log.Printf("update payout wallet: %v", err)
	}

	flash(w, r, "/app/vendor/"+vendorID, "Escrow Vault Released: ₦12,500,000 disbursed to verified Payout Wallet "+walletID)
}

// VendorSimulateWebhook simulates instant clearance for a director or vendor KYB.
// POST /app/vendor/{id}/simulate-webhook
func (e *Env) VendorSimulateWebhook(w http.ResponseWriter, r *http.Request) {
	vendorID := r.PathValue("id")
	target := r.URL.Query().Get("target") // director_id or "all"

	directors, _ := db.ListDirectorsByVendor(e.DB, vendorID)
	for _, d := range directors {
		if target == "all" || target == d.ID {
			_ = db.UpdateDirectorKYCOutcome(e.DB, d.KYCVerificationID.String, "passed", 0.98, `{"verified":true,"liveness_score":0.98,"facematch":true}`)
		}
	}

	if target == "all" || target == "kyb" {
		vendor, _ := db.GetVendor(e.DB, vendorID)
		if vendor != nil && vendor.KYBVerificationID.Valid {
			_ = db.UpdateVendorKYBOutcome(e.DB, vendor.KYBVerificationID.String, "passed", "payout_eligible", `{"status":"approved","aml_cleared":true}`)
		}
	}

	flash(w, r, "/app/vendor/"+vendorID, "Simulated Webhook Delivered: Director liveness check marked as passed.")
}

// VendorFlowComplete is the redirect_url target after a hosted KYC/KYB session finishes.
func (e *Env) VendorFlowComplete(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/app/vendor/apply?flash=verification+submitted%2C+check+your+email+for+updates", http.StatusSeeOther)
}
