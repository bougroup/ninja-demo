package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

func forwardToWebhookSite(body []byte, event, deliveryID, signature string) {
	webhookSiteURL := os.Getenv("WEBHOOK_SITE_URL")
	if webhookSiteURL == "" {
		webhookSiteURL = "https://webhook.site/be3933b6-2393-4566-b244-8886cf23c65d"
	}
	go func() {
		req, err := http.NewRequest("POST", webhookSiteURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Ninja-Event", event)
		req.Header.Set("X-Ninja-Delivery-Id", deliveryID)
		req.Header.Set("X-Ninja-Signature", signature)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}

// WebhookReceiver handles both "verification.completed" (hosted KYC —
// vendor directors or employee candidates) and "kyb_verification.completed"
// (hosted KYB — vendor businesses or agent-network aggregators) events from
// Ninja. Every delivery is persisted to webhook_events regardless of
// outcome, so the admin dashboard has a full audit trail even for
// deliveries whose signature failed to verify.
func (e *Env) WebhookReceiver(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	secret := os.Getenv("NINJA_WEBHOOK_SECRET")
	signature := r.Header.Get("X-Ninja-Signature")
	signatureOK := secret != "" && ninja.VerifyWebhookSignature(secret, body, signature)

	var envelope ninja.WebhookEnvelope
	_ = json.Unmarshal(body, &envelope) // best-effort; event may be flattened

	event := envelope.Event
	if event == "" {
		// Some deliveries flatten the event name onto the body itself.
		var probe struct {
			Event string `json:"event"`
		}
		_ = json.Unmarshal(body, &probe)
		event = probe.Event
	}

	deliveryID := r.Header.Get("X-Ninja-Delivery-Id")
	eventID := uuid.NewString()
	if err := db.InsertWebhookEvent(e.DB, eventID, event, deliveryID, string(body), signatureOK); err != nil {
		log.Printf("insert webhook event: %v", err)
	}

	forwardToWebhookSite(body, event, deliveryID, signature)

	if !signatureOK {
		log.Printf("webhook %s: signature verification failed, rejecting", eventID)
		_ = db.MarkWebhookEventProcessed(e.DB, eventID, errSignatureInvalid)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	payload := envelope.Data
	if len(payload) == 0 {
		payload = body // flattened delivery: event fields live at the top level
	}

	var procErr error
	switch event {
	case "verification.completed":
		procErr = e.handleVerificationCompleted(payload)
	case "kyb_verification.completed":
		procErr = e.handleKYBVerificationCompleted(payload)
	default:
		log.Printf("webhook %s: unrecognized event %q, storing only", eventID, event)
	}

	_ = db.MarkWebhookEventProcessed(e.DB, eventID, procErr)
	if procErr != nil {
		log.Printf("webhook %s: processing error: %v", eventID, procErr)
		http.Error(w, "processing error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

var errSignatureInvalid = signatureError{}

type signatureError struct{}

func (signatureError) Error() string { return "signature verification failed" }

// handleVerificationCompleted dispatches a "verification.completed" hosted
// KYC event to whichever scenario owns this verification_id: a vendor's
// director (Marketplace Vendor Payouts) or a job candidate (Employee
// Verification).
func (e *Env) handleVerificationCompleted(payload json.RawMessage) error {
	var p ninja.VerificationCompletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	verificationID := ninja.ExtractVerificationID(payload)
	raw, _ := json.Marshal(p)

	if director, err := db.GetDirectorByKYCVerificationID(e.DB, verificationID); err == nil {
		if err := db.UpdateDirectorKYCOutcome(e.DB, verificationID, p.Outcome, p.Score, string(raw)); err != nil {
			return err
		}
		return e.maybeActivatePayout(director.VendorID)
	}

	if _, err := db.GetCandidateByKYCVerificationID(e.DB, verificationID); err == nil {
		return db.UpdateCandidateKYCOutcome(e.DB, verificationID, p.Outcome, p.Score, string(raw))
	}

	log.Printf("verification.completed for unknown verification_id %s: storing event only", verificationID)
	return nil
}

// handleKYBVerificationCompleted dispatches a "kyb_verification.completed"
// hosted KYB event to whichever scenario owns this verification_id: a
// marketplace vendor or an agent-network aggregator.
func (e *Env) handleKYBVerificationCompleted(payload json.RawMessage) error {
	var p ninja.KYBVerificationCompletedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	verificationID := ninja.ExtractVerificationID(payload)
	raw, _ := json.Marshal(p)

	status := "kyb_review"
	if !ninja.IsPass(p.Outcome) {
		status = "rejected"
	}

	if vendor, err := db.GetVendorByKYBVerificationID(e.DB, verificationID); err == nil {
		if err := db.UpdateVendorKYBOutcome(e.DB, verificationID, p.Outcome, status, string(raw)); err != nil {
			return err
		}
		return e.maybeActivatePayout(vendor.ID)
	}

	if _, err := db.GetAggregatorByKYBVerificationID(e.DB, verificationID); err == nil {
		return db.UpdateAggregatorKYBOutcome(e.DB, verificationID, p.Outcome, status, string(raw))
	}

	log.Printf("kyb_verification.completed for unknown verification_id %s: storing event only", verificationID)
	return nil
}

// maybeActivatePayout flips a vendor to payout_eligible (with a mock payout
// wallet id) once both its KYB outcome and every director's KYC outcome have
// passed. This is the practical "why" behind the whole verification pipeline
// — money doesn't move until both checks clear.
func (e *Env) maybeActivatePayout(vendorID string) error {
	vendor, err := db.GetVendor(e.DB, vendorID)
	if err != nil {
		return err
	}
	if !vendor.KYBOutcome.Valid || !ninja.IsPass(vendor.KYBOutcome.String) {
		return nil
	}
	allPassed, err := db.AllDirectorsPassed(e.DB, vendorID)
	if err != nil {
		return err
	}
	if !allPassed {
		return nil
	}
	if vendor.PayoutWalletID.Valid && vendor.PayoutWalletID.String != "" {
		return nil // already activated
	}
	walletID := "wallet_" + uuid.NewString()
	return db.UpdateVendorPayoutWallet(e.DB, vendorID, walletID)
}
