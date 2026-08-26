package ninja

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleMockRequest returns simulated high-fidelity responses when MockMode
// is active or when offline fallback is requested.
func (c *Client) handleMockRequest(ctx context.Context, method, path string, reqBody, out any) (bool, error) {
	time.Sleep(50 * time.Millisecond)

	switch {
	case path == "/auth/session":
		if outPtr, ok := out.(*sessionResponse); ok {
			*outPtr = sessionResponse{
				Token:  "sandbox_mock_token_" + uuid.NewString()[:8],
				Expiry: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			}
		}
		return true, nil

	case path == "/api/identity/identify":
		req, ok := reqBody.(IdentifyRequest)
		if !ok {
			return false, nil
		}
		resp := mockIdentify(req)
		if resp == nil {
			return true, &APIError{StatusCode: 500, Body: `{"error":"simulated upstream provider 500"}`}
		}
		if outPtr, ok := out.(*IdentifyResponse); ok {
			*outPtr = *resp
		}
		return true, nil

	case path == "/api/identity/bulk-identify":
		req, ok := reqBody.(BulkIdentifyRequest)
		if !ok {
			return false, nil
		}
		resp := mockBulkIdentify(req)
		if outPtr, ok := out.(*BulkIdentifyResponse); ok {
			*outPtr = *resp
		}
		return true, nil

	case path == "/api/company/lookup":
		req, ok := reqBody.(CompanyLookupRequest)
		if !ok {
			return false, nil
		}
		resp, err := mockCompanyLookup(req)
		if err != nil {
			return true, err
		}
		if outPtr, ok := out.(*CompanyLookupResponse); ok {
			*outPtr = *resp
		}
		return true, nil

	case path == "/api/company/advanced-lookup":
		req, ok := reqBody.(CompanyAdvancedLookupRequest)
		if !ok {
			return false, nil
		}
		resp, err := mockCompanyAdvancedLookup(req)
		if err != nil {
			return true, err
		}
		if outPtr, ok := out.(*CompanyAdvancedLookupResponse); ok {
			*outPtr = *resp
		}
		return true, nil

	case path == "/api/flows":
		req, _ := reqBody.(CreateFlowRequest)
		flowID := "flow_mock_kyc_" + uuid.NewString()[:8]
		if outPtr, ok := out.(*FlowResponse); ok {
			*outPtr = FlowResponse{
				ID:   flowID,
				Name: req.Name,
			}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/flows/") && strings.HasSuffix(path, "/links"):
		verID := "ver_mock_kyc_" + uuid.NewString()[:8]
		if outPtr, ok := out.(*FlowLinkResponse); ok {
			*outPtr = FlowLinkResponse{
				ID:  verID,
				URL: "/app/vendor/kyc-complete?simulated=1&ver_id=" + verID,
			}
		}
		return true, nil

	case path == "/api/kyb-flows":
		req, _ := reqBody.(CreateKYBFlowRequest)
		flowID := "flow_mock_kyb_" + uuid.NewString()[:8]
		if outPtr, ok := out.(*KYBFlowResponse); ok {
			*outPtr = KYBFlowResponse{
				ID:   flowID,
				Name: req.Name,
			}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/kyb-flows/") && strings.HasSuffix(path, "/links"):
		verID := "ver_mock_kyb_" + uuid.NewString()[:8]
		if outPtr, ok := out.(*KYBFlowLinkResponse); ok {
			*outPtr = KYBFlowLinkResponse{
				ID:  verID,
				URL: "/app/vendor/kyb-complete?simulated=1&ver_id=" + verID,
			}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/verifications/") && strings.HasSuffix(path, "/resend"):
		if outPtr, ok := out.(*map[string]any); ok {
			*outPtr = map[string]any{"ok": true, "status": "resent"}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/verifications/") && strings.HasSuffix(path, "/cancel"):
		if outPtr, ok := out.(*map[string]any); ok {
			*outPtr = map[string]any{"ok": true, "status": "canceled"}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/kyb-verifications/") && strings.HasSuffix(path, "/resend"):
		if outPtr, ok := out.(*map[string]any); ok {
			*outPtr = map[string]any{"ok": true, "status": "resent"}
		}
		return true, nil

	case strings.HasPrefix(path, "/api/kyb-verifications/") && strings.HasSuffix(path, "/cancel"):
		if outPtr, ok := out.(*map[string]any); ok {
			*outPtr = map[string]any{"ok": true, "status": "canceled"}
		}
		return true, nil
	}

	return false, nil
}

func mockIdentify(req IdentifyRequest) *IdentifyResponse {
	id := req.IDNumber
	if id == "00000000000" {
		return nil
	}
	if id == "55555555555" {
		return &IdentifyResponse{
			ID:             uuid.NewString(),
			Found:          false,
			Verified:       false,
			Score:          0.0,
			Recommendation: "reject",
			Fields: []MatchField{
				{Field: "id_number", Score: 0.0, Match: "none", Provided: id, Detail: "Record not found in national registry"},
			},
		}
	}
	if id == "66666666666" {
		return &IdentifyResponse{
			ID:             uuid.NewString(),
			Found:          true,
			Verified:       true,
			Score:          0.85,
			Recommendation: "review",
			Fields: []MatchField{
				{Field: "first_name", Score: 1.0, Match: "exact", Provided: req.FirstName, Detail: "Alex"},
				{Field: "last_name", Score: 1.0, Match: "exact", Provided: req.LastName, Detail: "Watchlist"},
				{Field: "aml_sanction", Score: 0.5, Match: "partial", Provided: "PEP / Sanction Hit", Detail: "Matches domestic regulatory review flag"},
			},
			Data: &IdentifyData{
				IDNumber: id, Type: req.IDType, FirstName: "Alex", LastName: "Watchlist", DateOfBirth: "1975-01-01",
				Gender: "male", AddressState: "Lagos", Mobile: "08012345678",
			},
		}
	}
	if id == "88888888888" || id == "16161616161" {
		return &IdentifyResponse{
			ID:             uuid.NewString(),
			Found:          true,
			Verified:       true,
			Score:          1.0,
			Recommendation: "accept",
			Fields: []MatchField{
				{Field: "first_name", Score: 1.0, Match: "exact", Provided: req.FirstName, Detail: "Tobi"},
				{Field: "last_name", Score: 1.0, Match: "exact", Provided: req.LastName, Detail: "Minor"},
				{Field: "date_of_birth", Score: 1.0, Match: "exact", Provided: "2010-06-15", Detail: "2010-06-15"},
			},
			Data: &IdentifyData{
				IDNumber: id, Type: req.IDType, FirstName: "Tobi", LastName: "Minor", DateOfBirth: "2010-06-15",
				Gender: "male", AddressState: "Lagos", Mobile: "08088888888",
			},
		}
	}

	retFirst := "James"
	retLast := "Bond"
	retDOB := "1975-01-01"

	if req.IDNumber == "77777777772" {
		retFirst = "Chukwuemeka"
		retLast = "Emeka"
		retDOB = "1980-05-20"
	}

	sim1 := stringSimilarity(req.FirstName, retFirst)
	sim2 := stringSimilarity(req.LastName, retLast)
	score := (sim1 + sim2) / 2.0
	if req.DateOfBirth != "" && req.DateOfBirth != retDOB {
		score = score * 0.7
	}

	recommendation := "accept"
	matchType := "exact"
	if score < 0.60 {
		recommendation = "reject"
		matchType = "none"
	} else if score < 0.90 {
		recommendation = "review"
		matchType = "partial"
	}

	return &IdentifyResponse{
		ID:             uuid.NewString(),
		Found:          true,
		Verified:       score >= 0.60,
		Score:          score,
		Recommendation: recommendation,
		Fields: []MatchField{
			{Field: "first_name", Score: sim1, Match: matchType, Provided: req.FirstName, Detail: retFirst},
			{Field: "last_name", Score: sim2, Match: matchType, Provided: req.LastName, Detail: retLast},
			{Field: "date_of_birth", Score: 1.0, Match: "exact", Provided: req.DateOfBirth, Detail: retDOB},
		},
		Data: &IdentifyData{
			IDNumber: id, Type: req.IDType, FirstName: retFirst, LastName: retLast, DateOfBirth: retDOB,
			Gender: "male", AddressState: "Lagos", Mobile: "08077777777",
		},
	}
}

func stringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == "" || s2 == "" {
		return 0.0
	}
	if s1 == s2 {
		return 1.0
	}
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 0.85
	}
	d := levenshtein(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	if maxLen == 0 {
		return 1.0
	}
	sim := 1.0 - (float64(d) / float64(maxLen))
	if sim < 0 {
		return 0
	}
	return sim
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, min(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[la][lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mockBulkIdentify(req BulkIdentifyRequest) *BulkIdentifyResponse {
	rawIDs := strings.Split(req.IDNumbers, ",")
	var entries []BulkIdentifyEntry
	for _, idStr := range rawIDs {
		id := strings.TrimSpace(idStr)
		if id == "" {
			continue
		}
		if id == "55555555555" {
			entries = append(entries, BulkIdentifyEntry{
				Error: "ID number not found in registry",
			})
			continue
		}
		if id == "66666666666" {
			entries = append(entries, BulkIdentifyEntry{
				IdentifyData: IdentifyData{
					IDNumber: id, Type: req.IDType, FirstName: "Alex", LastName: "Watchlist", DateOfBirth: "1975-01-01",
					Gender: "male", AddressState: "Lagos", Mobile: "08012345678",
				},
			})
			continue
		}
		entries = append(entries, BulkIdentifyEntry{
			IdentifyData: IdentifyData{
				IDNumber: id, Type: req.IDType, FirstName: "James", LastName: "Bond", DateOfBirth: "1975-01-01",
				Gender: "male", AddressState: "Lagos", Mobile: "08077777777",
			},
		})
	}
	return &BulkIdentifyResponse{
		Status: "success",
		Data:   entries,
	}
}

func mockCompanyLookup(req CompanyLookupRequest) (*CompanyLookupResponse, error) {
	rc := strings.TrimSpace(req.RCNumber)
	if rc == "5555555" {
		return nil, &APIError{StatusCode: 404, Body: `{"error":"Company with RC number not found"}`}
	}
	if rc == "1111111" {
		return nil, &APIError{StatusCode: 500, Body: `{"error":"Upstream CAC registry timeout"}`}
	}
	name := "NINJA DEMO COMPANY LIMITED"
	if rc == "6666666" {
		name = "NINJA WATCHLIST HOLDINGS LIMITED"
	}
	return &CompanyLookupResponse{
		Status: "success",
		Data: CompanyData{
			RegistrationNumber: rc,
			Name:               name,
			Status:             "Active",
			CompanyType:        "Private Company Limited by Shares",
			NatureOfBusiness:   "Financial Technology & Verification Services",
			Email:              "compliance@ninjademo.com",
			Address:            "12 Marina Street, Lagos Island, Lagos State",
			Directors: []Director{
				{ID: "DIR-001", Name: "James Bond", IDType: "bvn", IDNumber: "77777777777"},
				{ID: "DIR-002", Name: "Amaka Okonkwo", IDType: "nin", IDNumber: "77777777777"},
				{ID: "DIR-003", Name: "Oluwaseun Adeleke", IDType: "bvn", IDNumber: "77777777777"},
			},
		},
	}, nil
}

func mockCompanyAdvancedLookup(req CompanyAdvancedLookupRequest) (*CompanyAdvancedLookupResponse, error) {
	base, err := mockCompanyLookup(CompanyLookupRequest{RCNumber: req.RCNumber, Reference: req.Reference})
	if err != nil {
		return nil, err
	}
	return &CompanyAdvancedLookupResponse{
		Status: "success",
		Data: CompanyAdvancedData{
			CompanyData: base.Data,
			Secretary: []Director{
				{ID: "SEC-001", Name: "Adeola Legal Practitioners", IDType: "cac", IDNumber: "BN-44821"},
			},
			Shareholders: []Shareholder{
				{ID: "SH-001", Name: "James Bond", SharesAllotted: "6,000,000 (60%)"},
				{ID: "SH-002", Name: "Amaka Okonkwo", SharesAllotted: "4,000,000 (40%)"},
			},
		},
	}, nil
}
