package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB      *sql.DB
	Dialect string // "sqlite" or "postgres"
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
	s := &Store{DB: db, Dialect: dialect}
	if err := s.migrate(); err != nil {
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

func (s *Store) migrate() error {
	_, err := s.DB.Exec(`
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
  threshold REAL NOT NULL DEFAULT 0, action TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_log (
  id TEXT PRIMARY KEY, organization_id TEXT NOT NULL, actor TEXT NOT NULL, action TEXT NOT NULL,
  object TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
`)
	return err
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
	_, err := s.exec(ctx, `INSERT INTO users(id,organization_id,email,display_name,role,password_hash,created_at) VALUES(?,?,?,?,?,?,?)`,
		u.ID, u.OrganizationID, strings.ToLower(u.Email), u.DisplayName, u.Role, u.PasswordHash, u.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,email,display_name,role,password_hash,created_at FROM users WHERE email=?`, strings.ToLower(email))
	return scanUser(row)
}

func (s *Store) UserByID(ctx context.Context, id string) (*model.User, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,email,display_name,role,password_hash,created_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*model.User, error) {
	var u model.User
	var created string
	if err := row.Scan(&u.ID, &u.OrganizationID, &u.Email, &u.DisplayName, &u.Role, &u.PasswordHash, &created); err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	return &u, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string, ttl time.Duration) (*model.Session, error) {
	sess := &model.Session{Token: idgen.Secret(24), UserID: userID, ExpiresAt: time.Now().UTC().Add(ttl)}
	_, err := s.exec(ctx, `INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)`, sess.Token, sess.UserID, sess.ExpiresAt.Format(time.RFC3339Nano))
	return sess, err
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
	return s.UserByID(ctx, userID)
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

func (s *Store) AssetByRef(ctx context.Context, orgID, ref string) (*model.Asset, error) {
	return s.getAsset(ctx, `SELECT id,organization_id,site_id,name,external_ref,kind,status,health,manufacturer,model,serial,latitude,longitude,last_seen_at,stale_after_sec,metadata,created_at,updated_at FROM assets WHERE organization_id=? AND external_ref=?`, orgID, ref)
}

func (s *Store) AssetByID(ctx context.Context, orgID, id string) (*model.Asset, error) {
	return s.getAsset(ctx, `SELECT id,organization_id,site_id,name,external_ref,kind,status,health,manufacturer,model,serial,latitude,longitude,last_seen_at,stale_after_sec,metadata,created_at,updated_at FROM assets WHERE organization_id=? AND id=?`, orgID, id)
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
	if err := row.Scan(&a.ID, &a.OrganizationID, &site, &a.Name, &a.ExternalRef, &a.Kind, &a.Status, &a.Health, &a.Manufacturer, &a.Model, &a.Serial, &lat, &lng, &lastN, &a.StaleAfterSec, &a.Metadata, &created, &updated); err != nil {
		return nil, err
	}
	a.SiteID = nullS(site)
	a.Latitude, a.Longitude = nullF(lat), nullF(lng)
	a.LastSeenAt = parseTimePtr(lastN)
	_ = last
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	return &a, nil
}

func (s *Store) ListAssets(ctx context.Context, orgID, q, kind, health string) ([]model.Asset, error) {
	query := `SELECT id,organization_id,site_id,name,external_ref,kind,status,health,manufacturer,model,serial,latitude,longitude,last_seen_at,stale_after_sec,metadata,created_at,updated_at FROM assets WHERE organization_id=?`
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
	_, err := s.exec(ctx, `INSERT INTO observations(id,organization_id,asset_id,capability,value,unit,quality,source,observed_at,received_at,dedupe_key)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`, o.ID, o.OrganizationID, o.AssetID, o.Capability, o.Value, o.Unit, o.Quality, o.Source, o.ObservedAt.Format(time.RFC3339Nano), o.ReceivedAt.Format(time.RFC3339Nano), o.DedupeKey)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) ListObservations(ctx context.Context, orgID, assetID, cap string, limit int) ([]model.Observation, error) {
	if limit <= 0 || limit > 2000 {
		limit = 400
	}
	q := `SELECT id,organization_id,asset_id,capability,value,unit,quality,source,observed_at,received_at,dedupe_key FROM observations WHERE organization_id=?`
	args := []any{orgID}
	if assetID != "" {
		q += ` AND asset_id=?`
		args = append(args, assetID)
	}
	if cap != "" {
		q += ` AND capability=?`
		args = append(args, cap)
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
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.AssetID, &o.Capability, &o.Value, &o.Unit, &o.Quality, &o.Source, &obs, &rec, &o.DedupeKey); err != nil {
			return nil, err
		}
		o.ObservedAt = parseTime(obs)
		o.ReceivedAt = parseTime(rec)
		out = append(out, o)
	}
	if out == nil {
		out = []model.Observation{}
	}
	return out, rows.Err()
}

func (s *Store) LatestTelemetry(ctx context.Context, orgID string) ([]model.TelemetryPoint, error) {
	rows, err := s.query(ctx, `
SELECT o.asset_id, a.name, o.capability, o.value, o.unit, o.quality, o.source, o.observed_at, o.received_at, a.stale_after_sec
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
		if err := rows.Scan(&p.AssetID, &p.AssetName, &p.Capability, &p.Value, &p.Unit, &p.Quality, &p.Source, &obs, &rec, &stale); err != nil {
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
	_, err := s.exec(ctx, `INSERT INTO events(id,organization_id,asset_id,site_id,kind,severity,title,body,dedupe_key,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`, e.ID, e.OrganizationID, e.AssetID, e.SiteID, e.Kind, e.Severity, e.Title, e.Body, e.DedupeKey, e.CreatedAt.Format(time.RFC3339Nano))
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
	_, err := s.exec(ctx, `INSERT INTO incidents(id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,opened_at,resolved_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, inc.ID, inc.OrganizationID, inc.AssetID, inc.SiteID, inc.Title, inc.Severity, inc.Status, inc.Owner, inc.Summary, inc.Resolution, inc.OpenedAt.Format(time.RFC3339Nano), ts(inc.ResolvedAt))
	return err
}

func (s *Store) UpdateIncident(ctx context.Context, inc *model.Incident) error {
	_, err := s.exec(ctx, `UPDATE incidents SET title=?, severity=?, status=?, owner=?, summary=?, resolution=?, resolved_at=? WHERE id=? AND organization_id=?`,
		inc.Title, inc.Severity, inc.Status, inc.Owner, inc.Summary, inc.Resolution, ts(inc.ResolvedAt), inc.ID, inc.OrganizationID)
	return err
}

func (s *Store) GetIncident(ctx context.Context, orgID, id string) (*model.Incident, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,opened_at,resolved_at FROM incidents WHERE organization_id=? AND id=?`, orgID, id)
	return scanIncident(row)
}

func scanIncident(row scannable) (*model.Incident, error) {
	var inc model.Incident
	var asset, site, resolved sql.NullString
	var opened string
	if err := row.Scan(&inc.ID, &inc.OrganizationID, &asset, &site, &inc.Title, &inc.Severity, &inc.Status, &inc.Owner, &inc.Summary, &inc.Resolution, &opened, &resolved); err != nil {
		return nil, err
	}
	inc.AssetID, inc.SiteID = nullS(asset), nullS(site)
	inc.OpenedAt = parseTime(opened)
	inc.ResolvedAt = parseTimePtr(resolved)
	return &inc, nil
}

func (s *Store) ListIncidents(ctx context.Context, orgID, status string) ([]model.Incident, error) {
	q := `SELECT id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,opened_at,resolved_at FROM incidents WHERE organization_id=?`
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
	q := `SELECT id,organization_id,asset_id,site_id,title,severity,status,owner,summary,resolution,opened_at,resolved_at FROM incidents WHERE organization_id=? AND title=? AND status IN ('open','ack')`
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
	_, err := s.exec(ctx, `INSERT INTO work_orders(id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,due_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, wo.ID, wo.OrganizationID, wo.AssetID, wo.SiteID, wo.IncidentID, wo.Title, wo.Kind, wo.Priority, wo.Status, wo.Assignee, wo.Notes, ts(wo.DueAt), wo.CreatedAt.Format(time.RFC3339Nano), wo.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateWorkOrder(ctx context.Context, wo *model.WorkOrder) error {
	wo.UpdatedAt = time.Now().UTC()
	_, err := s.exec(ctx, `UPDATE work_orders SET title=?, kind=?, priority=?, status=?, assignee=?, notes=?, incident_id=?, updated_at=? WHERE id=? AND organization_id=?`,
		wo.Title, wo.Kind, wo.Priority, wo.Status, wo.Assignee, wo.Notes, wo.IncidentID, wo.UpdatedAt.Format(time.RFC3339Nano), wo.ID, wo.OrganizationID)
	return err
}

func (s *Store) GetWorkOrder(ctx context.Context, orgID, id string) (*model.WorkOrder, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,due_at,created_at,updated_at FROM work_orders WHERE organization_id=? AND id=?`, orgID, id)
	return scanWO(row)
}

func scanWO(row scannable) (*model.WorkOrder, error) {
	var wo model.WorkOrder
	var asset, site, inc, due sql.NullString
	var created, updated string
	if err := row.Scan(&wo.ID, &wo.OrganizationID, &asset, &site, &inc, &wo.Title, &wo.Kind, &wo.Priority, &wo.Status, &wo.Assignee, &wo.Notes, &due, &created, &updated); err != nil {
		return nil, err
	}
	wo.AssetID, wo.SiteID, wo.IncidentID = nullS(asset), nullS(site), nullS(inc)
	wo.DueAt = parseTimePtr(due)
	wo.CreatedAt, wo.UpdatedAt = parseTime(created), parseTime(updated)
	return &wo, nil
}

func (s *Store) ListWorkOrders(ctx context.Context, orgID, status string) ([]model.WorkOrder, error) {
	q := `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,due_at,created_at,updated_at FROM work_orders WHERE organization_id=?`
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
	rows, err := s.query(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,actions,config,last_sync_at,created_at FROM connectors WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Connector
	for rows.Next() {
		var c model.Connector
		var last sql.NullString
		var created string
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.Actions, &c.Config, &last, &created); err != nil {
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
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,token_hash,actions,config,last_sync_at,created_at FROM connectors WHERE token_hash=?`, hash)
	var c model.Connector
	var last sql.NullString
	var created string
	if err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.TokenHash, &c.Actions, &c.Config, &last, &created); err != nil {
		return nil, err
	}
	c.LastSyncAt = parseTimePtr(last)
	c.CreatedAt = parseTime(created)
	return &c, nil
}

func (s *Store) TouchConnector(ctx context.Context, id string) error {
	_, err := s.exec(ctx, `UPDATE connectors SET last_sync_at=?, status='connected' WHERE id=?`, now(), id)
	return err
}

func (s *Store) ConnectorByID(ctx context.Context, orgID, id string) (*model.Connector, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,token_hash,actions,config,last_sync_at,created_at FROM connectors WHERE organization_id=? AND id=?`, orgID, id)
	var c model.Connector
	var last sql.NullString
	var created string
	if err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Kind, &c.Status, &c.Endpoint, &c.TokenHint, &c.TokenHash, &c.Actions, &c.Config, &last, &created); err != nil {
		return nil, err
	}
	c.LastSyncAt = parseTimePtr(last)
	c.CreatedAt = parseTime(created)
	return &c, nil
}

func (s *Store) UpdateConnector(ctx context.Context, orgID string, c *model.Connector) error {
	_, err := s.exec(ctx, `UPDATE connectors SET name=?, status=?, endpoint=?, actions=?, config=? WHERE organization_id=? AND id=?`,
		c.Name, c.Status, c.Endpoint, c.Actions, c.Config, orgID, c.ID)
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

func (s *Store) CreateAutomation(ctx context.Context, a *model.Automation) error {
	if a.ID == "" {
		a.ID = idgen.New("auto")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	_, err := s.exec(ctx, `INSERT INTO automations(id,organization_id,name,enabled,trigger_kind,capability,operator,threshold,action,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`, a.ID, a.OrganizationID, a.Name, boolInt(a.Enabled), a.TriggerKind, a.Capability, a.Operator, a.Threshold, a.Action, a.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAutomations(ctx context.Context, orgID string) ([]model.Automation, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,enabled,trigger_kind,capability,operator,threshold,action,created_at FROM automations WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Automation
	for rows.Next() {
		var a model.Automation
		var en int
		var created string
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Name, &en, &a.TriggerKind, &a.Capability, &a.Operator, &a.Threshold, &a.Action, &created); err != nil {
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
