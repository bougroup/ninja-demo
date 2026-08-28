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

	// Static assets from web/public/assets
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("web/public/assets"))))

	// Core ApexBet Sportsbook & Registration Routes
	mux.HandleFunc("GET /{$}", env.SportsbookDashboard)
	mux.HandleFunc("GET /sportsbook", env.SportsbookDashboard)
	mux.HandleFunc("GET /register", env.RegisterForm)
	mux.HandleFunc("POST /register", env.RegisterSubmit)
	mux.HandleFunc("GET /signup", env.RegisterForm)
	mux.HandleFunc("POST /signup", env.RegisterSubmit)

	// Live Betting & Payout Compliance Actions
	mux.HandleFunc("POST /api/kyc/verify", env.VerifyIdentitySubmit)
	mux.HandleFunc("POST /api/bets/place", env.PlaceBetSubmit)
	mux.HandleFunc("POST /api/bets/simulate-win", env.SimulateMatchWin)
	mux.HandleFunc("POST /api/payout/request", env.RequestPayoutCheck)
	mux.HandleFunc("POST /api/responsible-gaming/self-exclude", env.UserSelfExclude)
	mux.HandleFunc("POST /api/demo/reset", env.ResetDemoState)

	// Global system controls & Activity Stream
	mux.HandleFunc("POST /app/toggle-mock-mode", env.ToggleMockMode)
	mux.HandleFunc("POST /app/env/toggle-mock", env.ToggleMockMode)
	mux.HandleFunc("GET /app/api/activity-stream", env.ActivityStreamAPI)
	mux.HandleFunc("GET /api/activity-stream", env.ActivityStreamAPI)

	// Sandbox fixture reference
	mux.HandleFunc("GET /app/sandbox-data", env.SandboxDataPage)
	mux.HandleFunc("GET /app/guide", env.GuidePage)

	// Compliance ops / admin audit tools
	mux.HandleFunc("GET /app/admin", env.AdminDashboard)
	mux.HandleFunc("GET /app/admin/webhooks", env.AdminWebhookDeliveries)
	mux.HandleFunc("POST /app/admin/webhooks/{id}/retry", env.AdminRetryWebhookDelivery)
	mux.HandleFunc("GET /app/admin/identity-check", env.AdminIdentityCheckForm)
	mux.HandleFunc("POST /app/admin/identity-check", env.AdminIdentityCheckSubmit)

	// Webhook endpoint from Ninja (HMAC-verified)
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
