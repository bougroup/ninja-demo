package handlers

import (
	"encoding/json"
	"net/http"
)

// jsonResult is the shape every widget-backed endpoint responds with — the
// widget itself already ran the Ninja call client-side, so the server's job
// here is just to apply local business rules and persist the outcome.
type jsonResult struct {
	OK       bool   `json:"ok"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message"`
	Redirect string `json:"redirect,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, jsonResult{OK: false, Message: message})
}
