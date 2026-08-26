package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bernardoko/ninja-demo/internal/db"
	"github.com/bernardoko/ninja-demo/internal/ninja"
	"github.com/google/uuid"
)

// AdminIdentityCheckForm renders the ad-hoc spot-check tool compliance ops
// use outside the vendor pipeline — e.g. re-checking a director's BVN by
// hand when a dispute comes in. GET /app/admin/identity-check
func (e *Env) AdminIdentityCheckForm(w http.ResponseWriter, r *http.Request) {
	checks, err := db.ListIdentityChecksByScenario(e.DB, "admin_spot_check", 20)
	if err != nil {
		checks = nil
	}
	render(w, r, "admin_identity_check.html", map[string]any{
		"Checks": checks,
		"Flash":  r.URL.Query().Get("flash"),
	})
}

// AdminIdentityCheckSubmit calls POST /api/identity/identify directly (mode
// lookup or verify) and stores the result. POST /app/admin/identity-check
func (e *Env) AdminIdentityCheckSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	req := ninja.IdentifyRequest{
		IDType:      r.FormValue("id_type"),
		Mode:        r.FormValue("mode"),
		IDNumber:    r.FormValue("id_number"),
		FirstName:   r.FormValue("first_name"),
		LastName:    r.FormValue("last_name"),
		DateOfBirth: r.FormValue("date_of_birth"),
		Reference:   uuid.NewString(),
	}

	result, err := e.Ninja.Identify(r.Context(), req)
	if err != nil {
		flash(w, r, "/app/admin/identity-check", "identify failed: "+err.Error())
		return
	}

	raw, _ := json.Marshal(result)
	_ = db.InsertIdentityCheck(e.DB, uuid.NewString(), "admin_spot_check", "demo-operator", "", req.Mode, req.IDType, req.IDNumber, req.Reference, string(raw))

	flash(w, r, "/app/admin/identity-check", "check complete: see history below")
}
