package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) AutomationHold(ctx context.Context, orgID, automationID, assetID string) (time.Time, bool, error) {
	var since string
	err := s.queryRow(ctx, `SELECT since FROM automation_holds WHERE organization_id=? AND automation_id=? AND asset_id=?`, orgID, automationID, assetID).Scan(&since)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return parseTime(since), true, nil
}

func (s *Store) SetAutomationHold(ctx context.Context, orgID, automationID, assetID string, since time.Time) error {
	_, err := s.exec(ctx, `INSERT INTO automation_holds(organization_id,automation_id,asset_id,since) VALUES(?,?,?,?)
ON CONFLICT(organization_id,automation_id,asset_id) DO UPDATE SET since=excluded.since`,
		orgID, automationID, assetID, since.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ClearAutomationHold(ctx context.Context, orgID, automationID, assetID string) error {
	_, err := s.exec(ctx, `DELETE FROM automation_holds WHERE organization_id=? AND automation_id=? AND asset_id=?`, orgID, automationID, assetID)
	return err
}
