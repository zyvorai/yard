package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

// ErrNotLeader means another replica holds the scheduler lock.
var ErrNotLeader = errors.New("not leader")

// leaderLock is a stable Postgres advisory-lock key for schedule ticks.
const leaderLock int64 = 748001

// WithLeader runs fn while this process holds the scheduler lock.
// SQLite has one process, so fn always runs. On Postgres the lock is
// held on a single connection for the duration of fn and released after.
func (s *Store) WithLeader(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.Dialect != "postgres" {
		return fn(ctx)
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var ok bool
	if err := conn.QueryRowContext(ctx, s.rebind(`SELECT pg_try_advisory_lock(?)`), leaderLock).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotLeader
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), s.rebind(`SELECT pg_advisory_unlock(?)`), leaderLock)
	}()
	return fn(ctx)
}

func (s *Store) TooManyLogins(ctx context.Context, key string, limit int, since time.Time) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM login_attempts WHERE attempt_key=? AND created_at>=?`, key, since.UTC().Format(time.RFC3339Nano)).Scan(&n)
	if err != nil {
		return false, err
	}
	return n >= limit, nil
}

func (s *Store) RecordLoginFailure(ctx context.Context, key string, before time.Time) error {
	_, _ = s.exec(ctx, `DELETE FROM login_attempts WHERE created_at<?`, before.UTC().Format(time.RFC3339Nano))
	_, err := s.exec(ctx, `INSERT INTO login_attempts(id,attempt_key,created_at) VALUES(?,?,?)`, idgen.New("login"), key, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ClearLoginFailures(ctx context.Context, key string) error {
	_, err := s.exec(ctx, `DELETE FROM login_attempts WHERE attempt_key=?`, key)
	return err
}

func (s *Store) ApproveJob(ctx context.Context, orgID, id string, runAfter time.Time) error {
	res, err := s.exec(ctx, `UPDATE jobs SET status='queued', run_after=?, locked_by='', locked_at=NULL, updated_at=? WHERE organization_id=? AND id=? AND status='pending_approval'`,
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

func (s *Store) ConnectorsDueSync(ctx context.Context, nowTime time.Time) ([]model.Connector, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,kind,status,endpoint,token_hint,actions,config,last_sync_at,last_error,last_latency_ms,sync_interval_sec,created_at FROM connectors WHERE sync_interval_sec>0`)
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
		due := c.LastSyncAt == nil || !c.LastSyncAt.Add(time.Duration(c.SyncIntervalSec)*time.Second).After(nowTime)
		if due {
			out = append(out, c)
		}
	}
	if out == nil {
		out = []model.Connector{}
	}
	return out, rows.Err()
}

func (s *Store) OpenJobForConnector(ctx context.Context, orgID, connectorID string) (bool, error) {
	rows, err := s.query(ctx, `SELECT payload FROM jobs WHERE organization_id=? AND status IN ('queued','running','pending_approval')`, orgID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return false, err
		}
		if connectorID != "" && containsConnector(payload, connectorID) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func containsConnector(payload, connectorID string) bool {
	return strings.Contains(payload, `"connector_id":"`+connectorID+`"`)
}

// LiveEvent is one broadcast row other replicas can relay.
type LiveEvent struct {
	ID             string
	OrganizationID string
	Kind           string
	Payload        string
	Origin         string
	CreatedAt      time.Time
}

func (s *Store) InsertLiveEvent(ctx context.Context, orgID, kind, payload, origin string) (string, error) {
	id := idgen.New("evt")
	_, err := s.exec(ctx, `INSERT INTO live_events(id,organization_id,kind,payload,origin,created_at) VALUES(?,?,?,?,?,?)`,
		id, orgID, kind, payload, origin, time.Now().UTC().Format(time.RFC3339Nano))
	return id, err
}

func (s *Store) LiveEvent(ctx context.Context, id string) (*LiveEvent, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,kind,payload,origin,created_at FROM live_events WHERE id=?`, id)
	var ev LiveEvent
	var created string
	if err := row.Scan(&ev.ID, &ev.OrganizationID, &ev.Kind, &ev.Payload, &ev.Origin, &created); err != nil {
		return nil, err
	}
	ev.CreatedAt = parseTime(created)
	return &ev, nil
}

func (s *Store) NotifyLive(ctx context.Context, id string) {
	if s == nil || s.Dialect != "postgres" || id == "" {
		return
	}
	_, _ = s.exec(ctx, `SELECT pg_notify('yard_live', ?)`, id)
}

// ListenLive holds a Postgres connection on LISTEN yard_live and calls fn with each payload.
// SQLite returns immediately.
func (s *Store) ListenLive(ctx context.Context, fn func(string)) {
	if s == nil || s.Dialect != "postgres" || s.dsn == "" || fn == nil {
		return
	}
	go func() {
		for ctx.Err() == nil {
			conn, err := pgx.Connect(ctx, s.dsn)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				continue
			}
			if _, err := conn.Exec(ctx, `LISTEN yard_live`); err != nil {
				conn.Close(context.Background())
				continue
			}
			for ctx.Err() == nil {
				n, err := conn.WaitForNotification(ctx)
				if err != nil {
					break
				}
				if n != nil && n.Payload != "" {
					fn(n.Payload)
				}
			}
			conn.Close(context.Background())
		}
	}()
}
