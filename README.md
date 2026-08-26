# Ninja Solutions Demo — One API, Six Real Jobs

Six live scenarios against the [Ninja](https://ninja.ng/) KYC/KYB sandbox
API, each demonstrating the specific practical use case from Ninja's own
solution pages — same handful of endpoints, six different real-world rules
applied on top of them.

Stack: Go backend (SQLite, `net/http`), server-rendered pages
(`html/template`), and a static landing gallery built with the
[HAM](https://github.com/bougroup/ham) framework.

**Every API call is server-side, against the sandbox, with no exceptions.**
Ninja also ships a client-side `ninja.js` widget SDK, but its bundle has
exactly one hardcoded API host — `api.ninja.boucloud.io` (production), no
sandbox awareness at all — so it's deliberately *not* embedded anywhere in
this demo (the International Companies page explains why and shows the
integration pattern as a static code sample instead). Two new reference
pages make this concrete: `/app/guide` walks through what each scenario
does and how to use it, and `/app/sandbox-data` lists every documented
sandbox fixture value — every form field in the app ships pre-filled with
one of them, so nothing needs to be typed to try it.

## The six scenarios

| Scenario | Solution page | Entry point | What it demonstrates |
|---|---|---|---|
| **Vendor Onboarding** | [ninja.ng/solutions/vendor-onboarding](https://ninja.ng/solutions/vendor-onboarding/) | `/app/vendor/apply` | Hosted KYB + advanced company lookup + director bulk-identify + AML; a payout wallet that only activates once the business and every director clear. |
| **Gaming & Betting** | [ninja.ng/for-gaming-betting](https://ninja.ng/for-gaming-betting/) | `/app/gaming/signup` | Age/identity gate at signup (`identify`, verify mode), a re-check before every payout, one identity → one account (unique constraint), self-exclusion enforcement. |
| **Fintechs** | [ninja.ng/for-fintechs](https://ninja.ng/for-fintechs/) | `/app/fintech/onboard` | Per-field name-matching score/recommendation (not blunt pass/fail) and a re-KYC workflow for flagged accounts. |
| **International Companies** | [ninja.ng/for-international-companies](https://ninja.ng/for-international-companies/) | `/app/international/console` | A bare verification console — single or bulk `identify` calls, raw typed JSON, full audit trail (timestamp, request id, operator). No workflow, just the API. |
| **Agent Network KYC** | [ninja.ng/solutions/agent-network-kyc](https://ninja.ng/solutions/agent-network-kyc/) | `/app/agents/recruit` | Real-time agent verification at recruitment, hosted KYB links for aggregator businesses, re-verification before reactivating a dormant agent, and a "one BVN behind fifty terminals" duplicate-identity flag. |
| **Employee Verification** | [ninja.ng/solutions/employee-verification](https://ninja.ng/solutions/employee-verification/) | `/app/employees/apply` | Instant dashboard lookup or a hosted selfie + liveness link for remote candidates, pre-employment. |

The landing page at `/` is a gallery linking into all six. Every scenario
shares the same `internal/ninja` client and the same webhook receiver — the
point is that it's one API surface, not six different integrations.

## Endpoint coverage

| Endpoint | Used by |
|---|---|
| `POST /auth/session` | `internal/ninja/client.go` — cached, auto-refreshed on every call |
| `POST /api/identity/identify` | Gaming (signup + payout re-check), Fintechs (onboard + re-KYC), International console, Agent Network (recruit + reactivate), Employees (dashboard lookup), Admin spot-check |
| `POST /api/identity/bulk-identify` | Vendor application (all directors in one call), International console (bulk mode) |
| `POST /api/company/lookup` | Vendor application, Agent Network aggregator registration |
| `POST /api/company/advanced-lookup` | Vendor application (directors + shareholders) |
| `POST /api/flows` | Created once on first use — separate hosted KYC flow configs for vendor directors and employee candidates |
| `POST /api/flows/:id/links` | Vendor directors, Employee candidates (hosted_link method) |
| `GET /api/verifications` | Admin dashboard — live list, filtered by the vendor-director KYC flow |
| `GET /api/verifications/:id` | Vendor admin detail, Employee candidate detail |
| `GET /api/verifications/:id/selfie` | Vendor admin, Employee candidate detail |
| `POST /api/verifications/:id/resend` | Vendor admin, Employee candidate detail (shared handler) |
| `POST /api/verifications/:id/cancel` | Vendor admin, Employee candidate detail (shared handler) |
| `POST /api/kyb-flows` | Created once on first use — separate hosted KYB flow configs for vendor businesses and agent-network aggregators |
| `POST /api/kyb-flows/:id/links` | Vendor application, Agent Network aggregator registration |
| `GET /api/kyb-verifications` | Admin dashboard — live list, filtered by the vendor KYB flow |
| `GET /api/kyb-verifications/:id` | Vendor admin detail |
| `GET /api/kyb-verifications/:id/document/:doc` | Vendor admin — cert / status report / memart / proof of address |
| `POST /api/kyb-verifications/:id/resend` | Vendor admin, Aggregator status page (shared handler) |
| `POST /api/kyb-verifications/:id/cancel` | Vendor admin, Aggregator status page (shared handler) |
| `GET /api/webhook-deliveries` | Admin → Webhook Deliveries page |
| `POST /api/webhook-deliveries/:id/retry` | Admin → "Retry" button |
| `verification.completed` webhook | `POST /webhooks/ninja` — dispatches to vendor director or employee candidate by verification id |
| `kyb_verification.completed` webhook | `POST /webhooks/ninja` — dispatches to vendor or aggregator by verification id |

## Running it locally, step by step

1. **Get sandbox credentials.** Sign up at [ninja.ng](https://ninja.ng/), grab
   your sandbox `client_key`/`client_secret` from the dashboard, and generate
   a webhook signing secret there too.

2. **Configure the app.**

   ```
   cp .env.example .env
   ```

   Fill in `NINJA_CLIENT_KEY`, `NINJA_CLIENT_SECRET`, and
   `NINJA_WEBHOOK_SECRET`. Leave `NINJA_API_BASE`, `PUBLIC_URL`,
   `DATABASE_URL`, and `ADDR` at their defaults for now.

3. **Install the `ham` CLI** (compiles the static landing page):

   ```
   go install github.com/fobilow/ham/cmd/ham@latest
   ```

   Make sure `$(go env GOPATH)/bin` is on your `PATH` so the `ham` command
   resolves.

4. **Build the landing page.**

   ```
   sh web/build.sh
   ```

   This runs `ham build` and copies the stylesheet into `web/public/` — `ham`
   alone doesn't copy CSS (that's normally a Rollup step this project
   deliberately skips, since there's no JS to bundle).

5. **Build and run the server.**

   ```
   go build -o ninja-demo-server ./cmd/server
   ./ninja-demo-server
   ```

   or skip the explicit build with `go run ./cmd/server`. You should see:

   ```
   listening on :4000 (public url: http://localhost:4000)
   ```

6. **Open `http://localhost:4000`.** The six-scenario gallery should load
   with styling. Every GET page works immediately — SQLite is created
   on first run (`data.db` in the project root), no migration step needed.

7. **Try a scenario that doesn't need a public webhook first** — International
   Companies (`/app/international/console`) or Fintechs
   (`/app/fintech/onboard`) call `identify`/`bulk-identify` directly and work
   over plain `localhost`. This confirms your credentials and network access
   are good before you deal with tunneling.

   Note: sandbox lookups aren't free (₦100–₦1,200 depending on the call) —
   if your sandbox account has no credit, calls will fail with a `400
   insufficient funds` error from Ninja itself. That's a billing issue on the
   Ninja account, not a bug in the app; fund the sandbox account to get past
   it.

8. **For the hosted-flow scenarios** (Vendor Onboarding, Agent Network
   aggregators, Employee Verification's hosted-link method), see
   [Exposing the webhook to the sandbox](#exposing-the-webhook-to-the-sandbox)
   below — Ninja rejects flow creation outright if `webhook_url` isn't a
   public address, so these need a tunnel before they'll work at all.

### Exposing the webhook to the sandbox

Ninja needs to reach `/webhooks/ninja` on the public internet **before it will
even create a hosted flow** — `webhook_url` must be a real public address, or
`POST /api/flows` / `POST /api/kyb-flows` reject the request outright with
`webhook url must be a public address`. Run a tunnel:

```
ngrok http 4000
```

> **Windows note:** if `ngrok` errors with `cannot execute binary: Exec
> format error`, the npm package downloaded the wrong-platform binary. A
> valid `ngrok.exe` is usually sitting right next to it —
> `%APPDATA%\npm\node_modules\ngrok\bin\ngrok.exe` — call that directly.
> Run `ngrok config add-authtoken <token>` once (free at ngrok.com) if it
> asks for one.

Set `PUBLIC_URL` in `.env` to the ngrok URL **before using any of the hosted
KYC/KYB scenarios** (Vendor Onboarding, Agent Network aggregators, Employee
Verification's hosted-link method) — each hosted flow config (which embeds
`webhook_url` and `redirect_url`) is created once, on first use, and its ID
is cached in the `config` table. If you change `PUBLIC_URL` later (e.g. a
new ngrok session — free ngrok URLs change every restart), delete `data.db`
or clear the `config` table so the flows get recreated with the new URL,
otherwise Ninja keeps calling back the old, dead tunnel.

`NINJA_WEBHOOK_SECRET` must match whatever secret you configure in the Ninja
dashboard for signing webhook deliveries (HMAC-SHA256 over the raw body,
sent in `X-Ninja-Signature`) — deliveries that fail verification are logged
but rejected with `401`.

Scenarios that only call `identify`/`bulk-identify`/`company-lookup`
directly (Gaming, Fintechs, International console, Agent recruitment,
Employee dashboard-lookup) don't need the tunnel at all — only the three
hosted-flow paths do (Vendor KYB+KYC, Agent aggregator KYB, Employee hosted
KYC).

## Running it with Docker

The image is a multi-stage build: `golang:1.26-alpine` compiles both the
static site (via `ham`) and a fully static Go binary (`CGO_ENABLED=0` —
`modernc.org/sqlite` is pure Go, no C toolchain needed), then everything
ships on a minimal `alpine:3.20` runtime. No Node/npm anywhere in the image.

1. **Configure.**

   ```
   cp .env.example .env
   # fill in NINJA_CLIENT_KEY, NINJA_CLIENT_SECRET, NINJA_WEBHOOK_SECRET
   ```

2. **Build and run with Compose** (recommended — also gives you a named
   volume so `data.db` survives container recreation):

   ```
   docker compose up --build
   ```

   Visit `http://localhost:4000`.

3. **Or build and run with plain Docker:**

   ```
   docker build -t ninja-demo .
   docker run --rm -p 4000:4000 --env-file .env \
     -e DATABASE_URL=/data/data.db \
     -v ninja-demo-data:/data \
     ninja-demo
   ```

   The image defaults `DATABASE_URL` to `/data/data.db` (the volume mount
   point), but if your `.env` already sets `DATABASE_URL=./data.db` for
   local (non-Docker) runs — as `.env.example` suggests — `--env-file` will
   pass that value straight through and **override** the image default,
   silently writing the database inside the container's writable layer
   instead of the volume (verified: it lands at `/app/data.db`, lost on
   `docker rm`). The explicit `-e DATABASE_URL=/data/data.db` above wins
   over `--env-file` and avoids this — keep it, don't rely on the image
   default alone if your `.env` sets `DATABASE_URL`. `docker-compose.yml`
   already does this correctly (its `environment:` block overrides
   `env_file:`).

4. **Tunneling still applies the same way** for the hosted-flow scenarios —
   point `ngrok`/`cloudflared` at whatever host port you mapped (`4000` in
   the examples above) and set `PUBLIC_URL` in `.env` to the tunnel URL
   before using those scenarios, same as running natively.

5. **Health check.** The image defines a `HEALTHCHECK` that hits `/` every
   30s — `docker ps` will show `healthy`/`unhealthy` once the container's
   been up a few seconds.

## Notes

- No auth on any route — this is a demo, not production.
- RC numbers, BVNs, and NINs should be sandbox test values from your Ninja
  sandbox account.
- The `payout_wallet_id` (Vendor Onboarding) is a locally-generated mock
  string — Ninja doesn't provide payment rails, only verification.
- The gaming, agent, and identity-console dedupe/flagging logic (one
  identity → one account, one BVN behind many terminals) is local business
  logic built on top of Ninja's verification data, not a Ninja feature
  itself.
- Response shapes for `GET /api/verifications`, `GET /api/kyb-verifications`,
  and `GET /api/webhook-deliveries` were corrected against the real sandbox
  (they return bare JSON arrays with `id` fields, not `{"data": [...]}`
  wrappers with `verification_id` — the solution-page docs implied
  otherwise). The webhook *payload* shape is still unverified against a real
  delivery — `ninja.ExtractVerificationID` in `internal/ninja/webhook_payloads.go`
  defensively tries both `id` and `verification_id` keys.
