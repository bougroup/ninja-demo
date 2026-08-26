package handlers

import "net/http"

// SandboxDataPage renders the reference table of Ninja's documented sandbox
// fixture values — every scenario's "use this" chips pull from the same
// list, so this page also serves as the single source of truth for it.
// GET /app/sandbox-data
func (e *Env) SandboxDataPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, "sandbox_data.html", nil)
}
