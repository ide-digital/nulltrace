-- NullTrace vault schema v1
-- PII columns are stored as XChaCha20-Poly1305 blobs (nonce || ciphertext || tag).
-- The DEK never leaves process memory; only a KEK-wrapped copy lives in vault_meta.

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS vault_meta (
    k TEXT PRIMARY KEY,
    v BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS identities (
    id              TEXT PRIMARY KEY,
    first_name_enc  BLOB NOT NULL,
    last_name_enc   BLOB NOT NULL,
    middle_name_enc BLOB,
    dob_enc         BLOB,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS identity_attributes (
    id                   TEXT PRIMARY KEY,
    identity_id          TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    attribute_type       TEXT NOT NULL,
    attribute_value_enc  BLOB NOT NULL,
    is_primary           INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attr_identity ON identity_attributes(identity_id);
CREATE INDEX IF NOT EXISTS idx_attr_type ON identity_attributes(attribute_type);

CREATE TABLE IF NOT EXISTS brokers (
    id                          TEXT PRIMARY KEY,
    name                        TEXT NOT NULL,
    domain                      TEXT NOT NULL UNIQUE,
    category                    TEXT NOT NULL,
    mechanism                   TEXT NOT NULL,
    opt_out_url                 TEXT,
    contact_email               TEXT,
    requires_captcha            INTEGER NOT NULL DEFAULT 0,
    requires_email_confirmation INTEGER NOT NULL DEFAULT 0,
    jurisdiction_coverage       TEXT NOT NULL,
    repopulation_period_days    INTEGER NOT NULL DEFAULT 60,
    notes                       TEXT,
    playbook                    TEXT
);

CREATE INDEX IF NOT EXISTS idx_brokers_mechanism ON brokers(mechanism);
CREATE INDEX IF NOT EXISTS idx_brokers_category ON brokers(category);

CREATE TABLE IF NOT EXISTS exposed_records (
    id                     TEXT PRIMARY KEY,
    identity_id            TEXT NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    broker_id              TEXT NOT NULL REFERENCES brokers(id) ON DELETE CASCADE,
    profile_url            TEXT,
    extracted_data_json_enc BLOB NOT NULL,
    confidence_score       REAL NOT NULL,
    status                 TEXT NOT NULL,
    risk_tier              TEXT NOT NULL DEFAULT 'MEDIUM',
    first_detected_at      TEXT NOT NULL,
    last_verified_at       TEXT NOT NULL,
    UNIQUE(identity_id, broker_id)
);

CREATE INDEX IF NOT EXISTS idx_exposed_identity ON exposed_records(identity_id);
CREATE INDEX IF NOT EXISTS idx_exposed_status ON exposed_records(status);
CREATE INDEX IF NOT EXISTS idx_exposed_broker ON exposed_records(broker_id);

CREATE TABLE IF NOT EXISTS erasure_actions (
    id                   TEXT PRIMARY KEY,
    exposed_record_id    TEXT NOT NULL REFERENCES exposed_records(id) ON DELETE CASCADE,
    broker_id            TEXT NOT NULL REFERENCES brokers(id) ON DELETE CASCADE,
    strategy             TEXT NOT NULL,
    current_state        TEXT NOT NULL,
    attempts             INTEGER NOT NULL DEFAULT 0,
    tracking_token       TEXT,
    request_payload_enc  BLOB,
    response_log_enc     BLOB,
    statutory_deadline   TEXT,
    completed_at         TEXT,
    last_error           TEXT,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_actions_state ON erasure_actions(current_state);
CREATE INDEX IF NOT EXISTS idx_actions_record ON erasure_actions(exposed_record_id);
CREATE INDEX IF NOT EXISTS idx_actions_deadline ON erasure_actions(statutory_deadline);

CREATE TABLE IF NOT EXISTS secrets (
    k          TEXT PRIMARY KEY,
    v_enc      BLOB NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_ledger (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    event_uuid   TEXT NOT NULL UNIQUE,
    timestamp    TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    broker_id    TEXT REFERENCES brokers(id),
    payload_hash TEXT NOT NULL,
    details_enc  BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_ledger(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_type ON audit_ledger(event_type);

CREATE TRIGGER IF NOT EXISTS audit_ledger_no_update
BEFORE UPDATE ON audit_ledger
BEGIN
    SELECT RAISE(ABORT, 'audit_ledger is append-only');
END;

CREATE TRIGGER IF NOT EXISTS audit_ledger_no_delete
BEFORE DELETE ON audit_ledger
BEGIN
    SELECT RAISE(ABORT, 'audit_ledger is append-only');
END;
