package ninja

import (
	"context"
	"fmt"
	"net/url"
)

// WebhookDelivery is one delivery attempt log, as returned by
// GET /api/webhook-deliveries — confirmed field names against the live
// sandbox (bare array, no "url" or "last_attempt_at" fields).
type WebhookDelivery struct {
	ID           string `json:"id"`
	BusinessID   string `json:"business_id"`
	FlowID       string `json:"flow_id"`
	EventID      string `json:"event_id"`
	Event        string `json:"event"`
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	Attempts     int    `json:"attempts"`
	MaxAttempts  int    `json:"max_attempts"`
	ResponseCode int    `json:"response_code"`
	LastError    string `json:"last_error"`
	NextRetryAt  string `json:"next_retry_at"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ListWebhookDeliveries calls GET /api/webhook-deliveries, which returns a
// bare JSON array.
func (c *Client) ListWebhookDeliveries(ctx context.Context) ([]WebhookDelivery, error) {
	var out []WebhookDelivery
	if err := c.do(ctx, "GET", "/api/webhook-deliveries", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RetryWebhookDelivery calls POST /api/webhook-deliveries/:id/retry.
func (c *Client) RetryWebhookDelivery(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/webhook-deliveries/%s/retry", url.PathEscape(id))
	return c.do(ctx, "POST", path, nil, nil)
}
