-- Marketplace Vendor Payouts & Fintech Business Showcase schema.
-- One row per Ninja concept round-tripped through the sandbox.

CREATE TABLE IF NOT EXISTS config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS api_logs (
    id               TEXT PRIMARY KEY,
    endpoint         TEXT NOT NULL,
    method           TEXT NOT NULL,
    status_code      INTEGER NOT NULL,
    duration_ms      INTEGER NOT NULL,
    request_payload  TEXT,
    response_payload TEXT,
    is_mock          INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS vendors (
    id                  TEXT PRIMARY KEY,
    business_name       TEXT NOT NULL,
    rc_number           TEXT NOT NULL,
    contact_name        TEXT NOT NULL,
    contact_email       TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'applied',
        -- applied -> company_verified -> kyb_pending -> kyb_review -> payout_eligible | rejected
    payout_wallet_id    TEXT,
    escrow_balance_kobo INTEGER NOT NULL DEFAULT 1250000000, -- ₦12,500,000 escrow vault
    escrow_status       TEXT NOT NULL DEFAULT 'locked',     -- locked -> released

    -- POST /api/company/lookup snapshot
    company_registration_number TEXT,
    company_status               TEXT,
    company_type                 TEXT,
    company_nature_of_business   TEXT,
    company_email                TEXT,
    company_address              TEXT,
    company_lookup_raw            TEXT, -- full JSON

    -- POST /api/company/advanced-lookup snapshot (adds secretary + shareholders)
    company_advanced_raw          TEXT, -- full JSON (directors, secretary, shareholders)

    -- Hosted KYB flow
    kyb_flow_id         TEXT,
    kyb_verification_id TEXT,
    kyb_verification_url TEXT,
    kyb_outcome         TEXT,
    kyb_raw             TEXT,

    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS directors (
    id                   TEXT PRIMARY KEY,
    vendor_id            TEXT NOT NULL REFERENCES vendors(id),
    cac_director_id      TEXT,        -- id as it appears in the CAC director list
    full_name            TEXT,
    id_type              TEXT,        -- nin / bvn
    id_number            TEXT,

    -- POST /api/identity/bulk-identify result for this director
    bulk_identify_status TEXT,
    bulk_identify_raw    TEXT,

    -- Hosted KYC flow (per-director selfie/liveness check)
    kyc_flow_id          TEXT,
    kyc_verification_id  TEXT,
    kyc_verification_url TEXT,
    kyc_outcome          TEXT,
    kyc_score            REAL,
    kyc_raw              TEXT,

    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Shared audit log for direct POST /api/identity/identify + bulk-identify
-- calls that don't belong to a hosted flow.
CREATE TABLE IF NOT EXISTS identity_checks (
    id          TEXT PRIMARY KEY,
    scenario    TEXT NOT NULL DEFAULT 'admin_spot_check',
    operator    TEXT NOT NULL DEFAULT 'demo-operator',
    batch_id    TEXT,           -- groups rows created by one bulk-identify call
    mode        TEXT NOT NULL, -- lookup | verify
    id_type     TEXT NOT NULL,
    id_number   TEXT NOT NULL,
    reference   TEXT,
    result_raw  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS webhook_events (
    id           TEXT PRIMARY KEY,
    event        TEXT NOT NULL,
    delivery_id  TEXT,          -- Ninja's webhook-delivery id, if present in payload/header
    payload_raw  TEXT NOT NULL,
    signature_ok INTEGER NOT NULL,
    processed    INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    received_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS users (
    id                TEXT PRIMARY KEY,
    first_name        TEXT NOT NULL,
    last_name         TEXT NOT NULL,
    email             TEXT UNIQUE NOT NULL,
    password_hash     TEXT NOT NULL,
    mobile            TEXT,
    email_verified    INTEGER NOT NULL DEFAULT 1,
    kyc_status        TEXT NOT NULL DEFAULT 'unverified',
        -- unverified -> verified | blocked_underage | blocked_mismatch | blocked_duplicate | blocked_self_excluded
    id_type           TEXT, -- bvn | nin
    id_number         TEXT,
    date_of_birth     TEXT,
    age               INTEGER NOT NULL DEFAULT 0,
    kyc_score         REAL NOT NULL DEFAULT 0,
    identify_raw      TEXT,
    liveness_status   TEXT NOT NULL DEFAULT 'none',
    balance_kobo      INTEGER NOT NULL DEFAULT 10000000, -- ₦100,000 promo welcome balance
    winnings_kobo     INTEGER NOT NULL DEFAULT 0,
    self_excluded     INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS email_logs (
    id          TEXT PRIMARY KEY,
    to_email    TEXT NOT NULL,
    subject     TEXT NOT NULL,
    body_html   TEXT NOT NULL,
    status      TEXT NOT NULL, -- sent_resend | simulated | failed
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Gaming & Betting: signup age/identity gate + duplicate-account prevention + live wallet.
CREATE TABLE IF NOT EXISTS gaming_players (
    id                TEXT PRIMARY KEY,
    full_name         TEXT NOT NULL,
    date_of_birth     TEXT NOT NULL,
    id_type           TEXT NOT NULL, -- bvn | nin
    id_number         TEXT NOT NULL,
    identify_raw      TEXT,
    self_excluded     INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'pending',
        -- pending -> active | blocked_underage | blocked_mismatch | blocked_duplicate | blocked_self_excluded
    balance_kobo      INTEGER NOT NULL DEFAULT 35000000, -- ₦350,000
    winnings_kobo     INTEGER NOT NULL DEFAULT 18000000, -- ₦180,000
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (id_type, id_number)
);

CREATE TABLE IF NOT EXISTS gaming_payouts (
    id           TEXT PRIMARY KEY,
    player_id    TEXT NOT NULL,
    amount_kobo  INTEGER NOT NULL,
    identify_raw TEXT,
    status       TEXT NOT NULL DEFAULT 'pending', -- pending -> approved | blocked_mismatch | blocked_underage | blocked_self_excluded
    reason       TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS gaming_bets (
    id           TEXT PRIMARY KEY,
    player_id    TEXT NOT NULL,
    match_event  TEXT NOT NULL,
    selection    TEXT NOT NULL,
    odds         REAL NOT NULL,
    stake_kobo   INTEGER NOT NULL,
    status       TEXT NOT NULL DEFAULT 'placed', -- placed | won | lost | flagged_aml
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Fintechs: name-matching score/recommendation + Tiered KYC + Wire transfers.
CREATE TABLE IF NOT EXISTS fintech_customers (
    id               TEXT PRIMARY KEY,
    full_name        TEXT NOT NULL,
    date_of_birth    TEXT NOT NULL,
    id_type          TEXT NOT NULL,
    id_number        TEXT NOT NULL,
    score            REAL,
    recommendation   TEXT, -- accept | review | reject
    mismatches_raw   TEXT, -- json array
    fields_raw       TEXT, -- json array
    tier             INTEGER NOT NULL DEFAULT 1,
    daily_limit_kobo INTEGER NOT NULL DEFAULT 5000000,  -- ₦50,000 limit for Tier 1
    balance_kobo     INTEGER NOT NULL DEFAULT 125000000, -- ₦1,250,000
    status           TEXT NOT NULL DEFAULT 'onboarded',
        -- onboarded -> flagged_review -> re_kyc_cleared | re_kyc_failed
    last_checked_at  TEXT NOT NULL DEFAULT (datetime('now')),
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS fintech_transfers (
    id              TEXT PRIMARY KEY,
    customer_id     TEXT NOT NULL REFERENCES fintech_customers(id),
    recipient_name  TEXT NOT NULL,
    recipient_bank  TEXT NOT NULL,
    recipient_acct  TEXT NOT NULL,
    amount_kobo     INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'completed', -- completed | blocked_tier_limit | held_re_kyc
    reason          TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Agent Network KYC: field recruitment of agents + hosted KYB for aggregators + POS fleet.
CREATE TABLE IF NOT EXISTS aggregators (
    id                   TEXT PRIMARY KEY,
    business_name        TEXT NOT NULL,
    rc_number            TEXT NOT NULL,
    contact_name         TEXT NOT NULL,
    contact_email        TEXT NOT NULL,
    company_lookup_raw   TEXT,
    kyb_flow_id          TEXT,
    kyb_verification_id  TEXT,
    kyb_verification_url TEXT,
    kyb_outcome          TEXT,
    kyb_raw              TEXT,
    status               TEXT NOT NULL DEFAULT 'applied',
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS agents (
    id                 TEXT PRIMARY KEY,
    full_name          TEXT NOT NULL,
    date_of_birth      TEXT,
    id_type            TEXT NOT NULL,
    id_number          TEXT NOT NULL,
    terminal_id        TEXT NOT NULL,
    aggregator_id      TEXT REFERENCES aggregators(id),
    identify_raw       TEXT,
    status             TEXT NOT NULL DEFAULT 'pending',
        -- pending -> active | flagged_duplicate_bvn | dormant | deactivated
    pos_terminal_count INTEGER NOT NULL DEFAULT 1,
    cash_float_kobo    INTEGER NOT NULL DEFAULT 200000000, -- ₦2,000,000 float
    last_reverified_at TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Employee Verification: pre-employment checks + payroll unlock.
CREATE TABLE IF NOT EXISTS candidates (
    id                   TEXT PRIMARY KEY,
    full_name            TEXT NOT NULL,
    role                 TEXT,
    id_type              TEXT NOT NULL,
    id_number            TEXT,
    check_method         TEXT NOT NULL, -- dashboard_lookup | hosted_link
    identify_raw         TEXT,
    kyc_flow_id          TEXT,
    kyc_verification_id  TEXT,
    kyc_verification_url TEXT,
    kyc_outcome          TEXT,
    kyc_score            REAL,
    kyc_raw              TEXT,
    monthly_salary_kobo  INTEGER NOT NULL DEFAULT 85000000, -- ₦850,000 / mo
    payroll_status       TEXT NOT NULL DEFAULT 'locked',    -- locked -> active
    status               TEXT NOT NULL DEFAULT 'pending',
        -- pending -> cleared | mismatch | failed
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
