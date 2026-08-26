package ninja

import (
	"context"
	"testing"
)

func TestMockIdentify(t *testing.T) {
	client := NewClient("https://api.sandbox.ninja.boucloud.io", "test-key", "test-secret")
	client.SetMockMode(true)
	ctx := context.Background()

	// 1. James Bond (Exact match)
	res, err := client.Identify(ctx, IdentifyRequest{
		IDType:      "bvn",
		Mode:        "verify",
		IDNumber:    "77777777777",
		FirstName:   "James",
		LastName:    "Bond",
		DateOfBirth: "1975-01-01",
		Reference:   "ref-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Found || !res.Verified {
		t.Errorf("expected James Bond to be verified")
	}
	if res.Score < 0.8 && res.Score < 80 {
		t.Errorf("expected high score for exact match, got %v", res.Score)
	}

	// 2. Tobi Minor (Underage)
	resMinor, err := client.Identify(ctx, IdentifyRequest{
		IDType:      "bvn",
		Mode:        "verify",
		IDNumber:    "88888888888",
		FirstName:   "Tobi",
		LastName:    "Minor",
		DateOfBirth: "2010-06-15",
		Reference:   "ref-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resMinor.Data.DateOfBirth != "2010-06-15" {
		t.Errorf("expected DOB 2010-06-15, got %s", resMinor.Data.DateOfBirth)
	}

	// 3. Unregistered ID (404)
	resNotFound, err := client.Identify(ctx, IdentifyRequest{
		IDType:   "bvn",
		Mode:     "lookup",
		IDNumber: "55555555555",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resNotFound.Found {
		t.Errorf("expected 55555555555 not to be found")
	}
}

func TestMockBulkIdentify(t *testing.T) {
	client := NewClient("https://api.sandbox.ninja.boucloud.io", "test-key", "test-secret")
	client.SetMockMode(true)
	ctx := context.Background()

	res, err := client.BulkIdentify(ctx, BulkIdentifyRequest{
		IDType:    "bvn",
		IDNumbers: "77777777777,66666666666,55555555555,00000000000",
		Reference: "batch-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Data) != 4 {
		t.Fatalf("expected 4 entries in batch response, got %d", len(res.Data))
	}
	if res.Data[0].IDNumber != "77777777777" || res.Data[0].Error != "" {
		t.Errorf("expected 77777777777 to be found without error")
	}
	if res.Data[2].Error == "" {
		t.Errorf("expected 55555555555 to have error")
	}
}

func TestMockCompanyLookup(t *testing.T) {
	client := NewClient("https://api.sandbox.ninja.boucloud.io", "test-key", "test-secret")
	client.SetMockMode(true)
	ctx := context.Background()

	// 1. Found Company (RC 0000000)
	comp, err := client.CompanyLookup(ctx, CompanyLookupRequest{RCNumber: "0000000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.Data.RegistrationNumber != "0000000" || comp.Data.Status != "Active" {
		t.Errorf("expected Active company 0000000, got %+v", comp.Data)
	}

	// 2. Advanced Lookup with Directors
	adv, err := client.CompanyAdvancedLookup(ctx, CompanyAdvancedLookupRequest{RCNumber: "0000000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adv.Data.Directors) != 3 {
		t.Errorf("expected 3 directors for RC 0000000, got %d", len(adv.Data.Directors))
	}
}
