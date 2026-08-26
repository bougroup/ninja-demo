package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/bernardoko/ninja-demo/internal/db"
)

var tmpl *template.Template

// StepperData drives the shared "stepper" template partial: Steps are
// labels left-to-right, Current is the 0-indexed step the user is on
// (earlier steps render as done, later ones as upcoming).
type StepperData struct {
	Steps   []string
	Current int
}

// badgeClass buckets the many status/outcome/recommendation strings used
// across scenarios into one of three visual treatments for the shared
// "badge" template partial.
func badgeClass(status string) string {
	s := strings.ToLower(status)
	switch {
	case s == "":
		return "pending"
	case strings.HasPrefix(s, "blocked") || strings.HasPrefix(s, "flagged") ||
		strings.Contains(s, "reject") || strings.Contains(s, "fail") ||
		strings.Contains(s, "mismatch") || s == "rejected" || s == "dormant" || s == "deactivated" || s == "locked":
		return "fail"
	case strings.Contains(s, "pass") || strings.Contains(s, "accept") || s == "active" ||
		strings.Contains(s, "cleared") || strings.Contains(s, "eligible") || strings.Contains(s, "verified") ||
		s == "approved" || s == "onboarded" || s == "released":
		return "pass"
	default:
		return "pending"
	}
}

// formatKobo converts a kobo amount to a formatted Naira string (e.g. ₦350,000).
func formatKobo(kobo int64) string {
	naira := kobo / 100
	if naira < 0 {
		return fmt.Sprintf("-₦%s", formatWithCommas(-naira))
	}
	return fmt.Sprintf("₦%s", formatWithCommas(naira))
}

func formatWithCommas(n int64) string {
	in := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(in)+(len(in)-1)/3)
	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func formatScorePercent(score float64) string {
	return fmt.Sprintf("%.0f%%", score*100)
}

func jsonify(v any) template.JS {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(string(b))
}

func LoadTemplates(pattern string) {
	funcs := template.FuncMap{
		"badgeClass":         badgeClass,
		"hasPrefix":          strings.HasPrefix,
		"upper":              strings.ToUpper,
		"lower":              strings.ToLower,
		"add1":               func(i int) int { return i + 1 },
		"sub":                func(a, b int) int { return a - b },
		"mul":                func(a, b int) int64 { return int64(a) * int64(b) },
		"div":                func(a, b int64) int64 { if b == 0 { return 0 }; return a / b },
		"slice":              func(start, end int, s string) string {
			if start < 0 { start = 0 }
			if end > len(s) { end = len(s) }
			if start >= end { return "" }
			return s[start:end]
		},
		"ageFromDOB":         ageFromDOB,
		"formatKobo":         formatKobo,
		"formatScorePercent": formatScorePercent,
		"jsonify":            jsonify,
	}
	tmpl = template.Must(template.New("").Funcs(funcs).ParseGlob(pattern))
}

func (e *Env) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Path"] = r.URL.Path
	if _, ok := data["Flash"]; !ok {
		data["Flash"] = r.URL.Query().Get("flash")
	}
	data["IsMockMode"] = e.Ninja.IsMockMode()

	// Provide recent API logs for the global activity stream component
	if logs, err := db.ListRecentAPILogs(e.DB, 8); err == nil {
		data["RecentAPILogs"] = logs
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

// Global helper for handlers that call render
func render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Path"] = r.URL.Path
	if _, ok := data["Flash"]; !ok {
		data["Flash"] = r.URL.Query().Get("flash")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

func flash(w http.ResponseWriter, r *http.Request, redirectTo, message string) {
	http.Redirect(w, r, redirectTo+"?flash="+template.URLQueryEscaper(message), http.StatusSeeOther)
}
