package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type ResendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendWelcomeEmail sends a transactional welcome email via Resend if RESEND_API_KEY
// is configured; otherwise it gracefully logs a simulated delivery for demonstration.
func SendWelcomeEmail(toEmail, firstName string) (string, string, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "ApexBet <onboarding@resend.dev>"
	}

	subject := "🎰 Welcome to ApexBet — Complete 18+ Verification to Activate Betting"
	htmlContent := fmt.Sprintf(`
		<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 32px 24px; background: #0f172a; color: #f8fafc; border-radius: 12px; border: 1px solid #1e293b;">
			<div style="text-align: center; margin-bottom: 24px;">
				<h1 style="color: #ef4444; font-size: 28px; margin: 0; letter-spacing: -0.02em;">🎰 ApexBet Sportsbook</h1>
				<span style="display: inline-block; background: #1e293b; color: #94a3b8; font-size: 12px; font-weight: 700; padding: 4px 10px; border-radius: 999px; margin-top: 8px;">NLRC Licensed &middot; Powered by Ninja</span>
			</div>
			
			<h2 style="font-size: 20px; color: #ffffff; margin-bottom: 12px;">Welcome aboard, %s!</h2>
			<p style="font-size: 15px; line-height: 1.6; color: #cbd5e1;">Your ApexBet player account has been created. You have been credited with a <strong>₦100,000</strong> welcome betting credit balance.</p>
			
			<div style="background: #1e293b; border-left: 4px solid #38bdf8; padding: 16px 20px; border-radius: 6px; margin: 24px 0;">
				<strong style="color: #38bdf8; font-size: 14px; display: block; margin-bottom: 4px;">⚠️ Mandatory 18+ Age &amp; Identity Verification</strong>
				<p style="margin: 0; font-size: 13px; color: #94a3b8; line-height: 1.5;">Under Nigerian gambling regulation (NLRC Act §34), you must verify your National Identity (BVN or NIN) before your first wager or payout disbursement.</p>
			</div>

			<div style="text-align: center; margin: 30px 0;">
				<a href="http://localhost:4000/sportsbook" style="background: #10b981; color: #ffffff; text-decoration: none; font-weight: 700; font-size: 15px; padding: 14px 28px; border-radius: 8px; display: inline-block;">Complete 30-Second Verification →</a>
			</div>

			<p style="font-size: 12px; color: #64748b; text-align: center; border-top: 1px solid #1e293b; padding-top: 16px; margin-top: 24px;">
				ApexBet Sportsbook &middot; Identity &amp; Age Compliance powered by Ninja Sandbox API
			</p>
		</div>
	`, firstName)

	if apiKey == "" {
		log.Printf("[EMAIL SIMULATION] Sent welcome email to %s (No RESEND_API_KEY set).", toEmail)
		return "simulated", htmlContent, nil
	}

	payload := ResendEmailRequest{
		From:    fromEmail,
		To:      []string{toEmail},
		Subject: subject,
		HTML:    htmlContent,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "error", htmlContent, err
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return "error", htmlContent, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[RESEND ERROR] Failed to dispatch email to %s: %v", toEmail, err)
		return "failed", htmlContent, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[RESEND SUCCESS] Sent welcome email to %s (Status %d)", toEmail, resp.StatusCode)
		return "sent_resend", htmlContent, nil
	}

	log.Printf("[RESEND WARN] API responded with status %d for %s", resp.StatusCode, toEmail)
	return "resend_error", htmlContent, fmt.Errorf("resend api status %d", resp.StatusCode)
}
