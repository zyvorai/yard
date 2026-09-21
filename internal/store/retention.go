package store

import (
	"context"
	"time"
)

func (s *Store) RetentionDays(ctx context.Context, orgID string) (int, error) {
	var days int
	err := s.queryRow(ctx, `SELECT retention_days FROM organizations WHERE id=?`, orgID).Scan(&days)
	return days, err
}

func (s *Store) SetRetentionDays(ctx context.Context, orgID string, days int) error {
	_, err := s.exec(ctx, `UPDATE organizations SET retention_days=? WHERE id=?`, days, orgID)
	return err
}

func (s *Store) EnergyCentsPerKWh(ctx context.Context, orgID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT energy_cents_per_kwh FROM organizations WHERE id=?`, orgID).Scan(&n)
	return n, err
}

func (s *Store) SetEnergyCentsPerKWh(ctx context.Context, orgID string, cents int) error {
	_, err := s.exec(ctx, `UPDATE organizations SET energy_cents_per_kwh=? WHERE id=?`, cents, orgID)
	return err
}

func (s *Store) PurgeObservationsBefore(ctx context.Context, orgID string, before time.Time) (int64, error) {
	res, err := s.exec(ctx, `DELETE FROM observations WHERE organization_id=? AND observed_at<?`, orgID, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := s.exec(ctx, `DELETE FROM observation_rollups WHERE organization_id=? AND bucket_start<?`, orgID, before.UTC().Format(time.RFC3339Nano)); err != nil {
		return n, err
	}
	return n, nil
}
