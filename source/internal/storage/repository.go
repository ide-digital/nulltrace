package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	ncrypto "github.com/4x3/nulltrace/internal/crypto"
	"github.com/4x3/nulltrace/internal/nterr"
)

const (
	metaSalt       = "kdf.salt"
	metaWrappedDEK = "kdf.wrapped_dek"
	metaVerifier   = "kdf.verifier"
	metaTime       = "kdf.time"
	metaMemory     = "kdf.memory"
	metaThreads    = "kdf.threads"
	metaKeyLen     = "kdf.key_len"
)

// Store is the persistence façade. The DEK is held in memory only while the
// vault is unlocked. Lock wipes it.
type Store struct {
	db  *sql.DB
	dek []byte
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error {
	s.Lock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) Locked() bool { return s.dek == nil }

func (s *Store) Lock() {
	ncrypto.Zeroize(s.dek)
	s.dek = nil
}

func (s *Store) DEK() []byte { return s.dek }

func (s *Store) Initialized(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vault_meta WHERE k = ?`, metaWrappedDEK).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("vault initialized check: %w", err)
	}
	return n > 0, nil
}

func (s *Store) InitVault(ctx context.Context, passphrase []byte) error {
	ok, err := s.Initialized(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nterr.ErrVaultExists
	}
	env, dek, err := ncrypto.InitEnvelope(passphrase)
	if err != nil {
		return err
	}
	defer ncrypto.Zeroize(dek)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin vault init: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	puts := []struct {
		k string
		v []byte
	}{
		{metaSalt, env.Salt},
		{metaWrappedDEK, env.WrappedDEK},
		{metaVerifier, env.Verifier},
		{metaTime, []byte(fmt.Sprintf("%d", env.Params.Time))},
		{metaMemory, []byte(fmt.Sprintf("%d", env.Params.Memory))},
		{metaThreads, []byte(fmt.Sprintf("%d", env.Params.Threads))},
		{metaKeyLen, []byte(fmt.Sprintf("%d", env.Params.KeyLen))},
	}
	for _, p := range puts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vault_meta(k, v) VALUES (?, ?)`, p.k, p.v); err != nil {
			return fmt.Errorf("store vault meta %s: %w", p.k, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vault init: %w", err)
	}
	s.dek = append([]byte(nil), dek...)
	return nil
}

func (s *Store) Unlock(ctx context.Context, passphrase []byte) error {
	env, err := s.loadEnvelope(ctx)
	if err != nil {
		return err
	}
	dek, err := ncrypto.OpenEnvelope(passphrase, env)
	if err != nil {
		return err
	}
	s.Lock()
	s.dek = dek
	return nil
}

func (s *Store) loadEnvelope(ctx context.Context) (*ncrypto.Envelope, error) {
	get := func(k string) ([]byte, error) {
		var v []byte
		err := s.db.QueryRowContext(ctx, `SELECT v FROM vault_meta WHERE k = ?`, k).Scan(&v)
		if err == sql.ErrNoRows {
			return nil, nterr.ErrNotInitialized
		}
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	salt, err := get(metaSalt)
	if err != nil {
		return nil, err
	}
	wrapped, err := get(metaWrappedDEK)
	if err != nil {
		return nil, err
	}
	verifier, err := get(metaVerifier)
	if err != nil {
		return nil, err
	}
	timeB, err := get(metaTime)
	if err != nil {
		return nil, err
	}
	memB, err := get(metaMemory)
	if err != nil {
		return nil, err
	}
	thrB, err := get(metaThreads)
	if err != nil {
		return nil, err
	}
	klB, err := get(metaKeyLen)
	if err != nil {
		return nil, err
	}
	var timeU, memU, keyU uint32
	var thrU uint8
	if _, err := fmt.Sscanf(string(timeB), "%d", &timeU); err != nil {
		return nil, fmt.Errorf("parse kdf time: %w", err)
	}
	if _, err := fmt.Sscanf(string(memB), "%d", &memU); err != nil {
		return nil, fmt.Errorf("parse kdf memory: %w", err)
	}
	var thrInt int
	if _, err := fmt.Sscanf(string(thrB), "%d", &thrInt); err != nil {
		return nil, fmt.Errorf("parse kdf threads: %w", err)
	}
	thrU = uint8(thrInt)
	if _, err := fmt.Sscanf(string(klB), "%d", &keyU); err != nil {
		return nil, fmt.Errorf("parse kdf keylen: %w", err)
	}
	return &ncrypto.Envelope{
		Salt:       salt,
		WrappedDEK: wrapped,
		Verifier:   verifier,
		Params: ncrypto.KDFParams{
			Time:    timeU,
			Memory:  memU,
			Threads: thrU,
			KeyLen:  keyU,
		},
	}, nil
}

func (s *Store) requireDEK() error {
	if s.dek == nil {
		return nterr.ErrVaultLocked
	}
	return nil
}

func (s *Store) encrypt(table, column, rowID string, plain []byte) ([]byte, error) {
	if err := s.requireDEK(); err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		return nil, nil
	}
	return ncrypto.Encrypt(s.dek, plain, ncrypto.AAD(table, column, rowID))
}

func (s *Store) decrypt(table, column, rowID string, blob []byte) ([]byte, error) {
	if err := s.requireDEK(); err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, nil
	}
	return ncrypto.Decrypt(s.dek, blob, ncrypto.AAD(table, column, rowID))
}

func (s *Store) PutSecret(ctx context.Context, key string, value []byte) error {
	if err := s.requireDEK(); err != nil {
		return err
	}
	enc, err := s.encrypt("secrets", "v_enc", key, value)
	if err != nil {
		return err
	}
	now := nowRFC3339()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO secrets(k, v_enc, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(k) DO UPDATE SET v_enc = excluded.v_enc, updated_at = excluded.updated_at
	`, key, enc, now)
	return err
}

func (s *Store) DeleteSecret(ctx context.Context, key string) error {
	if err := s.requireDEK(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE k = ?`, key)
	return err
}

func (s *Store) GetSecret(ctx context.Context, key string) ([]byte, error) {
	if err := s.requireDEK(); err != nil {
		return nil, err
	}
	var enc []byte
	err := s.db.QueryRowContext(ctx, `SELECT v_enc FROM secrets WHERE k = ?`, key).Scan(&enc)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.decrypt("secrets", "v_enc", key, enc)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

func parseTimePtr(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTime(s.String)
	return &t
}

func (s *Store) UpsertBroker(ctx context.Context, b BrokerRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO brokers (
			id, name, domain, category, mechanism, opt_out_url, contact_email,
			requires_captcha, requires_email_confirmation, jurisdiction_coverage,
			repopulation_period_days, notes, playbook
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			domain = excluded.domain,
			category = excluded.category,
			mechanism = excluded.mechanism,
			opt_out_url = excluded.opt_out_url,
			contact_email = excluded.contact_email,
			requires_captcha = excluded.requires_captcha,
			requires_email_confirmation = excluded.requires_email_confirmation,
			jurisdiction_coverage = excluded.jurisdiction_coverage,
			repopulation_period_days = excluded.repopulation_period_days,
			notes = excluded.notes,
			playbook = excluded.playbook
	`, b.ID, b.Name, b.Domain, b.Category, b.Mechanism, nullStr(b.OptOutURL), nullStr(b.ContactEmail),
		boolInt(b.RequiresCaptcha), boolInt(b.RequiresEmailConfirmation), b.JurisdictionCoverage,
		b.RepopulationPeriodDays, nullStr(b.Notes), nullStr(b.Playbook))
	if err != nil {
		return fmt.Errorf("upsert broker %s: %w", b.ID, err)
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) ListBrokers(ctx context.Context) ([]BrokerRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, domain, category, mechanism,
			COALESCE(opt_out_url, ''), COALESCE(contact_email, ''),
			requires_captcha, requires_email_confirmation, jurisdiction_coverage,
			repopulation_period_days, COALESCE(notes, ''), COALESCE(playbook, '')
		FROM brokers ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list brokers: %w", err)
	}
	defer rows.Close()
	var out []BrokerRow
	for rows.Next() {
		var b BrokerRow
		var cap, conf int
		if err := rows.Scan(&b.ID, &b.Name, &b.Domain, &b.Category, &b.Mechanism,
			&b.OptOutURL, &b.ContactEmail, &cap, &conf, &b.JurisdictionCoverage,
			&b.RepopulationPeriodDays, &b.Notes, &b.Playbook); err != nil {
			return nil, err
		}
		b.RequiresCaptcha = cap != 0
		b.RequiresEmailConfirmation = conf != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) GetBroker(ctx context.Context, id string) (BrokerRow, error) {
	var b BrokerRow
	var cap, conf int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, domain, category, mechanism,
			COALESCE(opt_out_url, ''), COALESCE(contact_email, ''),
			requires_captcha, requires_email_confirmation, jurisdiction_coverage,
			repopulation_period_days, COALESCE(notes, ''), COALESCE(playbook, '')
		FROM brokers WHERE id = ?`, id).Scan(
		&b.ID, &b.Name, &b.Domain, &b.Category, &b.Mechanism,
		&b.OptOutURL, &b.ContactEmail, &cap, &conf, &b.JurisdictionCoverage,
		&b.RepopulationPeriodDays, &b.Notes, &b.Playbook)
	if err == sql.ErrNoRows {
		return BrokerRow{}, nterr.ErrBrokerNotFound
	}
	if err != nil {
		return BrokerRow{}, err
	}
	b.RequiresCaptcha = cap != 0
	b.RequiresEmailConfirmation = conf != 0
	return b, nil
}

func (s *Store) InsertIdentity(ctx context.Context, first, last, middle, dob []byte) (string, error) {
	if err := s.requireDEK(); err != nil {
		return "", err
	}
	id := uuid.NewString()
	fn, err := s.encrypt("identities", "first_name_enc", id, first)
	if err != nil {
		return "", err
	}
	ln, err := s.encrypt("identities", "last_name_enc", id, last)
	if err != nil {
		return "", err
	}
	var mn, db []byte
	if len(middle) > 0 {
		mn, err = s.encrypt("identities", "middle_name_enc", id, middle)
		if err != nil {
			return "", err
		}
	}
	if len(dob) > 0 {
		db, err = s.encrypt("identities", "dob_enc", id, dob)
		if err != nil {
			return "", err
		}
	}
	now := nowRFC3339()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO identities(id, first_name_enc, last_name_enc, middle_name_enc, dob_enc, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, fn, ln, mn, db, now, now)
	if err != nil {
		return "", fmt.Errorf("insert identity: %w", err)
	}
	return id, nil
}

func (s *Store) ListIdentities(ctx context.Context) ([]IdentityRow, error) {
	if err := s.requireDEK(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, first_name_enc, last_name_enc, middle_name_enc, dob_enc, created_at, updated_at
		FROM identities ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentityRow
	for rows.Next() {
		var r IdentityRow
		var created, updated string
		var middle, dob []byte
		if err := rows.Scan(&r.ID, &r.FirstNameEnc, &r.LastNameEnc, &middle, &dob, &created, &updated); err != nil {
			return nil, err
		}
		r.MiddleNameEnc = middle
		r.DOBEnc = dob
		r.CreatedAt = parseTime(created)
		r.UpdatedAt = parseTime(updated)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DecryptIdentity(row IdentityRow) (first, last, middle, dob []byte, err error) {
	first, err = s.decrypt("identities", "first_name_enc", row.ID, row.FirstNameEnc)
	if err != nil {
		return
	}
	last, err = s.decrypt("identities", "last_name_enc", row.ID, row.LastNameEnc)
	if err != nil {
		ncrypto.Zeroize(first)
		return
	}
	if len(row.MiddleNameEnc) > 0 {
		middle, err = s.decrypt("identities", "middle_name_enc", row.ID, row.MiddleNameEnc)
		if err != nil {
			ncrypto.Zeroize(first)
			ncrypto.Zeroize(last)
			return
		}
	}
	if len(row.DOBEnc) > 0 {
		dob, err = s.decrypt("identities", "dob_enc", row.ID, row.DOBEnc)
		if err != nil {
			ncrypto.Zeroize(first)
			ncrypto.Zeroize(last)
			ncrypto.Zeroize(middle)
			return
		}
	}
	return
}

func (s *Store) AddAttribute(ctx context.Context, identityID, attrType string, value []byte, primary bool) (string, error) {
	if err := s.requireDEK(); err != nil {
		return "", err
	}
	id := uuid.NewString()
	enc, err := s.encrypt("identity_attributes", "attribute_value_enc", id, value)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO identity_attributes(id, identity_id, attribute_type, attribute_value_enc, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, identityID, strings.ToUpper(attrType), enc, boolInt(primary), nowRFC3339())
	if err != nil {
		return "", fmt.Errorf("insert attribute: %w", err)
	}
	return id, nil
}

func (s *Store) ListAttributes(ctx context.Context, identityID string) ([]AttributeRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, identity_id, attribute_type, attribute_value_enc, is_primary, created_at
		FROM identity_attributes WHERE identity_id = ? ORDER BY created_at`, identityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AttributeRow
	for rows.Next() {
		var r AttributeRow
		var primary int
		var created string
		if err := rows.Scan(&r.ID, &r.IdentityID, &r.Type, &r.ValueEnc, &primary, &created); err != nil {
			return nil, err
		}
		r.IsPrimary = primary != 0
		r.CreatedAt = parseTime(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DecryptAttribute(row AttributeRow) ([]byte, error) {
	return s.decrypt("identity_attributes", "attribute_value_enc", row.ID, row.ValueEnc)
}

func (s *Store) LookupExposedID(ctx context.Context, identityID, brokerID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM exposed_records WHERE identity_id = ? AND broker_id = ?`,
		identityID, brokerID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// UpsertExposed inserts or updates an exposure row. The ciphertext AAD is bound
// to rec.ID, so this method reuses the existing primary key when the
// (identity_id, broker_id) pair already exists.
func (s *Store) UpsertExposed(ctx context.Context, rec ExposedRecordRow) (string, error) {
	existing, err := s.LookupExposedID(ctx, rec.IdentityID, rec.BrokerID)
	if err != nil {
		return "", err
	}
	if existing != "" {
		rec.ID = existing
	} else if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	if rec.FirstDetectedAt.IsZero() {
		rec.FirstDetectedAt = time.Now().UTC()
	}
	if rec.LastVerifiedAt.IsZero() {
		rec.LastVerifiedAt = time.Now().UTC()
	}
	first := rec.FirstDetectedAt.UTC().Format(time.RFC3339Nano)
	last := rec.LastVerifiedAt.UTC().Format(time.RFC3339Nano)
	if existing != "" {
		_, err = s.db.ExecContext(ctx, `
			UPDATE exposed_records SET
				profile_url = ?,
				extracted_data_json_enc = ?,
				confidence_score = ?,
				status = ?,
				risk_tier = ?,
				last_verified_at = ?
			WHERE id = ?`,
			nullStr(rec.ProfileURL), rec.ExtractedEnc, rec.Confidence, rec.Status, rec.RiskTier, last, rec.ID)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO exposed_records (
				id, identity_id, broker_id, profile_url, extracted_data_json_enc,
				confidence_score, status, risk_tier, first_detected_at, last_verified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, rec.IdentityID, rec.BrokerID, nullStr(rec.ProfileURL), rec.ExtractedEnc,
			rec.Confidence, rec.Status, rec.RiskTier, first, last)
	}
	if err != nil {
		return "", fmt.Errorf("upsert exposed record: %w", err)
	}
	return rec.ID, nil
}

// SaveFinding encrypts details under the stable exposed-record id, then upserts.
func (s *Store) SaveFinding(ctx context.Context, rec ExposedRecordRow, detailsJSON []byte) (string, error) {
	existing, err := s.LookupExposedID(ctx, rec.IdentityID, rec.BrokerID)
	if err != nil {
		return "", err
	}
	if existing != "" {
		rec.ID = existing
	} else if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	enc, err := s.EncryptJSON(rec.ID, detailsJSON)
	if err != nil {
		return "", err
	}
	rec.ExtractedEnc = enc
	return s.UpsertExposed(ctx, rec)
}

func (s *Store) ListExposed(ctx context.Context) ([]ExposedRecordRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, identity_id, broker_id, COALESCE(profile_url, ''), extracted_data_json_enc,
			confidence_score, status, risk_tier, first_detected_at, last_verified_at
		FROM exposed_records ORDER BY confidence_score DESC, first_detected_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExposedRecordRow
	for rows.Next() {
		var r ExposedRecordRow
		var first, last string
		if err := rows.Scan(&r.ID, &r.IdentityID, &r.BrokerID, &r.ProfileURL, &r.ExtractedEnc,
			&r.Confidence, &r.Status, &r.RiskTier, &first, &last); err != nil {
			return nil, err
		}
		r.FirstDetectedAt = parseTime(first)
		r.LastVerifiedAt = parseTime(last)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetExposed(ctx context.Context, id string) (ExposedRecordRow, error) {
	var r ExposedRecordRow
	var first, last string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, identity_id, broker_id, COALESCE(profile_url, ''), extracted_data_json_enc,
			confidence_score, status, risk_tier, first_detected_at, last_verified_at
		FROM exposed_records WHERE id = ?`, id).Scan(
		&r.ID, &r.IdentityID, &r.BrokerID, &r.ProfileURL, &r.ExtractedEnc,
		&r.Confidence, &r.Status, &r.RiskTier, &first, &last)
	if err == sql.ErrNoRows {
		return ExposedRecordRow{}, nterr.ErrRecordNotFound
	}
	if err != nil {
		return ExposedRecordRow{}, err
	}
	r.FirstDetectedAt = parseTime(first)
	r.LastVerifiedAt = parseTime(last)
	return r, nil
}

func (s *Store) SetExposedStatus(ctx context.Context, id, status string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE exposed_records SET status = ?, last_verified_at = ? WHERE id = ?`,
		status, nowRFC3339(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nterr.ErrRecordNotFound
	}
	return nil
}

func (s *Store) EncryptJSON(rowID string, json []byte) ([]byte, error) {
	return s.encrypt("exposed_records", "extracted_data_json_enc", rowID, json)
}

func (s *Store) DecryptJSON(row ExposedRecordRow) ([]byte, error) {
	return s.decrypt("exposed_records", "extracted_data_json_enc", row.ID, row.ExtractedEnc)
}

func (s *Store) InsertAction(ctx context.Context, a ActionRow) (string, error) {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.TrackingToken == "" {
		a.TrackingToken = uuid.NewString()
	}
	now := nowRFC3339()
	var deadline any
	if a.StatutoryDeadline != nil {
		deadline = a.StatutoryDeadline.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO erasure_actions (
			id, exposed_record_id, broker_id, strategy, current_state, attempts,
			tracking_token, request_payload_enc, response_log_enc, statutory_deadline,
			completed_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ExposedRecordID, a.BrokerID, a.Strategy, a.CurrentState, a.Attempts,
		a.TrackingToken, a.RequestPayloadEnc, a.ResponseLogEnc, deadline,
		nil, nullStr(a.LastError), now, now)
	if err != nil {
		return "", fmt.Errorf("insert action: %w", err)
	}
	return a.ID, nil
}

func (s *Store) UpdateActionState(ctx context.Context, id, state string, attemptDelta int, lastErr string, response []byte) error {
	now := nowRFC3339()
	var completed any
	if state == "COMPLETED" || state == "REJECTED" {
		completed = now
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE erasure_actions
		SET current_state = ?,
		    attempts = attempts + ?,
		    last_error = ?,
		    response_log_enc = COALESCE(?, response_log_enc),
		    completed_at = COALESCE(?, completed_at),
		    updated_at = ?
		WHERE id = ?`, state, attemptDelta, nullStr(lastErr), response, completed, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nterr.ErrActionNotFound
	}
	return nil
}

func (s *Store) ListActions(ctx context.Context) ([]ActionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, exposed_record_id, broker_id, strategy, current_state, attempts,
			COALESCE(tracking_token, ''), request_payload_enc, response_log_enc,
			statutory_deadline, completed_at, COALESCE(last_error, ''), created_at, updated_at
		FROM erasure_actions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionRow
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ActionsByState(ctx context.Context, state string) ([]ActionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, exposed_record_id, broker_id, strategy, current_state, attempts,
			COALESCE(tracking_token, ''), request_payload_enc, response_log_enc,
			statutory_deadline, completed_at, COALESCE(last_error, ''), created_at, updated_at
		FROM erasure_actions WHERE current_state = ? ORDER BY created_at`, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionRow
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAction(rows rowScanner) (ActionRow, error) {
	var a ActionRow
	var deadline, completed sql.NullString
	var created, updated string
	var req, resp []byte
	err := rows.Scan(&a.ID, &a.ExposedRecordID, &a.BrokerID, &a.Strategy, &a.CurrentState, &a.Attempts,
		&a.TrackingToken, &req, &resp, &deadline, &completed, &a.LastError, &created, &updated)
	if err != nil {
		return ActionRow{}, err
	}
	a.RequestPayloadEnc = req
	a.ResponseLogEnc = resp
	a.StatutoryDeadline = parseTimePtr(deadline)
	a.CompletedAt = parseTimePtr(completed)
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	return a, nil
}

func (s *Store) GetActionByTracking(ctx context.Context, token string) (ActionRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, exposed_record_id, broker_id, strategy, current_state, attempts,
			COALESCE(tracking_token, ''), request_payload_enc, response_log_enc,
			statutory_deadline, completed_at, COALESCE(last_error, ''), created_at, updated_at
		FROM erasure_actions WHERE tracking_token = ?`, token)
	a, err := scanAction(row)
	if err == sql.ErrNoRows {
		return ActionRow{}, nterr.ErrActionNotFound
	}
	return a, err
}

func (s *Store) CompletedPastRepopulation(ctx context.Context, now time.Time) ([]ActionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.exposed_record_id, a.broker_id, a.strategy, a.current_state, a.attempts,
			COALESCE(a.tracking_token, ''), a.request_payload_enc, a.response_log_enc,
			a.statutory_deadline, a.completed_at, COALESCE(a.last_error, ''), a.created_at, a.updated_at
		FROM erasure_actions a
		JOIN brokers b ON b.id = a.broker_id
		JOIN exposed_records r ON r.id = a.exposed_record_id
		WHERE a.current_state = 'COMPLETED'
		  AND r.status = 'VERIFIED_REMOVED'
		  AND a.completed_at IS NOT NULL
		  AND julianday(?) - julianday(a.completed_at) >= b.repopulation_period_days
	`, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActionRow
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) AppendAudit(ctx context.Context, eventType, brokerID string, details []byte) error {
	if err := s.requireDEK(); err != nil {
		return err
	}
	id := uuid.NewString()
	enc, err := s.encrypt("audit_ledger", "details_enc", id, details)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(details)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO audit_ledger(event_uuid, timestamp, event_type, broker_id, payload_hash, details_enc)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, nowRFC3339(), eventType, nullStr(brokerID), hex.EncodeToString(sum[:]), enc)
	if err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_uuid, timestamp, event_type, COALESCE(broker_id, ''), payload_hash, details_enc
		FROM audit_ledger ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		var ts string
		if err := rows.Scan(&r.ID, &r.EventUUID, &ts, &r.EventType, &r.BrokerID, &r.PayloadHash, &r.DetailsEnc); err != nil {
			return nil, err
		}
		r.Timestamp = parseTime(ts)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DecryptAudit(row AuditRow) ([]byte, error) {
	return s.decrypt("audit_ledger", "details_enc", row.EventUUID, row.DetailsEnc)
}

func (s *Store) Stats(ctx context.Context) (DashboardStats, error) {
	var st DashboardStats
	queries := []struct {
		dst *int
		q   string
	}{
		{&st.BrokerCount, `SELECT COUNT(*) FROM brokers`},
		{&st.ActiveRemovals, `SELECT COUNT(*) FROM erasure_actions WHERE current_state IN ('QUEUED','SUBMITTED','AWAITING_CONFIRMATION','AWAITING_MANUAL')`},
		{&st.VerifiedRemoved, `SELECT COUNT(*) FROM exposed_records WHERE status = 'VERIFIED_REMOVED'`},
		{&st.ExposedRecords, `SELECT COUNT(*) FROM exposed_records`},
		{&st.FailedActions, `SELECT COUNT(*) FROM erasure_actions WHERE current_state = 'FAILED'`},
		{&st.AwaitingConfirm, `SELECT COUNT(*) FROM erasure_actions WHERE current_state = 'AWAITING_CONFIRMATION'`},
		{&st.ManualPending, `SELECT COUNT(*) FROM erasure_actions WHERE current_state = 'AWAITING_MANUAL'`},
		{&st.Overdue, `SELECT COUNT(*) FROM erasure_actions WHERE statutory_deadline IS NOT NULL AND statutory_deadline < ? AND current_state NOT IN ('COMPLETED','REJECTED')`},
	}
	now := nowRFC3339()
	for _, q := range queries {
		var err error
		if strings.Contains(q.q, "?") {
			err = s.db.QueryRowContext(ctx, q.q, now).Scan(q.dst)
		} else {
			err = s.db.QueryRowContext(ctx, q.q).Scan(q.dst)
		}
		if err != nil {
			return DashboardStats{}, err
		}
	}
	return st, nil
}

func (s *Store) EncryptActionPayload(actionID string, payload []byte) ([]byte, error) {
	return s.encrypt("erasure_actions", "request_payload_enc", actionID, payload)
}

func (s *Store) DecryptActionPayload(a ActionRow) ([]byte, error) {
	return s.decrypt("erasure_actions", "request_payload_enc", a.ID, a.RequestPayloadEnc)
}

func (s *Store) GetIdentity(ctx context.Context, id string) (IdentityRow, error) {
	var r IdentityRow
	var created, updated string
	var middle, dob []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, first_name_enc, last_name_enc, middle_name_enc, dob_enc, created_at, updated_at
		FROM identities WHERE id = ?`, id).Scan(
		&r.ID, &r.FirstNameEnc, &r.LastNameEnc, &middle, &dob, &created, &updated)
	if err == sql.ErrNoRows {
		return IdentityRow{}, nterr.ErrIdentityNotFound
	}
	if err != nil {
		return IdentityRow{}, err
	}
	r.MiddleNameEnc = middle
	r.DOBEnc = dob
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return r, nil
}

func (s *Store) PrimaryIdentity(ctx context.Context) (IdentityRow, error) {
	rows, err := s.ListIdentities(ctx)
	if err != nil {
		return IdentityRow{}, err
	}
	if len(rows) == 0 {
		return IdentityRow{}, nterr.ErrNoIdentity
	}
	return rows[0], nil
}

func (s *Store) CountIdentities(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identities`).Scan(&n)
	return n, err
}

func (s *Store) GetAction(ctx context.Context, id string) (ActionRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, exposed_record_id, broker_id, strategy, current_state, attempts,
			COALESCE(tracking_token, ''), request_payload_enc, response_log_enc,
			statutory_deadline, completed_at, COALESCE(last_error, ''), created_at, updated_at
		FROM erasure_actions WHERE id = ?`, id)
	a, err := scanAction(row)
	if err == sql.ErrNoRows {
		return ActionRow{}, nterr.ErrActionNotFound
	}
	return a, err
}

func (s *Store) HasOpenAction(ctx context.Context, exposedID, strategy string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM erasure_actions
		WHERE exposed_record_id = ? AND strategy = ?
		  AND current_state NOT IN ('COMPLETED','REJECTED')`,
		exposedID, strategy).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) LatestAwaitingByBroker(ctx context.Context, brokerID string) (ActionRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, exposed_record_id, broker_id, strategy, current_state, attempts,
			COALESCE(tracking_token, ''), request_payload_enc, response_log_enc,
			statutory_deadline, completed_at, COALESCE(last_error, ''), created_at, updated_at
		FROM erasure_actions
		WHERE broker_id = ? AND current_state IN ('SUBMITTED','AWAITING_CONFIRMATION','QUEUED')
		ORDER BY created_at DESC LIMIT 1`, brokerID)
	a, err := scanAction(row)
	if err == sql.ErrNoRows {
		return ActionRow{}, nterr.ErrActionNotFound
	}
	return a, err
}

func (s *Store) EncryptActionResponse(actionID string, payload []byte) ([]byte, error) {
	return s.encrypt("erasure_actions", "response_log_enc", actionID, payload)
}

func (s *Store) DecryptActionResponse(a ActionRow) ([]byte, error) {
	return s.decrypt("erasure_actions", "response_log_enc", a.ID, a.ResponseLogEnc)
}

// StaleVerified returns VERIFIED_REMOVED records whose last_verified_at is
// older than cutoff. The scheduler marks these PENDING_REVIEW; it does not
// scrape third-party sites.
func (s *Store) StaleVerified(ctx context.Context, cutoff time.Time) ([]ExposedRecordRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, identity_id, broker_id, COALESCE(profile_url, ''), extracted_data_json_enc,
			confidence_score, status, risk_tier, first_detected_at, last_verified_at
		FROM exposed_records
		WHERE status = 'VERIFIED_REMOVED'
		  AND last_verified_at < ?
		ORDER BY last_verified_at`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExposedRecordRow
	for rows.Next() {
		var r ExposedRecordRow
		var first, last string
		if err := rows.Scan(&r.ID, &r.IdentityID, &r.BrokerID, &r.ProfileURL, &r.ExtractedEnc,
			&r.Confidence, &r.Status, &r.RiskTier, &first, &last); err != nil {
			return nil, err
		}
		r.FirstDetectedAt = parseTime(first)
		r.LastVerifiedAt = parseTime(last)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SetExposedVerified(ctx context.Context, id string) error {
	return s.SetExposedStatus(ctx, id, "VERIFIED_REMOVED")
}
