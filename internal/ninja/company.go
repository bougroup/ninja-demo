package ninja

import "context"

type Director struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IDType   string `json:"id_type,omitempty"`
	IDNumber string `json:"id_number,omitempty"`
}

type Shareholder struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SharesAllotted string `json:"shares_allotted,omitempty"`
}

// CompanyLookupRequest maps to POST /api/company/lookup (₦550/lookup).
type CompanyLookupRequest struct {
	RCNumber  string `json:"rc_number"`
	Reference string `json:"reference,omitempty"`
}

type CompanyData struct {
	RegistrationNumber string     `json:"registration_number"`
	Name               string     `json:"name"`
	Status             string     `json:"status"`
	CompanyType        string     `json:"company_type"`
	NatureOfBusiness   string     `json:"nature_of_business"`
	Email              string     `json:"email"`
	Address            string     `json:"address"`
	Directors          []Director `json:"directors"`
}

type CompanyLookupResponse struct {
	Status string      `json:"status"`
	Data   CompanyData `json:"data"`
}

// CompanyLookup calls POST /api/company/lookup.
func (c *Client) CompanyLookup(ctx context.Context, req CompanyLookupRequest) (*CompanyLookupResponse, error) {
	var out CompanyLookupResponse
	if err := c.do(ctx, "POST", "/api/company/lookup", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CompanyAdvancedLookupRequest maps to POST /api/company/advanced-lookup (₦1,200/lookup).
type CompanyAdvancedLookupRequest struct {
	RCNumber  string `json:"rc_number"`
	Reference string `json:"reference,omitempty"`
}

type CompanyAdvancedData struct {
	CompanyData
	Secretary    []Director    `json:"secretary"`
	Shareholders []Shareholder `json:"shareholders"`
}

type CompanyAdvancedLookupResponse struct {
	Status string              `json:"status"`
	Data   CompanyAdvancedData `json:"data"`
}

// CompanyAdvancedLookup calls POST /api/company/advanced-lookup.
func (c *Client) CompanyAdvancedLookup(ctx context.Context, req CompanyAdvancedLookupRequest) (*CompanyAdvancedLookupResponse, error) {
	var out CompanyAdvancedLookupResponse
	if err := c.do(ctx, "POST", "/api/company/advanced-lookup", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
