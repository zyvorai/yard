package store

import (
	"fmt"
	"os"
)

// maybeEnableTimescale turns on optional TimescaleDB when YARD_TIMESCALE=1.
// Plain Postgres and SQLite keep the existing schema. With Timescale, Yard
// loads the extension, moves observation dedupe keys to a side table (unique
// indexes on hypertables must include the time column), converts observed_at
// and received_at to timestamptz, and creates a hypertable on observations.
func (s *Store) maybeEnableTimescale() error {
	if s == nil {
		return nil
	}
	want := os.Getenv("YARD_TIMESCALE") == "1"
	if s.Dialect != "postgres" {
		if want {
			return fmt.Errorf("YARD_TIMESCALE=1 requires a postgres:// database")
		}
		return nil
	}
	if !want {
		var n int
		if err := s.DB.QueryRow(`SELECT COUNT(*) FROM pg_extension WHERE extname='timescaledb'`).Scan(&n); err == nil && n > 0 {
			s.Timescale = true
		}
		return nil
	}
	if _, err := s.DB.Exec(`CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE`); err != nil {
		return fmt.Errorf("timescaledb extension: %w (use a TimescaleDB image with YARD_TIMESCALE=1)", err)
	}
	if err := s.ensureObservationsHypertable(); err != nil {
		return err
	}
	s.Timescale = true
	return nil
}

func (s *Store) ensureObservationsHypertable() error {
	var isHyper bool
	if err := s.DB.QueryRow(`
SELECT EXISTS (
  SELECT 1 FROM timescaledb_information.hypertables WHERE hypertable_name = 'observations'
)`).Scan(&isHyper); err != nil {
		return fmt.Errorf("timescale hypertables: %w", err)
	}
	if isHyper {
		return nil
	}

	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS observation_dedupe (
  organization_id TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  PRIMARY KEY (organization_id, dedupe_key)
)`); err != nil {
		return fmt.Errorf("observation_dedupe: %w", err)
	}
	if _, err := s.DB.Exec(`
INSERT INTO observation_dedupe(organization_id, dedupe_key)
SELECT organization_id, dedupe_key FROM observations
WHERE dedupe_key != ''
ON CONFLICT DO NOTHING`); err != nil {
		return fmt.Errorf("observation_dedupe seed: %w", err)
	}
	if _, err := s.DB.Exec(`DROP INDEX IF EXISTS observations_dedupe`); err != nil {
		return fmt.Errorf("drop observations_dedupe: %w", err)
	}

	if _, err := s.DB.Exec(`
ALTER TABLE observations
  ALTER COLUMN observed_at TYPE TIMESTAMPTZ USING observed_at::timestamptz,
  ALTER COLUMN received_at TYPE TIMESTAMPTZ USING received_at::timestamptz`); err != nil {
		return fmt.Errorf("observations timestamptz: %w", err)
	}
	if _, err := s.DB.Exec(`ALTER TABLE observations DROP CONSTRAINT IF EXISTS observations_pkey`); err != nil {
		return fmt.Errorf("drop observations_pkey: %w", err)
	}
	if _, err := s.DB.Exec(`ALTER TABLE observations ADD PRIMARY KEY (id, observed_at)`); err != nil {
		return fmt.Errorf("observations composite pk: %w", err)
	}
	if _, err := s.DB.Exec(`
SELECT create_hypertable(
  'observations',
  'observed_at',
  chunk_time_interval => INTERVAL '7 days',
  if_not_exists => TRUE,
  migrate_data => TRUE
)`); err != nil {
		return fmt.Errorf("create_hypertable(observations): %w", err)
	}
	return nil
}
