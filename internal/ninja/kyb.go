package ninja

import (
	"context"
	"fmt"
	"net/url"
)

// CreateKYBFlowRequest maps to POST /api/kyb-flows.
type CreateKYBFlowRequest struct {
	Name                  string         `json:"name"`
	RequireDocuments      bool           `json:"require_documents"`
	RequireProofOfAddress bool           `json:"require_proof_of_address"`
	RequireDirectorIDs    bool           `json:"require_director_ids"`
	RequireOTP            bool           `json:"require_otp"`
	GlobalAML             bool           `json:"global_aml"`
	CustomFields          map[string]any `json:"custom_fields,omitempty"`
	Branding              map[string]any `json:"branding,omitempty"`
	RedirectURL           string         `json:"redirect_url,omitempty"`
	WebhookURL            string         `json:"webhook_url,omitempty"`
}

type KYBFlowResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateKYBFlow calls POST /api/kyb-flows.
func (c *Client) CreateKYBFlow(ctx context.Context, req CreateKYBFlowRequest) (*KYBFlowResponse, error) {
	var out KYBFlowResponse
	if err := c.do(ctx, "POST", "/api/kyb-flows", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateKYBFlowLinkRequest maps to POST /api/kyb-flows/:id/links.
type CreateKYBFlowLinkRequest struct {
	ExpectedName string `json:"expected_name"`
	ContactName  string `json:"contact_name"`
	ContactEmail string `json:"contact_email"`
	CustomerRef  string `json:"customer_ref"`
}

type KYBFlowLinkResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status"`
	Sandbox   bool   `json:"sandbox"`
}

// CreateKYBFlowLink calls POST /api/kyb-flows/:id/links.
func (c *Client) CreateKYBFlowLink(ctx context.Context, flowID string, req CreateKYBFlowLinkRequest) (*KYBFlowLinkResponse, error) {
	var out KYBFlowLinkResponse
	path := fmt.Sprintf("/api/kyb-flows/%s/links", url.PathEscape(flowID))
	if err := c.do(ctx, "POST", path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type AMLHit struct {
	List   string  `json:"list"`
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	Status string  `json:"status"`
}

// KYBVerification is a hosted KYB session, as returned by
// GET /api/kyb-verifications and GET /api/kyb-verifications/:id — confirmed
// field names against the live sandbox (bare array/object, id not
// verification_id, custom_values not custom_fields).
type KYBVerification struct {
	ID           string         `json:"id"`
	FlowID       string         `json:"flow_id"`
	BusinessID   string         `json:"business_id"`
	CustomerRef  string         `json:"customer_ref"`
	Status       string         `json:"status"`
	Outcome      string         `json:"outcome"`
	Score        float64        `json:"score"`
	RCNumber     string         `json:"rc_number"`
	CompanyName  string         `json:"company_name"`
	ExpectedName string         `json:"expected_name"`
	ContactName  string         `json:"contact_name"`
	ContactEmail string         `json:"contact_email"`
	CustomValues map[string]any `json:"custom_values"`
	AML          []AMLHit       `json:"aml"`
	ExpiresAt    string         `json:"expires_at"`
	OpenedAt     string         `json:"opened_at"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	Sandbox      bool           `json:"sandbox"`
}

// ListKYBVerificationsFilter maps to the query params on GET /api/kyb-verifications.
type ListKYBVerificationsFilter struct {
	FlowID      string
	Status      string
	CustomerRef string
}

// ListKYBVerifications calls GET /api/kyb-verifications, which returns a
// bare JSON array.
func (c *Client) ListKYBVerifications(ctx context.Context, f ListKYBVerificationsFilter) ([]KYBVerification, error) {
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
	path := "/api/kyb-verifications"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var out []KYBVerification
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetKYBVerification calls GET /api/kyb-verifications/:id.
func (c *Client) GetKYBVerification(ctx context.Context, id string) (*KYBVerification, error) {
	var out KYBVerification
	path := fmt.Sprintf("/api/kyb-verifications/%s", url.PathEscape(id))
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetKYBDocument calls GET /api/kyb-verifications/:id/document/:doc, where
// doc is one of: cert, status_report, memart, proof_of_address,
// director-<cac_id>. Every access is audit-logged by Ninja.
func (c *Client) GetKYBDocument(ctx context.Context, id, doc string) ([]byte, string, error) {
	path := fmt.Sprintf("/api/kyb-verifications/%s/document/%s", url.PathEscape(id), url.PathEscape(doc))
	return c.doRaw(ctx, "GET", path)
}

// ResendKYBVerification calls POST /api/kyb-verifications/:id/resend.
func (c *Client) ResendKYBVerification(ctx context.Context, id string) (*KYBFlowLinkResponse, error) {
	var out KYBFlowLinkResponse
	path := fmt.Sprintf("/api/kyb-verifications/%s/resend", url.PathEscape(id))
	if err := c.do(ctx, "POST", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelKYBVerification calls POST /api/kyb-verifications/:id/cancel.
func (c *Client) CancelKYBVerification(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/kyb-verifications/%s/cancel", url.PathEscape(id))
	return c.do(ctx, "POST", path, nil, nil)
}
