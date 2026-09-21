package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB        *sql.DB
	Dialect   string // "sqlite" or "postgres"
	Timescale bool   // observations hypertable when YARD_TIMESCALE=1
	dsn       string
}

func Open(dsn string) (*Store, error) {
	if dsn == "" {
		dsn = "file:yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}
	driver, dialect, normalized := parseDSN(dsn)
	db, err := sql.Open(driver, normalized)
	if err != nil {
		return nil, err
	}
	if dialect == "sqlite" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", dialect, err)
	}
	s := &Store{DB: db, Dialect: dialect, dsn: normalized}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.maybeEnableTimescale(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func parseDSN(dsn string) (driver, dialect, normalized string) {
	lower := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return "pgx", "postgres", dsn
	case strings.HasPrefix(lower, "pgx://"):
		return "pgx", "postgres", "postgres://" + strings.TrimPrefix(dsn, "pgx://")
	case strings.HasPrefix(lower, "sqlite://"):
		return "sqlite", "sqlite", strings.TrimPrefix(dsn, "sqlite://")
	default:
		return "sqlite", "sqlite", dsn
	}
}

func (s *Store) rebind(q string) string {
	if s == nil || s.Dialect != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteByte(q[i])
		}
	}
	return b.String()
}

func (s *Store) exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return s.DB.ExecContext(ctx, s.rebind(q), args...)
}

func (s *Store) query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return s.DB.QueryContext(ctx, s.rebind(q), args...)
}

func (s *Store) queryRow(ctx context.Context, q string, args ...any) *sql.Row {
	return s.DB.QueryRowContext(ctx, s.rebind(q), args...)
}

func (s *Store) Close() error { return s.DB.Close() }

// migration is one versioned, forward-only schema change applied inside its
// own transaction and recorded in schema_migrations. Add new entries here
// (never edit existing ones) as the schema grows — see baselineSchema below
// for the pre-versioning DDL that already-deployed databases carry.
type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{1, `CREATE INDEX IF NOT EXISTS observations_org_asset_time ON observations(organization_id, asset_id, observed_at)`},
	{2, `ALTER TABLE users ADD COLUMN active INTEGER NOT NULL DEFAULT 1`},
	{3, `CREATE TABLE IF NOT EXISTS invite_tokens (
  token TEXT PRIMARY KEY, user_id TEXT NOT NULL, expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  token TEXT PRIMARY KEY, user_id TEXT NOT NULL, expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, user_id TEXT NOT NULL, name TEXT NOT NULL,
  token_hash TEXT NOT NULL, token_hint TEXT NOT NULL, created_at TEXT NOT NULL, last_used_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_hash ON api_keys(token_hash);`},
	{4, `ALTER TABLE work_orders ADD COLUMN checklist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE work_orders ADD COLUMN sla_due_at TEXT;
ALTER TABLE work_orders ADD COLUMN schedule_cron TEXT NOT NULL DEFAULT '';`},
	{5, `CREATE TABLE IF NOT EXISTS connector_secrets (
  connector_id TEXT PRIMARY KEY,
  ciphertext TEXT NOT NULL,
  hint TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);`},
	{6, `ALTER TABLE sessions ADD COLUMN id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN created_at TEXT NOT NULL DEFAULT '';`},
	{7, `CREATE TABLE IF NOT EXISTS stream_tickets (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  organization_id TEXT NOT NULL,
  expires_at TEXT NOT NULL
);`},
	{8, `CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  payload TEXT NOT NULL DEFAULT '{}',
  result TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  run_after TEXT NOT NULL,
  locked_by TEXT NOT NULL DEFAULT '',
  locked_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT
);
CREATE INDEX IF NOT EXISTS jobs_claim ON jobs(status, run_after);`},
	{9, `CREATE TABLE IF NOT EXISTS locations (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  parent_id TEXT,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  created_at TEXT NOT NULL
);
ALTER TABLE assets ADD COLUMN parent_asset_id TEXT;
ALTER TABLE assets ADD COLUMN location_id TEXT;
ALTER TABLE assets ADD COLUMN template_id TEXT;
CREATE TABLE IF NOT EXISTS asset_templates (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'equipment',
  capabilities TEXT NOT NULL DEFAULT '[]',
  stale_after_sec INTEGER NOT NULL DEFAULT 90,
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS asset_links (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  from_asset_id TEXT NOT NULL,
  to_asset_id TEXT NOT NULL,
  relation TEXT NOT NULL,
  created_at TEXT NOT NULL
);`},
	{10, `CREATE TABLE IF NOT EXISTS login_attempts (
  id TEXT PRIMARY KEY,
  attempt_key TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS login_attempts_key_time ON login_attempts(attempt_key, created_at);
ALTER TABLE connectors ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE connectors ADD COLUMN last_latency_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE connectors ADD COLUMN sync_interval_sec INTEGER NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS live_events (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  payload TEXT NOT NULL,
  origin TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS live_events_time ON live_events(created_at);`},
	{11, `ALTER TABLE work_orders ADD COLUMN last_fired_at TEXT NOT NULL DEFAULT '';`},
	{12, `CREATE TABLE IF NOT EXISTS attachments (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  name TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS attachments_asset ON attachments(organization_id, asset_id);`},
	{13, `CREATE TABLE IF NOT EXISTS work_order_lines (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  work_order_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  quantity REAL NOT NULL DEFAULT 1,
  unit TEXT NOT NULL DEFAULT '',
  unit_cost_cents INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS work_order_lines_wo ON work_order_lines(organization_id, work_order_id);`},
	{14, `ALTER TABLE organizations ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 0;`},
	{15, `ALTER TABLE observations ADD COLUMN value_kind TEXT NOT NULL DEFAULT 'number';
ALTER TABLE observations ADD COLUMN value_text TEXT NOT NULL DEFAULT '';`},
	{16, `CREATE TABLE IF NOT EXISTS observation_rollups (
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  capability TEXT NOT NULL,
  bucket_start TEXT NOT NULL,
  samples INTEGER NOT NULL,
  value_min REAL NOT NULL,
  value_max REAL NOT NULL,
  value_sum REAL NOT NULL,
  unit TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (organization_id, asset_id, capability, bucket_start)
);`},
	{17, `CREATE TABLE IF NOT EXISTS dashboards (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  panels TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS dashboards_org ON dashboards(organization_id, updated_at);`},
	{18, `CREATE TABLE IF NOT EXISTS automation_holds (
  organization_id TEXT NOT NULL,
  automation_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  since TEXT NOT NULL,
  PRIMARY KEY (organization_id, automation_id, asset_id)
);`},
	{19, `CREATE TABLE IF NOT EXISTS playbooks (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  body TEXT NOT NULL,
  source_url TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS playbook_runs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  playbook_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  connector_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  requested_by TEXT NOT NULL DEFAULT '',
  approved_by TEXT NOT NULL DEFAULT '',
  step_index INTEGER NOT NULL DEFAULT 0,
  dry_run INTEGER NOT NULL DEFAULT 0,
  job_id TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);`},
	{20, `ALTER TABLE assets ADD COLUMN desired_state TEXT NOT NULL DEFAULT '{}';`},
	{21, `CREATE TABLE IF NOT EXISTS permits (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  work_order_id TEXT NOT NULL,
  status TEXT NOT NULL,
  approved_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS permits_wo ON permits(organization_id, work_order_id);`},
	{22, `ALTER TABLE organizations ADD COLUMN energy_cents_per_kwh INTEGER NOT NULL DEFAULT 0;
ALTER TABLE assets ADD COLUMN downtime_cents_per_hour INTEGER NOT NULL DEFAULT 0;
ALTER TABLE assets ADD COLUMN replacement_cost_cents INTEGER NOT NULL DEFAULT 0;`},
	{23, `CREATE TABLE IF NOT EXISTS geofences (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  radius_m REAL NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS geofence_state (
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  geofence_id TEXT NOT NULL,
  inside INTEGER NOT NULL,
  PRIMARY KEY (organization_id, asset_id, geofence_id)
);
ALTER TABLE assets ADD COLUMN floor_x REAL;
ALTER TABLE assets ADD COLUMN floor_y REAL;
ALTER TABLE locations ADD COLUMN floorplan TEXT NOT NULL DEFAULT '';`},
	{24, `CREATE TABLE IF NOT EXISTS tariffs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  start_hour INTEGER NOT NULL,
  end_hour INTEGER NOT NULL,
  cents_per_kwh INTEGER NOT NULL
);
ALTER TABLE organizations ADD COLUMN carbon_grams_per_kwh INTEGER NOT NULL DEFAULT 0;`},
	{25, `CREATE TABLE IF NOT EXISTS roles (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  can_write INTEGER NOT NULL DEFAULT 0,
  UNIQUE (organization_id, name)
);`},
	{26, `CREATE TABLE IF NOT EXISTS assistant_previews (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  requested_by TEXT NOT NULL,
  title TEXT NOT NULL,
  asset_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  created_at TEXT NOT NULL
);`},
	{27, `CREATE TABLE IF NOT EXISTS webauthn_challenges (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  challenge TEXT NOT NULL,
  purpose TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS webauthn_credentials (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  credential_id TEXT NOT NULL,
  public_key TEXT NOT NULL,
  created_at TEXT NOT NULL
);`},
	{28, `ALTER TABLE users ADD COLUMN skills TEXT NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS shifts (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  starts_at TEXT NOT NULL,
  ends_at TEXT NOT NULL
);
ALTER TABLE work_orders ADD COLUMN required_skill TEXT NOT NULL DEFAULT '';`},
	{29, `CREATE TABLE IF NOT EXISTS ingest_budget (
  organization_id TEXT NOT NULL,
  window_start TEXT NOT NULL,
  hits INTEGER NOT NULL,
  PRIMARY KEY (organization_id, window_start)
);`},
	{30, `ALTER TABLE events ADD COLUMN region TEXT NOT NULL DEFAULT '';`},
	{31, `ALTER TABLE incidents ADD COLUMN parent_id TEXT;
ALTER TABLE incidents ADD COLUMN flap_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE incidents ADD COLUMN flap_armed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE incidents ADD COLUMN acked_at TEXT;
ALTER TABLE incidents ADD COLUMN ack_due_at TEXT;
ALTER TABLE incidents ADD COLUMN resolve_due_at TEXT;
ALTER TABLE organizations ADD COLUMN ack_minutes INTEGER NOT NULL DEFAULT 15;
ALTER TABLE organizations ADD COLUMN resolve_minutes INTEGER NOT NULL DEFAULT 240;
CREATE TABLE IF NOT EXISTS oncall (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  starts_at TEXT NOT NULL,
  ends_at TEXT NOT NULL
);`},
	{32, `ALTER TABLE events ADD COLUMN replicated INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';`},
	{33, `ALTER TABLE assets ADD COLUMN nfc_id TEXT NOT NULL DEFAULT '';
ALTER TABLE assets ADD COLUMN purchased_at TEXT;
ALTER TABLE assets ADD COLUMN warranty_expires_at TEXT;
ALTER TABLE assets ADD COLUMN purchase_cents INTEGER NOT NULL DEFAULT 0;
ALTER TABLE assets ADD COLUMN useful_life_months INTEGER NOT NULL DEFAULT 0;
ALTER TABLE assets ADD COLUMN deleted_at TEXT;
CREATE TABLE IF NOT EXISTS install_history (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  parent_asset_id TEXT,
  note TEXT NOT NULL DEFAULT '',
  installed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS asset_bom (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  part_number TEXT NOT NULL,
  name TEXT NOT NULL,
  quantity REAL NOT NULL DEFAULT 1,
  unit TEXT NOT NULL DEFAULT 'ea'
);
CREATE TABLE IF NOT EXISTS catalogs (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  template_ids TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS work_order_assets (
  work_order_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  PRIMARY KEY (work_order_id, asset_id)
);
CREATE TABLE IF NOT EXISTS time_entries (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  work_order_id TEXT NOT NULL,
  actor TEXT NOT NULL,
  minutes INTEGER NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS saved_views (
  id TEXT PRIMARY KEY,
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  query TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS org_members (
  organization_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'viewer',
  PRIMARY KEY (organization_id, user_id)
);
ALTER TABLE users ADD COLUMN widgets TEXT NOT NULL DEFAULT '["health","incidents","work","activity"]';
ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT 'en';`},
	{34, `ALTER TABLE observations ADD COLUMN sequence_num INTEGER NOT NULL DEFAULT 0;
ALTER TABLE observations ADD COLUMN quality_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE observations ADD COLUMN uncertainty REAL;
ALTER TABLE observations ADD COLUMN calibration_state TEXT NOT NULL DEFAULT '';`},
}

// baselineSchema is the idempotent CREATE TABLE IF NOT EXISTS block this
// project used before schema_migrations existed. Left exactly as-is (and not
// migration-tracked) so already-deployed databases are unaffected; every new
// schema change from here on is a versioned entry in `migrations` instead.
const baselineSchema = `
CREATE TABLE IF NOT EXISTS organizations (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, email TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL, role TEXT NOT NULL, password_hash TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY, user_id TEXT NOT NULL, expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sites (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL,
  address TEXT NOT NULL DEFAULT '', latitude REAL, longitude REAL, timezone TEXT NOT NULL DEFAULT 'UTC',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS assets (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, site_id TEXT,
  name TEXT NOT NULL, external_ref TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active', health TEXT NOT NULL DEFAULT 'unknown',
  manufacturer TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '', serial TEXT NOT NULL DEFAULT '',
  latitude REAL, longitude REAL, last_seen_at TEXT, stale_after_sec INTEGER NOT NULL DEFAULT 90,
  metadata TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS assets_org_ref ON assets(organization_id, external_ref) WHERE external_ref != '';
CREATE TABLE IF NOT EXISTS capabilities (
  id TEXT PRIMARY KEY, asset_id TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL,
  unit TEXT NOT NULL DEFAULT '', min REAL, max REAL, writable INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS observations (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, asset_id TEXT NOT NULL,
  capability TEXT NOT NULL, value REAL NOT NULL, unit TEXT NOT NULL DEFAULT '',
  quality TEXT NOT NULL DEFAULT 'good', source TEXT NOT NULL DEFAULT '',
  observed_at TEXT NOT NULL, received_at TEXT NOT NULL, dedupe_key TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS observations_dedupe ON observations(organization_id, dedupe_key) WHERE dedupe_key != '';
CREATE INDEX IF NOT EXISTS observations_asset_time ON observations(asset_id, observed_at);
CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, asset_id TEXT, site_id TEXT,
  kind TEXT NOT NULL, severity TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '',
  dedupe_key TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS events_dedupe ON events(organization_id, dedupe_key) WHERE dedupe_key != '';
CREATE TABLE IF NOT EXISTS incidents (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, asset_id TEXT, site_id TEXT,
  title TEXT NOT NULL, severity TEXT NOT NULL, status TEXT NOT NULL, owner TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '', resolution TEXT NOT NULL DEFAULT '',
  runbook TEXT NOT NULL DEFAULT '',
  opened_at TEXT NOT NULL, resolved_at TEXT
);
CREATE TABLE IF NOT EXISTS work_orders (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, asset_id TEXT, site_id TEXT, incident_id TEXT,
  title TEXT NOT NULL, kind TEXT NOT NULL, priority TEXT NOT NULL, status TEXT NOT NULL,
  assignee TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', due_at TEXT,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS connectors (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL,
  status TEXT NOT NULL, endpoint TEXT NOT NULL DEFAULT '', token_hint TEXT NOT NULL DEFAULT '',
  token_hash TEXT NOT NULL DEFAULT '', actions TEXT NOT NULL DEFAULT '[]',
  config TEXT NOT NULL DEFAULT '{}', last_sync_at TEXT, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS action_requests (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, asset_id TEXT, connector_id TEXT,
  action TEXT NOT NULL, idempotency_key TEXT NOT NULL, status TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}', result TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL, created_at TEXT NOT NULL, completed_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS action_idem ON action_requests(organization_id, idempotency_key);
CREATE TABLE IF NOT EXISTS automations (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, name TEXT NOT NULL, enabled INTEGER NOT NULL,
  trigger_kind TEXT NOT NULL, capability TEXT NOT NULL DEFAULT '', operator TEXT NOT NULL DEFAULT 'gt',
  threshold REAL NOT NULL DEFAULT 0, action TEXT NOT NULL, config TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_log (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, actor TEXT NOT NULL, action TEXT NOT NULL,
  object TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS severity_policies (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, name TEXT NOT NULL,
  match_kind TEXT NOT NULL, match_value TEXT NOT NULL DEFAULT '',
  severity TEXT NOT NULL, runbook TEXT NOT NULL DEFAULT '',
  priority INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
`

func (s *Store) migrate() error {
	if _, err := s.DB.Exec(baselineSchema); err != nil {
		return err
	}
	// Best-effort upgrades for DBs created before newer columns existed on
	// the baseline tables above (pre-dates schema_migrations; left as-is).
	_, _ = s.DB.Exec(`ALTER TABLE automations ADD COLUMN config TEXT NOT NULL DEFAULT '{}'`)
	_, _ = s.DB.Exec(`ALTER TABLE incidents ADD COLUMN runbook TEXT NOT NULL DEFAULT ''`)
	return s.applyMigrations()
}

func (s *Store) applyMigrations() error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL
)`); err != nil {
		return err
	}
	applied := map[int]bool{}
	rows, err := s.DB.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(s.migrationSQL(m)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("schema migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(s.rebind(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`), m.version, now()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("schema migration %d: record: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("schema migration %d: commit: %w", m.version, err)
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

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

func nullFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func floatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

func nullF(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

func nullS(n sql.NullString) *string {
	if !n.Valid || n.String == "" {
		return nil
	}
	v := n.String
	return &v
}

func (s *Store) CreateOrganization(ctx context.Context, name, slug string) (*model.Organization, error) {
	o := &model.Organization{ID: idgen.New("org"), Name: name, Slug: slug, CreatedAt: time.Now().UTC()}
	_, err := s.exec(ctx, `INSERT INTO organizations(id,name,slug,created_at) VALUES(?,?,?,?)`, o.ID, o.Name, o.Slug, o.CreatedAt.Format(time.RFC3339Nano))
	return o, err
}

func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	if u.ID == "" {
		u.ID = idgen.New("usr")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	_, err := s.exec(ctx, `INSERT INTO users(id,organization_id,email,display_name,role,password_hash,active,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		u.ID, u.OrganizationID, strings.ToLower(u.Email), u.DisplayName, u.Role, u.PasswordHash, boolInt(u.Active), u.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,email,display_name,role,password_hash,active,created_at FROM users WHERE email=?`, strings.ToLower(email))
	return scanUser(row)
}

func (s *Store) UserByID(ctx context.Context, id string) (*model.User, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,email,display_name,role,password_hash,active,created_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (s *Store) ListUsers(ctx context.Context, orgID string) ([]model.User, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,email,display_name,role,password_hash,active,created_at FROM users WHERE organization_id=? ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	if out == nil {
		out = []model.User{}
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserRole(ctx context.Context, orgID, userID, role string) error {
	res, err := s.exec(ctx, `UPDATE users SET role=? WHERE id=? AND organization_id=?`, role, userID, orgID)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (s *Store) SetUserActive(ctx context.Context, orgID, userID string, active bool) error {
	res, err := s.exec(ctx, `UPDATE users SET active=? WHERE id=? AND organization_id=?`, boolInt(active), userID, orgID)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (s *Store) SetUserPassword(ctx context.Context, userID, passwordHash string) error {
	res, err := s.exec(ctx, `UPDATE users SET password_hash=? WHERE id=?`, passwordHash, userID)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*model.User, error) {
	var u model.User
	var created string
	var active int
	if err := row.Scan(&u.ID, &u.OrganizationID, &u.Email, &u.DisplayName, &u.Role, &u.PasswordHash, &active, &created); err != nil {
		return nil, err
	}
	u.Active = active != 0
	u.CreatedAt = parseTime(created)
	return &u, nil
}

// CreateInviteToken issues a one-time, expiring token that lets a newly
// admin-created user set their own password (POST /api/v1/auth/accept-invite).
func (s *Store) CreateInviteToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	token := idgen.Secret(24)
	_, err := s.exec(ctx, `INSERT INTO invite_tokens(token,user_id,expires_at) VALUES(?,?,?)`,
		token, userID, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano))
	return token, err
}

// ConsumeInviteToken validates and deletes an invite token in one step, so
// it can't be replayed. Returns sql.ErrNoRows if the token is unknown or
// expired.
func (s *Store) ConsumeInviteToken(ctx context.Context, token string) (string, error) {
	return consumeToken(ctx, s, "invite_tokens", token)
}

// CreatePasswordResetToken issues a one-time, expiring token for the
// self-service "forgot password" flow.
func (s *Store) CreatePasswordResetToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	token := idgen.Secret(24)
	_, err := s.exec(ctx, `INSERT INTO password_reset_tokens(token,user_id,expires_at) VALUES(?,?,?)`,
		token, userID, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano))
	return token, err
}

// ConsumePasswordResetToken validates and deletes a password reset token in
// one step. Returns sql.ErrNoRows if the token is unknown or expired.
func (s *Store) ConsumePasswordResetToken(ctx context.Context, token string) (string, error) {
	return consumeToken(ctx, s, "password_reset_tokens", token)
}

// consumeToken looks up a (token, user_id, expires_at) row in one of the
// short-lived token tables (invite_tokens, password_reset_tokens), deletes
// it regardless of outcome so it's never usable twice, and reports
// sql.ErrNoRows for an unknown or expired token.
func consumeToken(ctx context.Context, s *Store, table, token string) (string, error) {
	var userID, exp string
	err := s.queryRow(ctx, `SELECT user_id, expires_at FROM `+table+` WHERE token=?`, token).Scan(&userID, &exp)
	if err != nil {
		return "", err
	}
	_, _ = s.exec(ctx, `DELETE FROM `+table+` WHERE token=?`, token)
	if parseTime(exp).Before(time.Now().UTC()) {
		return "", sql.ErrNoRows
	}
	return userID, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string, ttl time.Duration) (*model.Session, error) {
	now := time.Now().UTC()
	sess := &model.Session{
		ID:        idgen.New("sess"),
		Token:     idgen.Secret(24),
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	_, err := s.exec(ctx, `INSERT INTO sessions(token,user_id,expires_at,id,created_at) VALUES(?,?,?,?,?)`,
		sess.Token, sess.UserID, sess.ExpiresAt.Format(time.RFC3339Nano), sess.ID, sess.CreatedAt.Format(time.RFC3339Nano))
	return sess, err
}

func (s *Store) ListSessions(ctx context.Context, userID string) ([]model.Session, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, expires_at, created_at FROM sessions WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var sess model.Session
		var exp, created string
		if err := rows.Scan(&sess.ID, &sess.UserID, &exp, &created); err != nil {
			return nil, err
		}
		sess.ExpiresAt = parseTime(exp)
		sess.CreatedAt = parseTime(created)
		if sess.ExpiresAt.Before(time.Now().UTC()) {
			continue
		}
		out = append(out, sess)
	}
	if out == nil {
		out = []model.Session{}
	}
	return out, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM sessions WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (s *Store) DeleteSessionToken(ctx context.Context, userID, token string) error {
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE user_id=? AND token=?`, userID, token)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

func (s *Store) PutConnectorSecret(ctx context.Context, connectorID, ciphertext, hint string) error {
	_, err := s.exec(ctx, `INSERT INTO connector_secrets(connector_id, ciphertext, hint, updated_at) VALUES(?,?,?,?)
ON CONFLICT(connector_id) DO UPDATE SET ciphertext=excluded.ciphertext, hint=excluded.hint, updated_at=excluded.updated_at`,
		connectorID, ciphertext, hint, now())
	return err
}

func (s *Store) GetConnectorSecret(ctx context.Context, connectorID string) (ciphertext, hint string, err error) {
	err = s.queryRow(ctx, `SELECT ciphertext, hint FROM connector_secrets WHERE connector_id=?`, connectorID).Scan(&ciphertext, &hint)
	return ciphertext, hint, err
}

func (s *Store) CreateStreamTicket(ctx context.Context, tokenHash, userID, orgID string, ttl time.Duration) error {
	_, err := s.exec(ctx, `INSERT INTO stream_tickets(token_hash, user_id, organization_id, expires_at) VALUES(?,?,?,?)`,
		tokenHash, userID, orgID, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano))
	return err
}

func (s *Store) ConsumeStreamTicket(ctx context.Context, tokenHash string) (userID, orgID string, err error) {
	var exp string
	err = s.queryRow(ctx, `SELECT user_id, organization_id, expires_at FROM stream_tickets WHERE token_hash=?`, tokenHash).Scan(&userID, &orgID, &exp)
	if err != nil {
		return "", "", err
	}
	_, _ = s.exec(ctx, `DELETE FROM stream_tickets WHERE token_hash=?`, tokenHash)
	if parseTime(exp).Before(time.Now().UTC()) {
		return "", "", sql.ErrNoRows
	}
	return userID, orgID, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.DB.PingContext(ctx)
}

func LatestSchemaVersion() int {
	if len(migrations) == 0 {
		return 0
	}
	return migrations[len(migrations)-1].version
}

func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.queryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}

func (s *Store) SessionUser(ctx context.Context, token string) (*model.User, error) {
	var userID, exp string
	err := s.queryRow(ctx, `SELECT user_id, expires_at FROM sessions WHERE token=?`, token).Scan(&userID, &exp)
	if err != nil {
		return nil, err
	}
	if parseTime(exp).Before(time.Now().UTC()) {
		return nil, sql.ErrNoRows
	}
	u, err := s.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Deactivating a user (Admin > Users) must take effect immediately, not
	// just block their next login — an existing session token would
	// otherwise keep working until its 12h expiry.
	if !u.Active {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (s *Store) CountOrgs(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM organizations`).Scan(&n)
	return n, err
}

func (s *Store) UpsertSite(ctx context.Context, site *model.Site) error {
	if site.ID == "" {
		site.ID = idgen.New("site")
	}
	if site.CreatedAt.IsZero() {
		site.CreatedAt = time.Now().UTC()
	}
	_, err := s.exec(ctx, `INSERT INTO sites(id,organization_id,name,kind,address,latitude,longitude,timezone,created_at)
VALUES(?,?,?,?,?,?,?,?,?)`, site.ID, site.OrganizationID, site.Name, site.Kind, site.Address, site.Latitude, site.Longitude, site.Timezone, site.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListSites(ctx context.Context, orgID string) ([]model.Site, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,kind,address,latitude,longitude,timezone,created_at FROM sites WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Site
	for rows.Next() {
		var st model.Site
		var created string
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&st.ID, &st.OrganizationID, &st.Name, &st.Kind, &st.Address, &lat, &lng, &st.Timezone, &created); err != nil {
			return nil, err
		}
		st.Latitude, st.Longitude = nullF(lat), nullF(lng)
		st.CreatedAt = parseTime(created)
		out = append(out, st)
	}
	if out == nil {
		out = []model.Site{}
	}
	return out, rows.Err()
}

func (s *Store) SiteByName(ctx context.Context, orgID, name string) (*model.Site, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,address,latitude,longitude,timezone,created_at FROM sites WHERE organization_id=? AND name=?`, orgID, name)
	var st model.Site
	var created string
	var lat, lng sql.NullFloat64
	if err := row.Scan(&st.ID, &st.OrganizationID, &st.Name, &st.Kind, &st.Address, &lat, &lng, &st.Timezone, &created); err != nil {
		return nil, err
	}
	st.Latitude, st.Longitude = nullF(lat), nullF(lng)
	st.CreatedAt = parseTime(created)
	return &st, nil
}

func (s *Store) UpsertAsset(ctx context.Context, a *model.Asset) error {
	if a.ID == "" {
		a.ID = idgen.New("ast")
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "active"
	}
	if a.Health == "" {
		a.Health = "unknown"
	}
	if a.StaleAfterSec == 0 {
		a.StaleAfterSec = 90
	}
	if a.Metadata == "" {
		a.Metadata = "{}"
	}
	_, err := s.exec(ctx, `INSERT INTO assets(id,organization_id,site_id,name,external_ref,kind,status,health,manufacturer,model,serial,latitude,longitude,last_seen_at,stale_after_sec,metadata,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, site_id=excluded.site_id, kind=excluded.kind, status=excluded.status, health=excluded.health,
manufacturer=excluded.manufacturer, model=excluded.model, serial=excluded.serial, latitude=excluded.latitude, longitude=excluded.longitude,
last_seen_at=excluded.last_seen_at, metadata=excluded.metadata, updated_at=excluded.updated_at`,
		a.ID, a.OrganizationID, a.SiteID, a.Name, a.ExternalRef, a.Kind, a.Status, a.Health, a.Manufacturer, a.Model, a.Serial, a.Latitude, a.Longitude,
		ts(a.LastSeenAt), a.StaleAfterSec, a.Metadata, a.CreatedAt.Format(time.RFC3339Nano), a.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func ts(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

const assetSelect = `id,organization_id,site_id,name,external_ref,kind,status,health,manufacturer,model,serial,latitude,longitude,last_seen_at,stale_after_sec,metadata,created_at,updated_at,parent_asset_id,location_id,template_id,desired_state,downtime_cents_per_hour,replacement_cost_cents,floor_x,floor_y,nfc_id,purchased_at,warranty_expires_at,purchase_cents,useful_life_months,deleted_at`

func (s *Store) AssetByRef(ctx context.Context, orgID, ref string) (*model.Asset, error) {
	return s.getAsset(ctx, `SELECT `+assetSelect+` FROM assets WHERE organization_id=? AND external_ref=?`, orgID, ref)
}

func (s *Store) AssetByID(ctx context.Context, orgID, id string) (*model.Asset, error) {
	return s.getAsset(ctx, `SELECT `+assetSelect+` FROM assets WHERE organization_id=? AND id=?`, orgID, id)
}

func (s *Store) getAsset(ctx context.Context, q string, args ...any) (*model.Asset, error) {
	row := s.queryRow(ctx, q, args...)
	a, err := scanAsset(row)
	return a, err
}

func scanAsset(row scannable) (*model.Asset, error) {
	var a model.Asset
	var site sql.NullString
	var lat, lng sql.NullFloat64
	var last, created, updated string
	var lastN sql.NullString
	var parent, location, template sql.NullString
	var floorX, floorY sql.NullFloat64
	var purchased, warranty, deleted sql.NullString
	if err := row.Scan(&a.ID, &a.OrganizationID, &site, &a.Name, &a.ExternalRef, &a.Kind, &a.Status, &a.Health, &a.Manufacturer, &a.Model, &a.Serial, &lat, &lng, &lastN, &a.StaleAfterSec, &a.Metadata, &created, &updated, &parent, &location, &template, &a.DesiredState, &a.DowntimeCentsPerHour, &a.ReplacementCostCents, &floorX, &floorY, &a.NFCID, &purchased, &warranty, &a.PurchaseCents, &a.UsefulLifeMonths, &deleted); err != nil {
		return nil, err
	}
	a.SiteID = nullS(site)
	a.ParentAssetID, a.LocationID, a.TemplateID = nullS(parent), nullS(location), nullS(template)
	a.Latitude, a.Longitude = nullF(lat), nullF(lng)
	a.FloorX, a.FloorY = nullF(floorX), nullF(floorY)
	a.LastSeenAt = parseTimePtr(lastN)
	a.PurchasedAt = parseTimePtr(purchased)
	a.WarrantyExpiresAt = parseTimePtr(warranty)
	a.DeletedAt = parseTimePtr(deleted)
	_ = last
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	a.BookValueCents = bookValue(a)
	return &a, nil
}

func bookValue(a model.Asset) int {
	if a.PurchaseCents <= 0 || a.UsefulLifeMonths <= 0 || a.PurchasedAt == nil {
		return a.PurchaseCents
	}
	months := int(time.Since(a.PurchasedAt.UTC()).Hours() / (24 * 30))
	if months < 0 {
		months = 0
	}
	if months >= a.UsefulLifeMonths {
		return 0
	}
	return a.PurchaseCents * (a.UsefulLifeMonths - months) / a.UsefulLifeMonths
}

func (s *Store) ListAssets(ctx context.Context, orgID, q, kind, health string) ([]model.Asset, error) {
	query := `SELECT ` + assetSelect + ` FROM assets WHERE organization_id=? AND (deleted_at IS NULL OR deleted_at='')`
	args := []any{orgID}
	if kind != "" {
		query += ` AND kind=?`
		args = append(args, kind)
	}
	if health != "" {
		query += ` AND health=?`
		args = append(args, health)
	}
	if q != "" {
		query += ` AND (lower(name) LIKE ? OR lower(external_ref) LIKE ? OR lower(serial) LIKE ?)`
		like := "%" + strings.ToLower(q) + "%"
		args = append(args, like, like, like)
	}
	query += ` ORDER BY name`
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if out == nil {
		out = []model.Asset{}
	}
	return out, rows.Err()
}

func (s *Store) TouchAsset(ctx context.Context, id string, lat, lng *float64, health string) error {
	_, err := s.exec(ctx, `UPDATE assets SET last_seen_at=?, latitude=COALESCE(?, latitude), longitude=COALESCE(?, longitude), health=CASE WHEN ? != '' THEN ? ELSE health END, updated_at=? WHERE id=?`,
		now(), lat, lng, health, health, now(), id)
	return err
}

func (s *Store) SetAssetHealth(ctx context.Context, id, health string) error {
	_, err := s.exec(ctx, `UPDATE assets SET health=?, updated_at=? WHERE id=?`, health, now(), id)
	return err
}

func (s *Store) RefreshStale(ctx context.Context, orgID string) (int, error) {
	rows, err := s.query(ctx, `SELECT id, last_seen_at, stale_after_sec, health FROM assets WHERE organization_id=?`, orgID)
	if err != nil {
		return 0, err
	}
	nowT := time.Now().UTC()
	var ids []string
	for rows.Next() {
		var id, health string
		var last sql.NullString
		var stale int
		if err := rows.Scan(&id, &last, &stale, &health); err != nil {
			rows.Close()
			return 0, err
		}
		isStale := !last.Valid || parseTime(last.String).Add(time.Duration(stale)*time.Second).Before(nowT)
		if isStale && health != "stale" && health != "critical" {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, id := range ids {
		if err := s.SetAssetHealth(ctx, id, "stale"); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (s *Store) ReplaceCapabilities(ctx context.Context, assetID string, caps []model.Capability) error {
	_, _ = s.exec(ctx, `DELETE FROM capabilities WHERE asset_id=?`, assetID)
	for _, c := range caps {
		if c.ID == "" {
			c.ID = idgen.New("cap")
		}
		if _, err := s.exec(ctx, `INSERT INTO capabilities(id,asset_id,name,kind,unit,min,max,writable) VALUES(?,?,?,?,?,?,?,?)`,
			c.ID, assetID, c.Name, c.Kind, c.Unit, c.Min, c.Max, boolInt(c.Writable)); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *Store) ListCapabilities(ctx context.Context, assetID string) ([]model.Capability, error) {
	rows, err := s.query(ctx, `SELECT id,asset_id,name,kind,unit,min,max,writable FROM capabilities WHERE asset_id=?`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Capability
	for rows.Next() {
		var c model.Capability
		var min, max sql.NullFloat64
		var w int
		if err := rows.Scan(&c.ID, &c.AssetID, &c.Name, &c.Kind, &c.Unit, &min, &max, &w); err != nil {
			return nil, err
		}
		c.Min, c.Max, c.Writable = nullF(min), nullF(max), w == 1
		out = append(out, c)
	}
	if out == nil {
		out = []model.Capability{}
	}
	return out, rows.Err()
}

func obsKind(kind string) string {
	if kind == "" {
		return "number"
	}
	return kind
}

func (s *Store) InsertObservation(ctx context.Context, o *model.Observation) (bool, error) {
	if o.ID == "" {
		o.ID = idgen.New("obs")
	}
	if o.ReceivedAt.IsZero() {
		o.ReceivedAt = time.Now().UTC()
	}
	if o.Quality == "" {
		o.Quality = "good"
	}
	if s.Timescale && o.DedupeKey != "" {
		res, err := s.exec(ctx, `INSERT INTO observation_dedupe(organization_id, dedupe_key) VALUES(?,?) ON CONFLICT DO NOTHING`, o.OrganizationID, o.DedupeKey)
		if err != nil {
			return false, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return false, nil
		}
	}
	_, err := s.exec(ctx, `INSERT INTO observations(id,organization_id,asset_id,capability,value,unit,quality,source,observed_at,received_at,dedupe_key,value_kind,value_text,sequence_num,quality_reason,uncertainty,calibration_state)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, o.ID, o.OrganizationID, o.AssetID, o.Capability, o.Value, o.Unit, o.Quality, o.Source, o.ObservedAt.Format(time.RFC3339Nano), o.ReceivedAt.Format(time.RFC3339Nano), o.DedupeKey, obsKind(o.ValueKind), o.ValueText, o.SequenceNum, o.QualityReason, nullFloat(o.Uncertainty), o.Calibration)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListObservations returns observations for an org (optionally scoped to one
// asset/capability), newest first. from/to bound observed_at when non-zero;
// pass zero time.Time values for an unbounded range.
func (s *Store) ListObservations(ctx context.Context, orgID, assetID, cap string, from, to time.Time, limit int) ([]model.Observation, error) {
	if limit <= 0 || limit > 2000 {
		limit = 400
	}
	q := `SELECT id,organization_id,asset_id,capability,value,unit,quality,source,observed_at,received_at,dedupe_key,value_kind,value_text,sequence_num,quality_reason,uncertainty,calibration_state FROM observations WHERE organization_id=?`
	args := []any{orgID}
	if assetID != "" {
		q += ` AND asset_id=?`
		args = append(args, assetID)
	}
	if cap != "" {
		q += ` AND capability=?`
		args = append(args, cap)
	}
	if !from.IsZero() {
		q += ` AND observed_at >= ?`
		args = append(args, from.UTC().Format(time.RFC3339Nano))
	}
	if !to.IsZero() {
		q += ` AND observed_at <= ?`
		args = append(args, to.UTC().Format(time.RFC3339Nano))
	}
	q += ` ORDER BY observed_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Observation
	for rows.Next() {
		var o model.Observation
		var obs, rec string
		var unc sql.NullFloat64
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.AssetID, &o.Capability, &o.Value, &o.Unit, &o.Quality, &o.Source, &obs, &rec, &o.DedupeKey, &o.ValueKind, &o.ValueText, &o.SequenceNum, &o.QualityReason, &unc, &o.Calibration); err != nil {
			return nil, err
		}
		o.ObservedAt = parseTime(obs)
		o.ReceivedAt = parseTime(rec)
		o.Uncertainty = floatPtr(unc)
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rolls, err := s.listRollups(ctx, orgID, assetID, cap, from, to, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, rolls...)
	sort.Slice(out, func(i, j int) bool { return out[i].ObservedAt.After(out[j].ObservedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []model.Observation{}
	}
	return out, nil
}

func (s *Store) LatestTelemetry(ctx context.Context, orgID string) ([]model.TelemetryPoint, error) {
	rows, err := s.query(ctx, `
SELECT o.asset_id, a.name, o.capability, o.value, o.unit, o.quality, o.source, o.observed_at, o.received_at, a.stale_after_sec, o.value_kind, o.value_text
FROM observations o
JOIN assets a ON a.id = o.asset_id
JOIN (
  SELECT asset_id, capability, MAX(observed_at) AS mx
  FROM observations WHERE organization_id=? GROUP BY asset_id, capability
) t ON t.asset_id=o.asset_id AND t.capability=o.capability AND t.mx=o.observed_at
WHERE o.organization_id=?
ORDER BY a.name, o.capability
`, orgID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nowT := time.Now().UTC()
	var out []model.TelemetryPoint
	for rows.Next() {
		var p model.TelemetryPoint
		var obs, rec string
		var stale int
		if err := rows.Scan(&p.AssetID, &p.AssetName, &p.Capability, &p.Value, &p.Unit, &p.Quality, &p.Source, &obs, &rec, &stale, &p.ValueKind, &p.ValueText); err != nil {
			return nil, err
		}
		p.ObservedAt = parseTime(obs)
		p.ReceivedAt = parseTime(rec)
		p.Fresh = p.ObservedAt.Add(time.Duration(stale) * time.Second).After(nowT)
		out = append(out, p)
	}
	if out == nil {
		out = []model.TelemetryPoint{}
	}
	return out, rows.Err()
}

func (s *Store) InsertEvent(ctx context.Context, e *model.Event) (bool, error) {
	if e.ID == "" {
		e.ID = idgen.New("evt")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	region := e.Region
	if region == "" {
		region = os.Getenv("YARD_REGION")
	}
	e.Region = region
	replicated := 0
	if e.Replicated {
		replicated = 1
	}
	_, err := s.exec(ctx, `INSERT INTO events(id,organization_id,asset_id,site_id,kind,severity,title,body,dedupe_key,created_at,region,replicated)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.OrganizationID, e.AssetID, e.SiteID, e.Kind, e.Severity, e.Title, e.Body, e.DedupeKey, e.CreatedAt.Format(time.RFC3339Nano), region, replicated)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) ListEvents(ctx context.Context, orgID string, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 80
	}
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,site_id,kind,severity,title,body,dedupe_key,created_at FROM events WHERE organization_id=? ORDER BY created_at DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var asset, site sql.NullString
		var created string
		if err := rows.Scan(&e.ID, &e.OrganizationID, &asset, &site, &e.Kind, &e.Severity, &e.Title, &e.Body, &e.DedupeKey, &created); err != nil {
			return nil, err
		}
		e.AssetID, e.SiteID = nullS(asset), nullS(site)
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	if out == nil {
		out = []model.Event{}
	}
	return out, rows.Err()
}

const incidentCols = `id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,COALESCE(runbook,''),opened_at,resolved_at,parent_id,flap_count,flap_armed,acked_at,ack_due_at,resolve_due_at`

func (s *Store) CreateIncident(ctx context.Context, inc *model.Incident) error {
	if inc.ID == "" {
		inc.ID = idgen.New("inc")
	}
	if inc.OpenedAt.IsZero() {
		inc.OpenedAt = time.Now().UTC()
	}
	if inc.Status == "" {
		inc.Status = "open"
	}
	if err := s.applyIncidentDefaults(ctx, inc); err != nil {
		return err
	}
	var parent any
	if inc.ParentID != "" {
		parent = inc.ParentID
	}
	armed := 0
	if inc.FlapArmed {
		armed = 1
	}
	_, err := s.exec(ctx, `INSERT INTO incidents(id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,runbook,opened_at,resolved_at,parent_id,flap_count,flap_armed,acked_at,ack_due_at,resolve_due_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, inc.ID, inc.OrganizationID, inc.AssetID, inc.SiteID, inc.Title, inc.Severity, inc.Status, inc.Owner, inc.Summary, inc.Resolution, inc.Runbook, inc.OpenedAt.Format(time.RFC3339Nano), ts(inc.ResolvedAt), parent, inc.FlapCount, armed, ts(inc.AckedAt), ts(inc.AckDueAt), ts(inc.ResolveDueAt))
	if err != nil {
		return err
	}
	markIncidentSLA(inc)
	return nil
}

func (s *Store) UpdateIncident(ctx context.Context, inc *model.Incident) error {
	_, err := s.exec(ctx, `UPDATE incidents SET title=?, severity=?, status=?, owner=?, summary=?, resolution=?, runbook=?, resolved_at=?, acked_at=? WHERE id=? AND organization_id=?`,
		inc.Title, inc.Severity, inc.Status, inc.Owner, inc.Summary, inc.Resolution, inc.Runbook, ts(inc.ResolvedAt), ts(inc.AckedAt), inc.ID, inc.OrganizationID)
	if err != nil {
		return err
	}
	markIncidentSLA(inc)
	return nil
}

func (s *Store) GetIncident(ctx context.Context, orgID, id string) (*model.Incident, error) {
	row := s.queryRow(ctx, `SELECT `+incidentCols+` FROM incidents WHERE organization_id=? AND id=?`, orgID, id)
	return scanIncident(row)
}

func scanIncident(row scannable) (*model.Incident, error) {
	var inc model.Incident
	var asset, site, resolved, parent, acked, ackDue, resolveDue sql.NullString
	var opened string
	var armed int
	if err := row.Scan(&inc.ID, &inc.OrganizationID, &asset, &site, &inc.Title, &inc.Severity, &inc.Status, &inc.Owner, &inc.Summary, &inc.Resolution, &inc.Runbook, &opened, &resolved, &parent, &inc.FlapCount, &armed, &acked, &ackDue, &resolveDue); err != nil {
		return nil, err
	}
	inc.AssetID, inc.SiteID = nullS(asset), nullS(site)
	inc.OpenedAt = parseTime(opened)
	inc.ResolvedAt = parseTimePtr(resolved)
	if parent.Valid {
		inc.ParentID = parent.String
	}
	inc.FlapArmed = armed == 1
	inc.AckedAt = parseTimePtr(acked)
	inc.AckDueAt = parseTimePtr(ackDue)
	inc.ResolveDueAt = parseTimePtr(resolveDue)
	markIncidentSLA(&inc)
	return &inc, nil
}

func (s *Store) ListIncidents(ctx context.Context, orgID, status string) ([]model.Incident, error) {
	q := `SELECT ` + incidentCols + ` FROM incidents WHERE organization_id=?`
	args := []any{orgID}
	if status != "" {
		q += ` AND status=?`
		args = append(args, status)
	}
	q += ` ORDER BY opened_at DESC`
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inc)
	}
	if out == nil {
		out = []model.Incident{}
	}
	return out, rows.Err()
}

func (s *Store) OpenIncidentByDedupe(ctx context.Context, orgID, title string, assetID *string) (*model.Incident, bool, error) {
	q := `SELECT ` + incidentCols + ` FROM incidents WHERE organization_id=? AND title=? AND status IN ('open','ack')`
	if assetID != nil {
		q += ` AND asset_id=?`
		row := s.queryRow(ctx, q, orgID, title, *assetID)
		inc, err := scanIncident(row)
		if err == nil {
			return inc, false, nil
		}
		if err != sql.ErrNoRows {
			return nil, false, err
		}
	}
	return nil, true, nil
}

func (s *Store) CreateWorkOrder(ctx context.Context, wo *model.WorkOrder) error {
	if wo.ID == "" {
		wo.ID = idgen.New("wo")
	}
	nowT := time.Now().UTC()
	if wo.CreatedAt.IsZero() {
		wo.CreatedAt = nowT
	}
	wo.UpdatedAt = nowT
	if wo.Status == "" {
		wo.Status = "open"
	}
	if wo.Checklist == "" {
		wo.Checklist = "[]"
	}
	_, err := s.exec(ctx, `INSERT INTO work_orders(id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,checklist,schedule_cron,due_at,sla_due_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, wo.ID, wo.OrganizationID, wo.AssetID, wo.SiteID, wo.IncidentID, wo.Title, wo.Kind, wo.Priority, wo.Status, wo.Assignee, wo.Notes, wo.Checklist, wo.ScheduleCron, ts(wo.DueAt), ts(wo.SLADueAt), wo.CreatedAt.Format(time.RFC3339Nano), wo.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateWorkOrder(ctx context.Context, wo *model.WorkOrder) error {
	wo.UpdatedAt = time.Now().UTC()
	if wo.Checklist == "" {
		wo.Checklist = "[]"
	}
	_, err := s.exec(ctx, `UPDATE work_orders SET title=?, kind=?, priority=?, status=?, assignee=?, notes=?, checklist=?, schedule_cron=?, incident_id=?, due_at=?, sla_due_at=?, updated_at=? WHERE id=? AND organization_id=?`,
		wo.Title, wo.Kind, wo.Priority, wo.Status, wo.Assignee, wo.Notes, wo.Checklist, wo.ScheduleCron, wo.IncidentID, ts(wo.DueAt), ts(wo.SLADueAt), wo.UpdatedAt.Format(time.RFC3339Nano), wo.ID, wo.OrganizationID)
	return err
}

func (s *Store) GetWorkOrder(ctx context.Context, orgID, id string) (*model.WorkOrder, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,checklist,schedule_cron,due_at,sla_due_at,created_at,updated_at,last_fired_at FROM work_orders WHERE organization_id=? AND id=?`, orgID, id)
	return scanWO(row)
}

func scanWO(row scannable) (*model.WorkOrder, error) {
	var wo model.WorkOrder
	var asset, site, inc, due, sla, fired sql.NullString
	var created, updated string
	if err := row.Scan(&wo.ID, &wo.OrganizationID, &asset, &site, &inc, &wo.Title, &wo.Kind, &wo.Priority, &wo.Status, &wo.Assignee, &wo.Notes, &wo.Checklist, &wo.ScheduleCron, &due, &sla, &created, &updated, &fired); err != nil {
		return nil, err
	}
	wo.AssetID, wo.SiteID, wo.IncidentID = nullS(asset), nullS(site), nullS(inc)
	wo.DueAt = parseTimePtr(due)
	wo.SLADueAt = parseTimePtr(sla)
	wo.LastFiredAt = parseTimePtr(fired)
	wo.CreatedAt, wo.UpdatedAt = parseTime(created), parseTime(updated)
	if wo.Checklist == "" {
		wo.Checklist = "[]"
	}
	return &wo, nil
}

func (s *Store) ListWorkOrders(ctx context.Context, orgID, status string) ([]model.WorkOrder, error) {
	q := `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,checklist,schedule_cron,due_at,sla_due_at,created_at,updated_at,last_fired_at FROM work_orders WHERE organization_id=?`
	args := []any{orgID}
	if status != "" {
		q += ` AND status=?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WorkOrder
	for rows.Next() {
		wo, err := scanWO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *wo)
	}
	if out == nil {
		out = []model.WorkOrder{}
	}
	return out, rows.Err()
}

func (s *Store) CreateAPIKey(ctx context.Context, k *model.APIKey) error {
	if k.ID == "" {
		k.ID = idgen.New("key")
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	_, err := s.exec(ctx, `INSERT INTO api_keys(id,organization_id,user_id,name,token_hash,token_hint,created_at) VALUES(?,?,?,?,?,?,?)`,
		k.ID, k.OrganizationID, k.UserID, k.Name, k.TokenHash, k.TokenHint, k.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAPIKeysForUser(ctx context.Context, userID string) ([]model.APIKey, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,user_id,name,token_hint,created_at,last_used_at FROM api_keys WHERE user_id=? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.APIKey
	for rows.Next() {
		var k model.APIKey
		var created string
		var lastUsed sql.NullString
		if err := rows.Scan(&k.ID, &k.OrganizationID, &k.UserID, &k.Name, &k.TokenHint, &created, &lastUsed); err != nil {
			return nil, err
		}
		k.CreatedAt = parseTime(created)
		if lastUsed.Valid {
			t := parseTime(lastUsed.String)
			k.LastUsedAt = &t
		}
		out = append(out, k)
	}
	if out == nil {
		out = []model.APIKey{}
	}
	return out, rows.Err()
}

// UserByAPIKeyHash resolves the SHA-256 hash of a raw API key token to the
// user it belongs to — the same shape as ConnectorByTokenHash, but joined
// through to an active user rather than a connector.
func (s *Store) UserByAPIKeyHash(ctx context.Context, hash string) (*model.User, error) {
	var userID string
	err := s.queryRow(ctx, `SELECT user_id FROM api_keys WHERE token_hash=?`, hash).Scan(&userID)
	if err != nil {
		return nil, err
	}
	u, err := s.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.Active {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (s *Store) TouchAPIKeyHash(ctx context.Context, hash string) error {
	_, err := s.exec(ctx, `UPDATE api_keys SET last_used_at=? WHERE token_hash=?`, now(), hash)
	return err
}

func (s *Store) DeleteAPIKey(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	return checkAffected(res)
}

func (s *Store) CreateConnector(ctx context.Context, c *model.Connector) error {
	if c.ID == "" {
		c.ID = idgen.New("conn")
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = "pending"
	}
	if c.Actions == "" {
		c.Actions = "[]"
	}
	if c.Config == "" {
		c.Config = "{}"
	}
	_, err := s.exec(ctx, `INSERT INTO connectors(id,organization_id,name,kind,status,endpoint,token_hint,token_hash,actions,config,last_sync_at,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.OrganizationID, c.Name, c.Kind, c.Status, c.Endpoint, c.TokenHint, c.TokenHash, c.Actions, c.Config, ts(c.LastSyncAt), c.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListConnectors(ctx context.Context, orgID string) ([]model.Connector, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,actions,config,last_sync_at,last_error,last_latency_ms,sync_interval_sec,created_at FROM connectors WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Connector
	for rows.Next() {
		var c model.Connector
		var last sql.NullString
		var created string
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.Actions, &c.Config, &last, &c.LastError, &c.LastLatencyMs, &c.SyncIntervalSec, &created); err != nil {
			return nil, err
		}
		c.LastSyncAt = parseTimePtr(last)
		c.CreatedAt = parseTime(created)
		out = append(out, c)
	}
	if out == nil {
		out = []model.Connector{}
	}
	return out, rows.Err()
}

func (s *Store) ConnectorByTokenHash(ctx context.Context, hash string) (*model.Connector, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,token_hash,actions,config,last_sync_at,last_error,last_latency_ms,sync_interval_sec,created_at FROM connectors WHERE token_hash=?`, hash)
	var c model.Connector
	var last sql.NullString
	var created string
	if err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.TokenHash, &c.Actions, &c.Config, &last, &c.LastError, &c.LastLatencyMs, &c.SyncIntervalSec, &created); err != nil {
		return nil, err
	}
	c.LastSyncAt = parseTimePtr(last)
	c.CreatedAt = parseTime(created)
	return &c, nil
}

func (s *Store) TouchConnector(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `UPDATE connectors SET last_sync_at=?, status='connected', last_error='' WHERE id=?`, now(), id)
	return err
}

func (s *Store) RecordConnectorProbe(ctx context.Context, id, errText string, latencyMs int, synced bool) error {
	if synced {
		_, err := s.exec(ctx, `UPDATE connectors SET last_sync_at=?, status='connected', last_error='', last_latency_ms=? WHERE id=?`, now(), latencyMs, id)
		return err
	}
	if errText == "" {
		_, err := s.exec(ctx, `UPDATE connectors SET last_error='', last_latency_ms=? WHERE id=?`, latencyMs, id)
		return err
	}
	_, err := s.exec(ctx, `UPDATE connectors SET last_error=?, last_latency_ms=?, status='error', last_sync_at=? WHERE id=?`, errText, latencyMs, now(), id)
	return err
}

func (s *Store) ConnectorByID(ctx context.Context, orgID, id string) (*model.Connector, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,token_hash,actions,config,last_sync_at,last_error,last_latency_ms,sync_interval_sec,created_at FROM connectors WHERE organization_id=? AND id=?`, orgID, id)
	var c model.Connector
	var last sql.NullString
	var created string
	if err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.TokenHash, &c.Actions, &c.Config, &last, &c.LastError, &c.LastLatencyMs, &c.SyncIntervalSec, &created); err != nil {
		return nil, err
	}
	c.LastSyncAt = parseTimePtr(last)
	c.CreatedAt = parseTime(created)
	return &c, nil
}

func (s *Store) UpdateConnector(ctx context.Context, orgID string, c *model.Connector) error {
	_, err := s.exec(ctx, `UPDATE connectors SET name=?, status=?, endpoint=?, actions=?, config=?, sync_interval_sec=? WHERE organization_id=? AND id=?`,
		c.Name, c.Status, c.Endpoint, c.Actions, c.Config, c.SyncIntervalSec, orgID, c.ID)
	return err
}

func (s *Store) SetConnectorToken(ctx context.Context, orgID, id, tokenHash, tokenHint string) error {
	_, err := s.exec(ctx, `UPDATE connectors SET token_hash=?, token_hint=?, status='connected' WHERE organization_id=? AND id=?`,
		tokenHash, tokenHint, orgID, id)
	return err
}

func (s *Store) CreateAction(ctx context.Context, a *model.ActionRequest) error {
	if a.ID == "" {
		a.ID = idgen.New("act")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Status == "" {
		a.Status = "queued"
	}
	_, err := s.exec(ctx, `INSERT INTO action_requests(id,organization_id,asset_id,connector_id,action,idempotency_key,status,payload,result,expires_at,created_at,completed_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.OrganizationID, a.AssetID, a.ConnectorID, a.Action, a.IdempotencyKey, a.Status, a.Payload, a.Result, a.ExpiresAt.Format(time.RFC3339Nano), a.CreatedAt.Format(time.RFC3339Nano), ts(a.CompletedAt))
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return fmt.Errorf("duplicate action")
	}
	return err
}

func (s *Store) ListActions(ctx context.Context, orgID string) ([]model.ActionRequest, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,connector_id,action,idempotency_key,status,payload,result,expires_at,created_at,completed_at FROM action_requests WHERE organization_id=? ORDER BY created_at DESC LIMIT 100`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ActionRequest
	for rows.Next() {
		var a model.ActionRequest
		var asset, conn, done sql.NullString
		var exp, created string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &asset, &conn, &a.Action, &a.IdempotencyKey, &a.Status, &a.Payload, &a.Result, &exp, &created, &done); err != nil {
			return nil, err
		}
		a.AssetID, a.ConnectorID = nullS(asset), nullS(conn)
		a.ExpiresAt, a.CreatedAt, a.CompletedAt = parseTime(exp), parseTime(created), parseTimePtr(done)
		out = append(out, a)
	}
	if out == nil {
		out = []model.ActionRequest{}
	}
	return out, rows.Err()
}

func (s *Store) CompleteAction(ctx context.Context, id, status, result string) error {
	_, err := s.exec(ctx, `UPDATE action_requests SET status=?, result=?, completed_at=? WHERE id=?`, status, result, now(), id)
	return err
}

// ExpireActions marks queued/running actions past expires_at as expired.
func (s *Store) ExpireActions(ctx context.Context) (int, error) {
	res, err := s.exec(ctx, `UPDATE action_requests SET status='expired', result=?, completed_at=? WHERE status IN ('queued','running','accepted') AND expires_at<>'' AND expires_at<?`,
		"expired by sweeper", now(), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *Store) CreateAutomation(ctx context.Context, a *model.Automation) error {
	if a.ID == "" {
		a.ID = idgen.New("auto")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Config == "" {
		a.Config = "{}"
	}
	_, err := s.exec(ctx, `INSERT INTO automations(id,organization_id,name,enabled,trigger_kind,capability,operator,threshold,action,config,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.OrganizationID, a.Name, boolInt(a.Enabled), a.TriggerKind, a.Capability, a.Operator, a.Threshold, a.Action, a.Config, a.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAutomations(ctx context.Context, orgID string) ([]model.Automation, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,enabled,trigger_kind,capability,operator,threshold,action,COALESCE(config,'{}'),created_at FROM automations WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Automation
	for rows.Next() {
		var a model.Automation
		var en int
		var created string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &en, &a.TriggerKind, &a.Capability, &a.Operator, &a.Threshold, &a.Action, &a.Config, &created); err != nil {
			return nil, err
		}
		a.Enabled = en == 1
		a.CreatedAt = parseTime(created)
		out = append(out, a)
	}
	if out == nil {
		out = []model.Automation{}
	}
	return out, rows.Err()
}

func (s *Store) SetAutomationEnabled(ctx context.Context, orgID, id string, enabled bool) error {
	_, err := s.exec(ctx, `UPDATE automations SET enabled=? WHERE organization_id=? AND id=?`, boolInt(enabled), orgID, id)
	return err
}

func (s *Store) UpdateAutomation(ctx context.Context, a *model.Automation) error {
	if a.Config == "" {
		a.Config = "{}"
	}
	_, err := s.exec(ctx, `UPDATE automations SET name=?, enabled=?, trigger_kind=?, capability=?, operator=?, threshold=?, action=?, config=? WHERE organization_id=? AND id=?`,
		a.Name, boolInt(a.Enabled), a.TriggerKind, a.Capability, a.Operator, a.Threshold, a.Action, a.Config, a.OrganizationID, a.ID)
	return err
}

func (s *Store) DeleteAutomation(ctx context.Context, orgID, id string) error {
	_, err := s.exec(ctx, `DELETE FROM automations WHERE organization_id=? AND id=?`, orgID, id)
	return err
}

func (s *Store) UpdateSite(ctx context.Context, site *model.Site) error {
	_, err := s.exec(ctx, `UPDATE sites SET name=?, kind=?, address=?, latitude=?, longitude=?, timezone=? WHERE organization_id=? AND id=?`,
		site.Name, site.Kind, site.Address, site.Latitude, site.Longitude, site.Timezone, site.OrganizationID, site.ID)
	return err
}

func (s *Store) SiteByID(ctx context.Context, orgID, id string) (*model.Site, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,address,latitude,longitude,timezone,created_at FROM sites WHERE organization_id=? AND id=?`, orgID, id)
	var st model.Site
	var created string
	var lat, lng sql.NullFloat64
	if err := row.Scan(&st.ID, &st.OrganizationID, &st.Name, &st.Kind, &st.Address, &lat, &lng, &st.Timezone, &created); err != nil {
		return nil, err
	}
	st.Latitude, st.Longitude = nullF(lat), nullF(lng)
	st.CreatedAt = parseTime(created)
	return &st, nil
}

func (s *Store) DeleteSite(ctx context.Context, orgID, id string) error {
	_, err := s.exec(ctx, `DELETE FROM sites WHERE organization_id=? AND id=?`, orgID, id)
	return err
}

func (s *Store) DeleteAsset(ctx context.Context, orgID, id string) error {
	_, err := s.exec(ctx, `UPDATE assets SET deleted_at=?, updated_at=? WHERE organization_id=? AND id=? AND (deleted_at IS NULL OR deleted_at='')`,
		time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), orgID, id)
	return err
}

func (s *Store) ListOrgIDs(ctx context.Context) ([]string, error) {
	rows, err := s.query(ctx, `SELECT id FROM organizations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListEventsForAsset(ctx context.Context, orgID, assetID string, limit int) ([]model.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,site_id,kind,severity,title,body,dedupe_key,created_at FROM events WHERE organization_id=? AND asset_id=? ORDER BY created_at DESC LIMIT ?`, orgID, assetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var asset, site sql.NullString
		var created string
		if err := rows.Scan(&e.ID, &e.OrganizationID, &asset, &site, &e.Kind, &e.Severity, &e.Title, &e.Body, &e.DedupeKey, &created); err != nil {
			return nil, err
		}
		e.AssetID, e.SiteID = nullS(asset), nullS(site)
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	if out == nil {
		out = []model.Event{}
	}
	return out, rows.Err()
}

func (s *Store) ListWorkOrdersForAsset(ctx context.Context, orgID, assetID string) ([]model.WorkOrder, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,checklist,schedule_cron,due_at,sla_due_at,created_at,updated_at,last_fired_at FROM work_orders WHERE organization_id=? AND asset_id=? ORDER BY created_at DESC LIMIT 100`, orgID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WorkOrder
	for rows.Next() {
		wo, err := scanWO(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *wo)
	}
	if out == nil {
		out = []model.WorkOrder{}
	}
	return out, rows.Err()
}

func (s *Store) Audit(ctx context.Context, orgID, actor, action, object, detail string) error {
	_, err := s.exec(ctx, `INSERT INTO audit_log(id,organization_id,actor,action,object,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		idgen.New("aud"), orgID, actor, action, object, detail, now())
	return err
}

func (s *Store) ListAudit(ctx context.Context, orgID string, limit int) ([]model.AuditEntry, error) {
	if limit <= 0 {
		limit = 80
	}
	rows, err := s.query(ctx, `SELECT id,organization_id,actor,action,object,detail,created_at FROM audit_log WHERE organization_id=? ORDER BY created_at DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AuditEntry
	for rows.Next() {
		var a model.AuditEntry
		var created string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Actor, &a.Action, &a.Object, &a.Detail, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(created)
		out = append(out, a)
	}
	if out == nil {
		out = []model.AuditEntry{}
	}
	return out, rows.Err()
}

func (s *Store) Overview(ctx context.Context, orgID string) (*model.Overview, error) {
	ov := &model.Overview{HealthByKind: map[string]int{}}
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM assets WHERE organization_id=?`, orgID).Scan(&ov.AssetsTotal)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM assets WHERE organization_id=? AND health='healthy'`, orgID).Scan(&ov.AssetsHealthy)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM assets WHERE organization_id=? AND health='degraded'`, orgID).Scan(&ov.AssetsDegraded)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM assets WHERE organization_id=? AND health='critical'`, orgID).Scan(&ov.AssetsCritical)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM assets WHERE organization_id=? AND health='stale'`, orgID).Scan(&ov.AssetsStale)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE organization_id=? AND status IN ('open','ack')`, orgID).Scan(&ov.OpenIncidents)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM work_orders WHERE organization_id=? AND status NOT IN ('done','cancelled')`, orgID).Scan(&ov.OpenWorkOrders)
	_ = s.queryRow(ctx, `SELECT COUNT(*) FROM connectors WHERE organization_id=? AND status='connected'`, orgID).Scan(&ov.ActiveConnectors)
	rows, err := s.query(ctx, `SELECT kind, COUNT(*) FROM assets WHERE organization_id=? GROUP BY kind`, orgID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k string
			var n int
			if err := rows.Scan(&k, &n); err == nil {
				ov.HealthByKind[k] = n
			}
		}
	}
	ov.RecentEvents, _ = s.ListEvents(ctx, orgID, 8)
	ov.RecentActivity, _ = s.ListAudit(ctx, orgID, 8)
	return ov, nil
}

func (s *Store) CreateSeverityPolicy(ctx context.Context, p *model.SeverityPolicy) error {
	if p.ID == "" {
		p.ID = idgen.New("sev")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.MatchKind == "" {
		p.MatchKind = "default"
	}
	if p.Severity == "" {
		p.Severity = "warning"
	}
	_, err := s.exec(ctx, `INSERT INTO severity_policies(id,organization_id,name,match_kind,match_value,severity,runbook,priority,created_at)
VALUES(?,?,?,?,?,?,?,?,?)`, p.ID, p.OrganizationID, p.Name, p.MatchKind, p.MatchValue, p.Severity, p.Runbook, p.Priority, p.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateSeverityPolicy(ctx context.Context, p *model.SeverityPolicy) error {
	_, err := s.exec(ctx, `UPDATE severity_policies SET name=?, match_kind=?, match_value=?, severity=?, runbook=?, priority=? WHERE organization_id=? AND id=?`,
		p.Name, p.MatchKind, p.MatchValue, p.Severity, p.Runbook, p.Priority, p.OrganizationID, p.ID)
	return err
}

func (s *Store) DeleteSeverityPolicy(ctx context.Context, orgID, id string) error {
	_, err := s.exec(ctx, `DELETE FROM severity_policies WHERE organization_id=? AND id=?`, orgID, id)
	return err
}

func (s *Store) GetSeverityPolicy(ctx context.Context, orgID, id string) (*model.SeverityPolicy, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,match_kind,match_value,severity,runbook,priority,created_at FROM severity_policies WHERE organization_id=? AND id=?`, orgID, id)
	return scanSeverityPolicy(row)
}

func scanSeverityPolicy(row scannable) (*model.SeverityPolicy, error) {
	var p model.SeverityPolicy
	var created string
	if err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.MatchKind, &p.MatchValue, &p.Severity, &p.Runbook, &p.Priority, &created); err != nil {
		return nil, err
	}
	p.CreatedAt = parseTime(created)
	return &p, nil
}

func (s *Store) ListSeverityPolicies(ctx context.Context, orgID string) ([]model.SeverityPolicy, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,match_kind,match_value,severity,runbook,priority,created_at FROM severity_policies WHERE organization_id=? ORDER BY priority DESC, name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SeverityPolicy
	for rows.Next() {
		p, err := scanSeverityPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if out == nil {
		out = []model.SeverityPolicy{}
	}
	return out, rows.Err()
}

// ResolveSeverity picks the highest-priority matching policy for capability and/or automation name.
func (s *Store) ResolveSeverity(ctx context.Context, orgID, capability, automationName string) (severity, runbook string) {
	policies, err := s.ListSeverityPolicies(ctx, orgID)
	if err != nil || len(policies) == 0 {
		return "warning", ""
	}
	var fallback *model.SeverityPolicy
	for i := range policies {
		p := &policies[i]
		switch p.MatchKind {
		case "capability":
			if capability != "" && p.MatchValue == capability {
				return p.Severity, p.Runbook
			}
		case "automation":
			if automationName != "" && p.MatchValue == automationName {
				return p.Severity, p.Runbook
			}
		case "default":
			if fallback == nil {
				fallback = p
			}
		}
	}
	if fallback != nil {
		return fallback.Severity, fallback.Runbook
	}
	return "warning", ""
}

// CountSeverityPolicies returns how many policies exist for an org (for seed ensure).
func (s *Store) CountSeverityPolicies(ctx context.Context, orgID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM severity_policies WHERE organization_id=?`, orgID).Scan(&n)
	return n, err
}

// ImportAssetRow upserts one asset by external_ref when set, otherwise inserts a new row.
func (s *Store) ImportAssetRow(ctx context.Context, orgID string, in *model.Asset) (*model.Asset, string, error) {
	in.OrganizationID = orgID
	if in.Kind == "" {
		in.Kind = "equipment"
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Health == "" {
		in.Health = "unknown"
	}
	if in.StaleAfterSec <= 0 {
		in.StaleAfterSec = 90
	}
	if in.Metadata == "" {
		in.Metadata = "{}"
	}
	action := "created"
	if in.ExternalRef != "" {
		if existing, err := s.AssetByRef(ctx, orgID, in.ExternalRef); err == nil && existing != nil {
			in.ID = existing.ID
			in.CreatedAt = existing.CreatedAt
			if in.Health == "unknown" && existing.Health != "" {
				in.Health = existing.Health
			}
			action = "updated"
		}
	}
	if err := s.UpsertAsset(ctx, in); err != nil {
		return nil, "", err
	}
	return in, action, nil
}

// Job is a durable work item claimed by a single worker.
type Job struct {
	ID             string
	OrganizationID string
	Kind           string
	Status         string
	Attempts       int
	MaxAttempts    int
	Payload        string
	Result         string
	Error          string
	RunAfter       time.Time
	LockedBy       string
	LockedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

func (s *Store) CreateJob(ctx context.Context, j *Job) error {
	if j.ID == "" {
		j.ID = idgen.New("job")
	}
	now := time.Now().UTC()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	if j.Status == "" {
		j.Status = "queued"
	}
	if j.MaxAttempts <= 0 {
		j.MaxAttempts = 5
	}
	if j.Payload == "" {
		j.Payload = "{}"
	}
	if j.RunAfter.IsZero() {
		j.RunAfter = now
	}
	_, err := s.exec(ctx, `INSERT INTO jobs(id,organization_id,kind,status,attempts,max_attempts,payload,result,error,run_after,locked_by,locked_at,created_at,updated_at,completed_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, j.ID, j.OrganizationID, j.Kind, j.Status, j.Attempts, j.MaxAttempts, j.Payload, j.Result, j.Error,
		j.RunAfter.Format(time.RFC3339Nano), j.LockedBy, ts(j.LockedAt), j.CreatedAt.Format(time.RFC3339Nano), j.UpdatedAt.Format(time.RFC3339Nano), ts(j.CompletedAt))
	return err
}

func (s *Store) ClaimJob(ctx context.Context, worker string) (*Job, error) {
	now := time.Now().UTC()
	nowS := now.Format(time.RFC3339Nano)
	if s.Dialect == "postgres" {
		return s.claimJobPostgres(ctx, worker, nowS)
	}
	for try := 0; try < 5; try++ {
		row := s.queryRow(ctx, `SELECT id FROM jobs WHERE status='queued' AND run_after<=? ORDER BY created_at LIMIT 1`, nowS)
		var id string
		if err := row.Scan(&id); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		res, err := s.exec(ctx, `UPDATE jobs SET status='running', attempts=attempts+1, locked_by=?, locked_at=?, updated_at=? WHERE id=? AND status='queued'`,
			worker, nowS, nowS, id)
		if err != nil {
			return nil, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		return s.getJob(ctx, id)
	}
	return nil, nil
}

func (s *Store) claimJobPostgres(ctx context.Context, worker, nowS string) (*Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := s.rebind(`WITH picked AS (
  SELECT id FROM jobs WHERE status='queued' AND run_after<=? ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE jobs SET status='running', attempts=attempts+1, locked_by=?, locked_at=?, updated_at=?
WHERE id IN (SELECT id FROM picked)
RETURNING id`)
	var id string
	err = tx.QueryRowContext(ctx, q, nowS, worker, nowS, nowS).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.getJob(ctx, id)
}

func (s *Store) getJob(ctx context.Context, id string) (*Job, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,kind,status,attempts,max_attempts,payload,result,error,run_after,locked_by,locked_at,created_at,updated_at,completed_at FROM jobs WHERE id=?`, id)
	return scanJob(row)
}

func (s *Store) CompleteJob(ctx context.Context, id, status, result string) error {
	if status == "" {
		status = "succeeded"
	}
	_, err := s.exec(ctx, `UPDATE jobs SET status=?, result=?, error='', locked_by='', locked_at=NULL, completed_at=?, updated_at=? WHERE id=?`,
		status, result, now(), now(), id)
	return err
}

func (s *Store) FailJob(ctx context.Context, id, status, result, errText string, dead bool) error {
	st := "failed"
	if dead {
		st = "dead"
	}
	if status != "" {
		st = status
		if dead {
			st = "dead"
		}
	}
	_, err := s.exec(ctx, `UPDATE jobs SET status=?, result=?, error=?, locked_by='', locked_at=NULL, completed_at=?, updated_at=? WHERE id=?`,
		st, result, errText, now(), now(), id)
	return err
}

func (s *Store) RetryJob(ctx context.Context, id, result, errText string, runAfter time.Time) error {
	_, err := s.exec(ctx, `UPDATE jobs SET status='queued', result=?, error=?, run_after=?, locked_by='', locked_at=NULL, updated_at=? WHERE id=?`,
		result, errText, runAfter.Format(time.RFC3339Nano), now(), id)
	return err
}

func (s *Store) ListJobs(ctx context.Context, orgID string, limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.query(ctx, `SELECT id,organization_id,kind,status,attempts,max_attempts,payload,result,error,run_after,locked_by,locked_at,created_at,updated_at,completed_at FROM jobs WHERE organization_id=? ORDER BY created_at DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	if out == nil {
		out = []Job{}
	}
	return out, rows.Err()
}

func (s *Store) JobForOrg(ctx context.Context, orgID, id string) (*Job, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,kind,status,attempts,max_attempts,payload,result,error,run_after,locked_by,locked_at,created_at,updated_at,completed_at FROM jobs WHERE organization_id=? AND id=?`, orgID, id)
	return scanJob(row)
}

func scanJob(sc interface{ Scan(...any) error }) (*Job, error) {
	var j Job
	var runAfter, created, updated string
	var locked, completed sql.NullString
	if err := sc.Scan(&j.ID, &j.OrganizationID, &j.Kind, &j.Status, &j.Attempts, &j.MaxAttempts, &j.Payload, &j.Result, &j.Error, &runAfter, &j.LockedBy, &locked, &created, &updated, &completed); err != nil {
		return nil, err
	}
	j.RunAfter, j.CreatedAt, j.UpdatedAt = parseTime(runAfter), parseTime(created), parseTime(updated)
	j.LockedAt, j.CompletedAt = parseTimePtr(locked), parseTimePtr(completed)
	return &j, nil
}

// CancelQueuedJob marks a queued job cancelled. Running jobs are left alone.
func (s *Store) CancelQueuedJob(ctx context.Context, orgID, id string) error {
	res, err := s.exec(ctx, `UPDATE jobs SET status='cancelled', locked_by='', locked_at=NULL, completed_at=?, updated_at=? WHERE organization_id=? AND id=? AND status IN ('queued','pending_approval')`,
		now(), now(), orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RequeueDeadJob puts a dead or failed job back on the queue with attempts reset.
func (s *Store) RequeueDeadJob(ctx context.Context, orgID, id string, runAfter time.Time) error {
	res, err := s.exec(ctx, `UPDATE jobs SET status='queued', attempts=0, error='', run_after=?, locked_by='', locked_at=NULL, completed_at=NULL, updated_at=? WHERE organization_id=? AND id=? AND status IN ('dead','failed')`,
		runAfter.Format(time.RFC3339Nano), now(), orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetActionStatus(ctx context.Context, id, status, result string) error {
	_, err := s.exec(ctx, `UPDATE action_requests SET status=?, result=? WHERE id=?`, status, result, id)
	return err
}

type Location struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ParentID       *string   `json:"parent_id,omitempty"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Floorplan      string    `json:"floorplan,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreateLocation(ctx context.Context, loc *Location) error {
	if loc.ID == "" {
		loc.ID = idgen.New("loc")
	}
	if loc.CreatedAt.IsZero() {
		loc.CreatedAt = time.Now().UTC()
	}
	if loc.Kind == "" {
		loc.Kind = "zone"
	}
	_, err := s.exec(ctx, `INSERT INTO locations(id,organization_id,parent_id,name,kind,created_at) VALUES(?,?,?,?,?,?)`,
		loc.ID, loc.OrganizationID, loc.ParentID, loc.Name, loc.Kind, loc.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListLocations(ctx context.Context, orgID string) ([]Location, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,parent_id,name,kind,floorplan,created_at FROM locations WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var loc Location
		var parent sql.NullString
		var created string
		if err := rows.Scan(&loc.ID, &loc.OrganizationID, &parent, &loc.Name, &loc.Kind, &loc.Floorplan, &created); err != nil {
			return nil, err
		}
		loc.ParentID = nullS(parent)
		loc.CreatedAt = parseTime(created)
		out = append(out, loc)
	}
	if out == nil {
		out = []Location{}
	}
	return out, rows.Err()
}
