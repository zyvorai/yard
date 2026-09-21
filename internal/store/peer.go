package store

import (
	"context"
	"database/sql"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) UnreplicatedEvents(ctx context.Context, region string, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,site_id,kind,severity,title,body,dedupe_key,created_at,region FROM events WHERE replicated=0 AND region=? ORDER BY created_at LIMIT ?`, region, limit)
	if err != nil {
		return nil, err
	}
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var asset, site sql.NullString
		var created string
		if err := rows.Scan(&e.ID, &e.OrganizationID, &asset, &site, &e.Kind, &e.Severity, &e.Title, &e.Body, &e.DedupeKey, &created, &e.Region); err != nil {
			rows.Close()
			return nil, err
		}
		e.AssetID, e.SiteID = nullS(asset), nullS(site)
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	err = rows.Err()
	rows.Close()
	if out == nil {
		out = []model.Event{}
	}
	return out, err
}

func (s *Store) OrgExists(ctx context.Context, id string) (bool, error) {
	var got string
	err := s.queryRow(ctx, `SELECT id FROM organizations WHERE id=?`, id).Scan(&got)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) MarkEventsReplicated(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if _, err := s.exec(ctx, `UPDATE events SET replicated=1 WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) SetUserTOTP(ctx context.Context, userID, secret string) error {
	_, err := s.exec(ctx, `UPDATE users SET totp_secret=? WHERE id=?`, secret, userID)
	return err
}

func (s *Store) UserTOTP(ctx context.Context, userID string) (string, error) {
	var secret string
	err := s.queryRow(ctx, `SELECT totp_secret FROM users WHERE id=?`, userID).Scan(&secret)
	return secret, err
}
