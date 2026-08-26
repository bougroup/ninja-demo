package ninja

import (
	"context"
	"fmt"
	"net/url"
)

// CreateFlowRequest maps to POST /api/flows (hosted KYC flow config).
type CreateFlowRequest struct {
	Name        string         `json:"name"`
	IDTypes     []string       `json:"id_types"`
	Rules       map[string]any `json:"rules,omitempty"`
	Branding    map[string]any `json:"branding,omitempty"`
	RedirectURL string         `json:"redirect_url,omitempty"`
	WebhookURL  string         `json:"webhook_url,omitempty"`
}

type FlowResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateFlow calls POST /api/flows.
func (c *Client) CreateFlow(ctx context.Context, req CreateFlowRequest) (*FlowResponse, error) {
	var out FlowResponse
	if err := c.do(ctx, "POST", "/api/flows", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateFlowLinkRequest maps to POST /api/flows/:id/links (one-time customer link).
type CreateFlowLinkRequest struct {
	CustomerName  string         `json:"customer_name"`
	CustomerEmail string         `json:"customer_email"`
	CustomerRef   string         `json:"customer_ref"`
	Values        map[string]any `json:"values,omitempty"`
}

type FlowLinkResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status"`
	Sandbox   bool   `json:"sandbox"`
}

// CreateFlowLink calls POST /api/flows/:id/links.
func (c *Client) CreateFlowLink(ctx context.Context, flowID string, req CreateFlowLinkRequest) (*FlowLinkResponse, error) {
	var out FlowLinkResponse
	path := fmt.Sprintf("/api/flows/%s/links", url.PathEscape(flowID))
	if err := c.do(ctx, "POST", path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MatchField is one field's per-field name-matching result, returned both by
// POST /api/identity/identify (verify mode) and embedded in hosted KYC
// verification records — confirmed against the live sandbox response shape:
// {"field":"last_name","score":0,"match":"mismatch","provided":"Oko","detail":"does not match the record"}
type MatchField struct {
	Field    string  `json:"field"`
	Score    float64 `json:"score"`
	Match    string  `json:"match"` // exact | mismatch | ...
	Provided string  `json:"provided"`
	Detail   string  `json:"detail,omitempty"`
}

// Verification is a hosted KYC session, as returned by GET /api/verifications
// and GET /api/verifications/:id (confirmed field names against the live
// sandbox — the endpoint returns a bare array/object, not {"data": ...}).
type Verification struct {
	ID            string         `json:"id"`
	FlowID        string         `json:"flow_id"`
	BusinessID    string         `json:"business_id"`
	CustomerRef   string         `json:"customer_ref"`
	Status        string         `json:"status"`
	Outcome       string         `json:"outcome"`
	Score         float64        `json:"score"`
	FaceScore     float64        `json:"face_score"`
	IDType        string         `json:"id_type"`
	IDNumber      string         `json:"id_number"`
	CustomerName  string         `json:"customer_name"`
	CustomerEmail string         `json:"customer_email"`
	Values        map[string]any `json:"values"`
	Fields        []MatchField   `json:"fields"`
	ExpiresAt     string         `json:"expires_at"`
	OpenedAt      string         `json:"opened_at"`
	CompletedAt   string         `json:"completed_at"`
	ConsentAt     string         `json:"consent_at"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	Sandbox       bool           `json:"sandbox"`
}

// ListVerificationsFilter maps to the query params on GET /api/verifications.
type ListVerificationsFilter struct {
	FlowID      string
	Status      string
	CustomerRef string
}

// ListVerifications calls GET /api/verifications, which returns a bare JSON
// array.
func (c *Client) ListVerifications(ctx context.Context, f ListVerificationsFilter) ([]Verification, error) {
	q := url.Values{}
	if f.FlowID != "" {
		q.Set("flow_id", f.FlowID)
	}
	if f.Status != "" {
		q.Set("status", f.Status)
	}
	if f.CustomerRef != "" {
		q.Set("customer_ref", f.CustomerRef)
	}
	path := "/api/verifications"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out []Verification
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetVerification calls GET /api/verifications/:id.
func (c *Client) GetVerification(ctx context.Context, id string) (*Verification, error) {
	var out Verification
	path := fmt.Sprintf("/api/verifications/%s", url.PathEscape(id))
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVerificationSelfie calls GET /api/verifications/:id/selfie, returning the
// raw image bytes and content type (audit-logged by Ninja on every access).
func (c *Client) GetVerificationSelfie(ctx context.Context, id string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/verifications/%s/selfie", url.PathEscape(id))
	return c.doRaw(ctx, "GET", path)
}

// ResendVerification calls POST /api/verifications/:id/resend (rotates the
// link token and restarts its expiry).
func (c *Client) ResendVerification(ctx context.Context, id string) (*FlowLinkResponse, error) {
	var out FlowLinkResponse
	path := fmt.Sprintf("/api/verifications/%s/resend", url.PathEscape(id))
	if err := c.do(ctx, "POST", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelVerification calls POST /api/verifications/:id/cancel.
func (c *Client) CancelVerification(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/verifications/%s/cancel", url.PathEscape(id))
	return c.do(ctx, "POST", path, nil, nil)
}
