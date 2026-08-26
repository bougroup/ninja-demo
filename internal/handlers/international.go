package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// InternationalConsoleForm renders the bare verification console — the
// "call it from anywhere" pitch for teams with no Nigerian entity: raw
// single or bulk identity/identify calls with a visible audit trail
// (timestamp, request id, operator). GET /app/international/console
func (e *Env) InternationalConsoleForm(w http.ResponseWriter, r *http.Request) {
	e.renderInternationalConsole(w, r, nil, nil, 0, "")
}

// InternationalConsoleSubmit calls POST /api/identity/identify (single) or
// POST /api/identity/bulk-identify (bulk) directly from the console and
// renders the raw response inline. POST /app/international/console
func (e *Env) InternationalConsoleSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	checkType := r.FormValue("check_type")
	idType := r.FormValue("id_type")
	operator := "demo-operator"

	start := time.Now()
	if checkType == "bulk" {
		idNumbers := r.FormValue("id_numbers")
		batchID := uuid.NewString()

		result, err := e.Ninja.BulkIdentify(ctx, ninja.BulkIdentifyRequest{
			IDType:    idType,
			IDNumbers: idNumbers,
			Reference: batchID,
		})
		duration := time.Since(start).Milliseconds()
		if err != nil {
			e.renderInternationalConsole(w, r, nil, nil, 0, "bulk-identify failed: "+err.Error())
			return
		}
		for _, entry := range result.Data {
			raw, _ := json.Marshal(entry)
			num := entry.IDNumber
			if num == "" {
				num = "(unmatched)"
			}
			_ = db.InsertIdentityCheck(e.DB, uuid.NewString(), "international_console", operator, batchID, "bulk", idType, num, batchID, string(raw))
		}
		e.renderInternationalConsole(w, r, result, nil, duration, "")
		return
	}

	mode := r.FormValue("mode")
	idNumber := r.FormValue("id_number")
	reference := uuid.NewString()
	result, err := e.Ninja.Identify(ctx, ninja.IdentifyRequest{
		IDType:      idType,
		Mode:        mode,
		IDNumber:    idNumber,
		FirstName:   r.FormValue("first_name"),
		LastName:    r.FormValue("last_name"),
		DateOfBirth: r.FormValue("date_of_birth"),
		Reference:   reference,
	})
	duration := time.Since(start).Milliseconds()
	if err != nil {
		e.renderInternationalConsole(w, r, nil, nil, 0, "identify failed: "+err.Error())
		return
	}
	raw, _ := json.Marshal(result)
	_ = db.InsertIdentityCheck(e.DB, uuid.NewString(), "international_console", operator, "", mode, idType, idNumber, reference, string(raw))
	e.renderInternationalConsole(w, r, nil, result, duration, "")
}

func (e *Env) renderInternationalConsole(w http.ResponseWriter, r *http.Request, bulkResult *ninja.BulkIdentifyResponse, singleResult *ninja.IdentifyResponse, durationMs int64, errMsg string) {
	checks, err := db.ListIdentityChecksByScenario(e.DB, "international_console", 25)
	if err != nil {
		log.Printf("list identity checks: %v", err)
	}

	var resultJSON string
	if bulkResult != nil {
		b, _ := json.MarshalIndent(bulkResult, "", "  ")
		resultJSON = string(b)
	} else if singleResult != nil {
		b, _ := json.MarshalIndent(singleResult, "", "  ")
		resultJSON = string(b)
	}

	e.render(w, r, "international_console.html", map[string]any{
		"BulkResult":   bulkResult,
		"SingleResult": singleResult,
		"Result":       resultJSON,
		"DurationMs":   durationMs,
		"Error":        errMsg,
		"Checks":       checks,
		"Flash":        r.URL.Query().Get("flash"),
		"IDTypes":      []string{"nin", "bvn", "ndl"},
	})
}
