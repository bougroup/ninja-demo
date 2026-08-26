package ninja

import "encoding/json"

// WebhookEnvelope is the outer shape of every Ninja webhook delivery. The
// event-specific fields live in Data — some tenants get them flattened onto
// the envelope instead, so callers should fall back to unmarshalling the raw
// body directly into the specific payload type if Data is empty.
type WebhookEnvelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// VerificationCompletedPayload is the body for the "verification.completed"
// event (hosted KYC — one per director/candidate). The REST resource this
// mirrors (GET /api/verifications/:id) confirmed against the live sandbox
// uses "id", not "verification_id" — but the webhook payload itself is
// unverified (this demo has never received a real delivery), so
// extractVerificationID below tries both keys defensively.
type VerificationCompletedPayload struct {
	VerificationID string       `json:"verification_id"`
	FlowID         string       `json:"flow_id"`
	CustomerRef    string       `json:"customer_ref"`
	Outcome        string       `json:"outcome"`
	Score          float64      `json:"score"`
	FaceScore      float64      `json:"face_score"`
	IDType         string       `json:"id_type"`
	Fields         []MatchField `json:"fields"`
	Sandbox        bool         `json:"sandbox"`
}

// KYBVerificationCompletedPayload is the body for the
// "kyb_verification.completed" event (hosted KYB — one per vendor/aggregator).
// Same caveat as VerificationCompletedPayload: unverified against a real
// delivery, and the confirmed REST resource uses "id"/"custom_values" rather
// than "verification_id"/"custom_fields".
type KYBVerificationCompletedPayload struct {
	VerificationID string         `json:"verification_id"`
	FlowID         string         `json:"flow_id"`
	CustomerRef    string         `json:"customer_ref"`
	Outcome        string         `json:"outcome"`
	RCNumber       string         `json:"rc_number"`
	CompanyName    string         `json:"company_name"`
	CustomFields   map[string]any `json:"custom_fields"`
	Sandbox        bool           `json:"sandbox"`
	AML            []AMLHit       `json:"aml"`
}

// ExtractVerificationID pulls the verification id out of a raw webhook
// payload, trying both the documented "verification_id" key and the "id"
// key the confirmed REST resources actually use — whichever is present.
func ExtractVerificationID(raw json.RawMessage) string {
	var probe struct {
		ID             string `json:"id"`
		VerificationID string `json:"verification_id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	if probe.VerificationID != "" {
		return probe.VerificationID
	}
	return probe.ID
}

// IsPass normalizes the free-form "outcome" string Ninja sends into a
// pass/fail verdict for the demo's payout-eligibility decision.
func IsPass(outcome string) bool {
	switch outcome {
	case "pass", "passed", "approved", "verified", "success", "clear":
		return true
	default:
		return false
	}
}
