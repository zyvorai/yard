package store

import (
	"context"
	"fmt"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

// postgresRollupMigration is the Postgres form of migration 16. The rollup
// table is range-partitioned by month on bucket_start. SQLite keeps the
// plain table from the shared migration list. Timescale is not required.
const postgresRollupMigration = `CREATE TABLE IF NOT EXISTS observation_rollups (
  organization_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  capability TEXT NOT NULL,
  bucket_start TEXT NOT NULL,
  samples INTEGER NOT NULL,
  value_min DOUBLE PRECISION NOT NULL,
  value_max DOUBLE PRECISION NOT NULL,
  value_sum DOUBLE PRECISION NOT NULL,
  unit TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (organization_id, asset_id, capability, bucket_start)
) PARTITION BY RANGE (bucket_start);`

func (s *Store) migrationSQL(m migration) string {
	if m.version == 16 && s.Dialect == "postgres" {
		return postgresRollupMigration
	}
	return m.sql
}

// EnsureRollupPartitions creates the Postgres month partition that holds
// bucket. SQLite has one table, so this is a no-op there.
func (s *Store) EnsureRollupPartitions(ctx context.Context, bucket time.Time) error {
	if s == nil || s.Dialect != "postgres" {
		return nil
	}
	start := time.Date(bucket.UTC().Year(), bucket.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	name := fmt.Sprintf("observation_rollups_%04d_%02d", start.Year(), int(start.Month()))
	q := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF observation_rollups FOR VALUES FROM ('%s') TO ('%s')`,
		name, start.Format(time.RFC3339), end.Format(time.RFC3339),
	)
	_, err := s.DB.ExecContext(ctx, q)
	return err
}

// DownsampleBefore folds numeric observations older than cutoff into hourly
// average buckets and deletes those raw rows. Bool and text readings stay raw.
func (s *Store) DownsampleBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	hourExpr := `substr(observed_at,1,13)`
	if s.Dialect == "postgres" {
		hourExpr = `substring(observed_at from 1 for 13)`
	}
	cutoffS := cutoff.UTC().Format(time.RFC3339Nano)
	q := fmt.Sprintf(`SELECT organization_id, asset_id, capability, %s, COUNT(*), MIN(value), MAX(value), SUM(value), MAX(unit)
FROM observations
WHERE value_kind='number' AND observed_at < ?
GROUP BY organization_id, asset_id, capability, %s`, hourExpr, hourExpr)
	rows, err := s.query(ctx, q, cutoffS)
	if err != nil {
		return 0, err
	}
	type bucket struct {
		org, asset, cap, hour, unit string
		samples                     int64
		min, max, sum               float64
	}
	var buckets []bucket
	for rows.Next() {
		var b bucket
		if err := rows.Scan(&b.org, &b.asset, &b.cap, &b.hour, &b.samples, &b.min, &b.max, &b.sum, &b.unit); err != nil {
			rows.Close()
			return 0, err
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(buckets) == 0 {
		return 0, nil
	}
	seenMonth := map[string]bool{}
	for _, b := range buckets {
		start, err := time.Parse(time.RFC3339, b.hour+":00:00Z")
		if err != nil {
			return 0, err
		}
		key := start.Format("2006-01")
		if seenMonth[key] {
			continue
		}
		seenMonth[key] = true
		if err := s.EnsureRollupPartitions(ctx, start); err != nil {
			return 0, err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	ins := s.rebind(`INSERT INTO observation_rollups(organization_id,asset_id,capability,bucket_start,samples,value_min,value_max,value_sum,unit)
VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(organization_id,asset_id,capability,bucket_start) DO UPDATE SET
  samples = observation_rollups.samples + excluded.samples,
  value_min = CASE WHEN excluded.value_min < observation_rollups.value_min THEN excluded.value_min ELSE observation_rollups.value_min END,
  value_max = CASE WHEN excluded.value_max > observation_rollups.value_max THEN excluded.value_max ELSE observation_rollups.value_max END,
  value_sum = observation_rollups.value_sum + excluded.value_sum,
  unit = excluded.unit`)
	for _, b := range buckets {
		if len(b.hour) != 13 {
			continue
		}
		if _, err := tx.ExecContext(ctx, ins, b.org, b.asset, b.cap, b.hour+":00:00Z", b.samples, b.min, b.max, b.sum, b.unit); err != nil {
			return 0, err
		}
	}
	del := s.rebind(`DELETE FROM observations WHERE value_kind='number' AND observed_at < ?`)
	res, err := tx.ExecContext(ctx, del, cutoffS)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) listRollups(ctx context.Context, orgID, assetID, cap string, from, to time.Time, limit int) ([]model.Observation, error) {
	q := `SELECT asset_id, capability, bucket_start, samples, value_sum, unit FROM observation_rollups WHERE organization_id=?`
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
		q += ` AND bucket_start >= ?`
		args = append(args, from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		q += ` AND bucket_start <= ?`
		args = append(args, to.UTC().Format(time.RFC3339Nano))
	}
	q += ` ORDER BY bucket_start DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Observation
	for rows.Next() {
		var o model.Observation
		var bucket string
		var samples int64
		var sum float64
		if err := rows.Scan(&o.AssetID, &o.Capability, &bucket, &samples, &sum, &o.Unit); err != nil {
			return nil, err
		}
		if samples <= 0 {
			continue
		}
		o.OrganizationID = orgID
		o.ID = "rollup_" + o.AssetID + "_" + o.Capability + "_" + bucket
		o.Value = sum / float64(samples)
		o.ValueKind = "number"
		o.Quality = "good"
		o.Source = "rollup"
		o.ObservedAt = parseTime(bucket)
		o.ReceivedAt = o.ObservedAt
		out = append(out, o)
	}
	return out, rows.Err()
}
