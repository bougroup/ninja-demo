// Command server runs the Marketplace Vendor Payouts demo: a Go backend that
// drives every Ninja KYC/KYB sandbox endpoint end to end, backed by SQLite,
// with a HAM-built static landing page and server-rendered app pages.
package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/handlers"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

func main() {
	loadDotEnv(".env")

	clientKey := requireEnv("NINJA_CLIENT_KEY")
	clientSecret := requireEnv("NINJA_CLIENT_SECRET")
	apiBase := envOr("NINJA_API_BASE", "https://api.sandbox.ninja.boucloud.io")
	publicURL := envOr("PUBLIC_URL", "http://localhost:4000")
	dbPath := envOr("DATABASE_URL", "./data.db")
	addr := envOr("ADDR", ":4000")

	if os.Getenv("NINJA_WEBHOOK_SECRET") == "" {
		log.Println("warning: NINJA_WEBHOOK_SECRET is not set — every incoming webhook will be rejected as unsigned")
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	client := ninja.NewClient(apiBase, clientKey, clientSecret)
	client.SetLogger(func(endpoint, method string, statusCode, durationMs int, reqPayload, respPayload string, isMock bool) {
		_ = db.InsertAPILog(conn, uuid.NewString(), endpoint, method, statusCode, durationMs, reqPayload, respPayload, isMock)
	})

	env := &handlers.Env{DB: conn, Ninja: client, PublicURL: publicURL}

	handlers.LoadTemplates("templates/*.html")

	mux := http.NewServeMux()

	// Static landing page built by HAM (ham build -> web/public).
	mux.Handle("/", http.FileServer(http.Dir("web/public")))

	// Global system controls & Activity Stream
	mux.HandleFunc("POST /app/toggle-mock-mode", env.ToggleMockMode)
	mux.HandleFunc("POST /app/env/toggle-mock", env.ToggleMockMode)
	mux.HandleFunc("GET /app/api/activity-stream", env.ActivityStreamAPI)
	mux.HandleFunc("GET /api/activity-stream", env.ActivityStreamAPI)

	// Sandbox fixture reference — every "use this" chip pulls from here.
	mux.HandleFunc("GET /app/sandbox-data", env.SandboxDataPage)
	mux.HandleFunc("GET /app/guide", env.GuidePage)

	// Vendor-facing app (MarketPay Escrow).
	mux.HandleFunc("GET /app/vendor/apply", env.VendorApplyForm)
	mux.HandleFunc("POST /app/vendor/apply", env.VendorApplySubmit)
	mux.HandleFunc("GET /app/vendor/{id}", env.VendorStatus)
	mux.HandleFunc("POST /app/vendor/{id}/release-escrow", env.VendorReleaseEscrow)
	mux.HandleFunc("POST /app/vendor/{id}/simulate-webhook", env.VendorSimulateWebhook)
	mux.HandleFunc("GET /app/vendor/kyc-complete", env.VendorFlowComplete)
	mux.HandleFunc("GET /app/vendor/kyb-complete", env.VendorFlowComplete)

	// Compliance ops / admin.
	mux.HandleFunc("GET /app/admin", env.AdminDashboard)
	mux.HandleFunc("GET /app/admin/vendors/{id}", env.AdminVendorDetail)
	mux.HandleFunc("GET /app/admin/verifications/{id}/selfie", env.AdminVerificationSelfie)
	mux.HandleFunc("POST /app/admin/verifications/{id}/resend", env.AdminResendKYC)
	mux.HandleFunc("POST /app/admin/verifications/{id}/cancel", env.AdminCancelKYC)
	mux.HandleFunc("GET /app/admin/kyb-verifications/{id}/document/{doc}", env.AdminKYBDocument)
	mux.HandleFunc("POST /app/admin/kyb-verifications/{id}/resend", env.AdminResendKYB)
	mux.HandleFunc("POST /app/admin/kyb-verifications/{id}/cancel", env.AdminCancelKYB)
	mux.HandleFunc("GET /app/admin/webhooks", env.AdminWebhookDeliveries)
	mux.HandleFunc("POST /app/admin/webhooks/{id}/retry", env.AdminRetryWebhookDelivery)
	mux.HandleFunc("GET /app/admin/identity-check", env.AdminIdentityCheckForm)
	mux.HandleFunc("POST /app/admin/identity-check", env.AdminIdentityCheckSubmit)

	// Gaming & Betting (ApexBet).
	mux.HandleFunc("GET /app/gaming/signup", env.GamingSignupForm)
	mux.HandleFunc("POST /app/gaming/signup", env.GamingSignupSubmit)
	mux.HandleFunc("GET /app/gaming", env.GamingDashboard)
	mux.HandleFunc("POST /app/gaming/players/{id}/payout", env.GamingPayoutCheck)
	mux.HandleFunc("POST /app/gaming/players/{id}/bet", env.GamingBetSubmit)
	mux.HandleFunc("POST /app/gaming/players/{id}/self-exclude", env.GamingSelfExclude)

	// Fintechs (ApexPay Neobank).
	mux.HandleFunc("GET /app/fintech/onboard", env.FintechOnboardForm)
	mux.HandleFunc("POST /app/fintech/onboard", env.FintechOnboardSubmit)
	mux.HandleFunc("GET /app/fintech", env.FintechDashboard)
	mux.HandleFunc("POST /app/fintech/customers/{id}/transfer", env.FintechTransferSubmit)
	mux.HandleFunc("POST /app/fintech/customers/{id}/re-kyc", env.FintechReKYC)

	// International Companies (ApexGlobal FX Desk).
	mux.HandleFunc("GET /app/international/console", env.InternationalConsoleForm)
	mux.HandleFunc("POST /app/international/console", env.InternationalConsoleSubmit)

	// Agent Network KYC (ApexAgent POS).
	mux.HandleFunc("GET /app/agents/recruit", env.AgentRecruitForm)
	mux.HandleFunc("POST /app/agents/recruit", env.AgentRecruitSubmit)
	mux.HandleFunc("GET /app/agents", env.AgentsDashboard)
	mux.HandleFunc("POST /app/agents/simulate-fraud-ring", env.AgentSimulateFraudRing)
	mux.HandleFunc("POST /app/agents/{id}/reactivate", env.AgentReactivate)
	mux.HandleFunc("POST /app/agents/{id}/deactivate", env.AgentDeactivate)
	mux.HandleFunc("GET /app/agents/aggregators/apply", env.AggregatorApplyForm)
	mux.HandleFunc("POST /app/agents/aggregators/apply", env.AggregatorApplySubmit)
	mux.HandleFunc("GET /app/agents/aggregators/kyb-complete", env.AggregatorFlowComplete)
	mux.HandleFunc("GET /app/agents/aggregators/{id}", env.AggregatorStatus)

	// Employee Verification (WorkForce Payroll).
	mux.HandleFunc("GET /app/employees/apply", env.EmployeeApplyForm)
	mux.HandleFunc("POST /app/employees/apply", env.EmployeeApplySubmit)
	mux.HandleFunc("GET /app/employees/kyc-complete", env.EmployeeFlowComplete)
	mux.HandleFunc("GET /app/employees", env.EmployeesDashboard)
	mux.HandleFunc("GET /app/employees/{id}", env.EmployeeDetail)
	mux.HandleFunc("POST /app/employees/{id}/approve-payroll", env.EmployeeApprovePayroll)
	mux.HandleFunc("POST /app/employees/{id}/simulate-liveness", env.EmployeeSimulateLiveness)

	// Ninja sandbox calls this — HMAC-verified.
	mux.HandleFunc("POST /webhooks/ninja", env.WebhookReceiver)

	log.Printf("listening on %s (public url: %s)", addr, publicURL)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var %s (copy .env.example to .env and fill it in)", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv is a minimal .env loader (KEY=VALUE per line, # comments, no
// quoting/escaping) so the demo doesn't need an extra dependency for it.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // fine if absent — real env vars can be set another way
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("read %s: %v", path, err)
	}
}
