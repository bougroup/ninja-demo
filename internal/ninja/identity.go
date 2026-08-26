package ninja

import "context"

// IdentifyRequest maps to POST /api/identity/identify.
// Mode "lookup" returns the full registry record; "verify" checks the
// supplied name/DOB against the registry and returns a match verdict.
type IdentifyRequest struct {
	IDType      string `json:"idType"` // nin | bvn | ndl
	Mode        string `json:"mode"`   // lookup | verify
	IDNumber    string `json:"idNumber"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	DateOfBirth string `json:"dateOfBirth,omitempty"` // required for verify mode
	Reference   string `json:"reference,omitempty"`
}

type IdentifyData struct {
	IDNumber     string `json:"id_number"`
	Type         string `json:"type"`
	FirstName    string `json:"first_name"`
	MiddleName   string `json:"middle_name"`
	LastName     string `json:"last_name"`
	DateOfBirth  string `json:"date_of_birth"`
	Gender       string `json:"gender"`
	Mobile       string `json:"mobile"`
	AddressState string `json:"address_state"`
	Image        string `json:"image"`
}

// IdentifyResponse covers both modes. Lookup mode populates Status/Data;
// verify mode populates the rest, with per-field match results in Fields —
// confirmed to use the same {field,score,match,provided,detail} shape as
// hosted KYC verification records (see MatchField in kyc.go), rather than
// a separate "mismatches" list.
type IdentifyResponse struct {
	// lookup mode
	Status string        `json:"status"`
	Data   *IdentifyData `json:"data"`

	// verify mode
	ID             string       `json:"id"`
	Found          bool         `json:"found"`
	Verified       bool         `json:"verified"`
	Score          float64      `json:"score"`
	Recommendation string       `json:"recommendation"`
	Fields         []MatchField `json:"fields"`
}

// Mismatches returns the subset of Fields that didn't match exactly —
// useful for surfacing "why" a verify-mode check failed.
func (r *IdentifyResponse) Mismatches() []MatchField {
	var out []MatchField
	for _, f := range r.Fields {
		if f.Match != "exact" {
			out = append(out, f)
		}
	}
	return out
}

// Identify calls POST /api/identity/identify.
func (c *Client) Identify(ctx context.Context, req IdentifyRequest) (*IdentifyResponse, error) {
	var out IdentifyResponse
	if err := c.do(ctx, "POST", "/api/identity/identify", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BulkIdentifyRequest maps to POST /api/identity/bulk-identify (up to 25 IDs).
type BulkIdentifyRequest struct {
	IDType    string `json:"idType"`
	IDNumbers string `json:"idNumbers"` // comma-separated
	Reference string `json:"reference,omitempty"`
}

type BulkIdentifyEntry struct {
	IdentifyData
	Error string `json:"error,omitempty"`
}

type BulkIdentifyResponse struct {
	Status string              `json:"status"`
	Data   []BulkIdentifyEntry `json:"data"`
}

// BulkIdentify calls POST /api/identity/bulk-identify.
func (c *Client) BulkIdentify(ctx context.Context, req BulkIdentifyRequest) (*BulkIdentifyResponse, error) {
	var out BulkIdentifyResponse
	if err := c.do(ctx, "POST", "/api/identity/bulk-identify", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
