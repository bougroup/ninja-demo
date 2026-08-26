package handlers

import (
	"log"
	"net/http"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
)

// AdminDashboard lists every vendor with its local status, plus a live pull
// of everything Ninja itself has on file for the two flows
// (GET /api/kyb-verifications and GET /api/verifications) — useful for
// spotting sessions the local webhook handler hasn't reconciled yet.
// GET /app/admin
func (e *Env) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vendors, err := db.ListVendors(e.DB)
	if err != nil {
		log.Printf("list vendors: %v", err)
		http.Error(w, "could not load vendors", http.StatusInternalServerError)
		return
	}

	var kybVerifications []ninja.KYBVerification
	if kybFlowID, _ := getConfig(e.DB, cfgKYBFlowVendorBusiness); kybFlowID != "" {
		if v, err := e.Ninja.ListKYBVerifications(ctx, ninja.ListKYBVerificationsFilter{FlowID: kybFlowID}); err != nil {
			log.Printf("list kyb verifications: %v", err)
		} else {
			kybVerifications = v
		}
	}

	var kycVerifications []ninja.Verification
	if kycFlowID, _ := getConfig(e.DB, cfgKYCFlowVendorDirectors); kycFlowID != "" {
		if v, err := e.Ninja.ListVerifications(ctx, ninja.ListVerificationsFilter{FlowID: kycFlowID}); err != nil {
			log.Printf("list verifications: %v", err)
		} else {
			kycVerifications = v
		}
	}

	render(w, r, "admin_dashboard.html", map[string]any{
		"Vendors":          vendors,
		"KYBVerifications": kybVerifications,
		"KYCVerifications": kycVerifications,
		"Flash":            r.URL.Query().Get("flash"),
	})
}

// AdminVendorDetail shows the full picture for one vendor: company record,
// directors + shareholders, live KYB verification detail pulled straight
// from Ninja, and each director's live KYC verification detail.
// GET /app/admin/vendors/{id}
func (e *Env) AdminVendorDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	// Live GET /api/kyb-verifications/:id — refreshes outcome/AML straight
	// from Ninja rather than relying only on whatever the webhook already
	// wrote locally.
	var kybLive any
	if vendor.KYBVerificationID.Valid {
		if v, err := e.Ninja.GetKYBVerification(ctx, vendor.KYBVerificationID.String); err != nil {
			log.Printf("get kyb verification: %v", err)
		} else {
			kybLive = v
		}
	}

	type directorView struct {
		*db.Director
		Live any
	}
	directorViews := make([]directorView, 0, len(directors))
	for _, d := range directors {
		dv := directorView{Director: d}
		if d.KYCVerificationID.Valid {
			if v, err := e.Ninja.GetVerification(ctx, d.KYCVerificationID.String); err != nil {
				log.Printf("get verification %s: %v", d.KYCVerificationID.String, err)
			} else {
				dv.Live = v
			}
		}
		directorViews = append(directorViews, dv)
	}

	render(w, r, "admin_vendor_detail.html", map[string]any{
		"Vendor":    vendor,
		"KYBLive":   kybLive,
		"Directors": directorViews,
		"Flash":     r.URL.Query().Get("flash"),
	})
}

// AdminVerificationSelfie proxies GET /api/verifications/:id/selfie so the
// browser can render the image without exposing the Ninja bearer token.
// GET /app/admin/verifications/{id}/selfie
func (e *Env) AdminVerificationSelfie(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, contentType, err := e.Ninja.GetVerificationSelfie(r.Context(), id)
	if err != nil {
		http.Error(w, "selfie unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Write(data)
}

// AdminKYBDocument proxies GET /api/kyb-verifications/:id/document/:doc.
// GET /app/admin/kyb-verifications/{id}/document/{doc}
func (e *Env) AdminKYBDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc := r.PathValue("doc")
	data, contentType, err := e.Ninja.GetKYBDocument(r.Context(), id, doc)
	if err != nil {
		http.Error(w, "document unavailable: "+err.Error(), http.StatusBadGateway)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Write(data)
}

// returnTo resolves where a resend/cancel action should redirect back to:
// an explicit return_to query param, or (for backward compatibility with
// the vendor detail page) a vendor_id query param, falling back to the
// admin dashboard.
func returnTo(r *http.Request) string {
	if rt := r.URL.Query().Get("return_to"); rt != "" {
		return rt
	}
	if vendorID := r.URL.Query().Get("vendor_id"); vendorID != "" {
		return "/app/admin/vendors/" + vendorID
	}
	return "/app/admin"
}

// AdminResendKYC calls POST /api/verifications/:id/resend. Reused by both
// the vendor-director flow and the employee-candidate hosted-link flow.
// POST /app/admin/verifications/{id}/resend
func (e *Env) AdminResendKYC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dest := returnTo(r)
	link, err := e.Ninja.ResendVerification(r.Context(), id)
	if err != nil {
		flash(w, r, dest, "resend failed: "+err.Error())
		return
	}
	flash(w, r, dest, "verification link resent: "+link.URL)
}

// AdminCancelKYC calls POST /api/verifications/:id/cancel.
// POST /app/admin/verifications/{id}/cancel
func (e *Env) AdminCancelKYC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dest := returnTo(r)
	if err := e.Ninja.CancelVerification(r.Context(), id); err != nil {
		flash(w, r, dest, "cancel failed: "+err.Error())
		return
	}
	flash(w, r, dest, "verification cancelled")
}

// AdminResendKYB calls POST /api/kyb-verifications/:id/resend. Reused by
// both the vendor and agent-network aggregator flows.
// POST /app/admin/kyb-verifications/{id}/resend
func (e *Env) AdminResendKYB(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dest := returnTo(r)
	link, err := e.Ninja.ResendKYBVerification(r.Context(), id)
	if err != nil {
		flash(w, r, dest, "resend failed: "+err.Error())
		return
	}
	flash(w, r, dest, "kyb link resent: "+link.URL)
}

// AdminCancelKYB calls POST /api/kyb-verifications/:id/cancel. If the
// cancelled verification belongs to a vendor, its local status is flipped
// to rejected too (aggregators are left for ops to review manually).
// POST /app/admin/kyb-verifications/{id}/cancel
func (e *Env) AdminCancelKYB(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dest := returnTo(r)
	if err := e.Ninja.CancelKYBVerification(r.Context(), id); err != nil {
		flash(w, r, dest, "cancel failed: "+err.Error())
		return
	}
	if vendorID := r.URL.Query().Get("vendor_id"); vendorID != "" {
		_ = db.UpdateVendorStatus(e.DB, vendorID, "rejected")
	}
	flash(w, r, dest, "kyb verification cancelled")
}

// AdminWebhookDeliveries lists GET /api/webhook-deliveries alongside the
// locally received events (which include ones that failed signature checks
// and therefore never reached Ninja's own delivery log).
// GET /app/admin/webhooks
func (e *Env) AdminWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	deliveries, err := e.Ninja.ListWebhookDeliveries(r.Context())
	if err != nil {
		log.Printf("list webhook deliveries: %v", err)
	}
	localEvents, err := db.ListWebhookEvents(e.DB, 50)
	if err != nil {
		log.Printf("list webhook events: %v", err)
	}
	var deliveryList []any
	for _, d := range deliveries {
		deliveryList = append(deliveryList, d)
	}
	render(w, r, "admin_webhooks.html", map[string]any{
		"Deliveries":  deliveryList,
		"LocalEvents": localEvents,
		"Flash":       r.URL.Query().Get("flash"),
	})
}

// AdminRetryWebhookDelivery calls POST /api/webhook-deliveries/:id/retry.
// POST /app/admin/webhooks/{id}/retry
func (e *Env) AdminRetryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := e.Ninja.RetryWebhookDelivery(r.Context(), id); err != nil {
		flash(w, r, "/app/admin/webhooks", "retry failed: "+err.Error())
		return
	}
	flash(w, r, "/app/admin/webhooks", "retry queued for delivery "+id)
}
