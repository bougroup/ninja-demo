package handlers

import "net/http"

// GuidePage explains what each scenario does, which endpoints it exercises,
// and — critically — which ones work immediately on localhost vs. which
// need a public webhook URL first. GET /app/guide
func (e *Env) GuidePage(w http.ResponseWriter, r *http.Request) {
	render(w, r, "guide.html", nil)
}
