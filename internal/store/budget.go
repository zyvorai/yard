package store

import (
	"context"
	"time"
)

func (s *Store) TakeIngestBudget(ctx context.Context, orgID string, limit int, now time.Time) (bool, error) {
	if limit <= 0 {
		limit = 120
	}
	window := now.UTC().Truncate(time.Minute).Format(time.RFC3339)
	_, err := s.exec(ctx, `INSERT INTO ingest_budget(organization_id,window_start,hits) VALUES(?,?,1)
ON CONFLICT(organization_id, window_start) DO UPDATE SET hits = ingest_budget.hits + 1`, orgID, window)
	if err != nil {
		return false, err
	}
	var hits int
	if err := s.queryRow(ctx, `SELECT hits FROM ingest_budget WHERE organization_id=? AND window_start=?`, orgID, window).Scan(&hits); err != nil {
		return false, err
	}
	return hits <= limit, nil
}
