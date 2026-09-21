package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) SetAssetPlacement(ctx context.Context, orgID, id string, parent, location, template *string) error {
	_, err := s.exec(ctx, `UPDATE assets SET parent_asset_id=?, location_id=?, template_id=?, updated_at=? WHERE organization_id=? AND id=?`,
		parent, location, template, now(), orgID, id)
	return err
}

type AssetTemplate struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Capabilities   string    `json:"capabilities"`
	StaleAfterSec  int       `json:"stale_after_sec"`
	Metadata       string    `json:"metadata"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreateAssetTemplate(ctx context.Context, t *AssetTemplate) error {
	if t.ID == "" {
		t.ID = idgen.New("tpl")
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.Kind == "" {
		t.Kind = "equipment"
	}
	if t.Capabilities == "" {
		t.Capabilities = "[]"
	}
	if t.Metadata == "" {
		t.Metadata = "{}"
	}
	if t.StaleAfterSec <= 0 {
		t.StaleAfterSec = 90
	}
	_, err := s.exec(ctx, `INSERT INTO asset_templates(id,organization_id,name,kind,capabilities,stale_after_sec,metadata,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		t.ID, t.OrganizationID, t.Name, t.Kind, t.Capabilities, t.StaleAfterSec, t.Metadata, t.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAssetTemplates(ctx context.Context, orgID string) ([]AssetTemplate, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,kind,capabilities,stale_after_sec,metadata,created_at FROM asset_templates WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetTemplate
	for rows.Next() {
		var t AssetTemplate
		var created string
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.Name, &t.Kind, &t.Capabilities, &t.StaleAfterSec, &t.Metadata, &created); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(created)
		out = append(out, t)
	}
	if out == nil {
		out = []AssetTemplate{}
	}
	return out, rows.Err()
}

type AssetLink struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	FromAssetID    string    `json:"from_asset_id"`
	ToAssetID      string    `json:"to_asset_id"`
	Relation       string    `json:"relation"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreateAssetLink(ctx context.Context, l *AssetLink) error {
	if l.ID == "" {
		l.ID = idgen.New("lnk")
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := s.exec(ctx, `INSERT INTO asset_links(id,organization_id,from_asset_id,to_asset_id,relation,created_at) VALUES(?,?,?,?,?,?)`,
		l.ID, l.OrganizationID, l.FromAssetID, l.ToAssetID, l.Relation, l.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAssetLinks(ctx context.Context, orgID string) ([]AssetLink, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,from_asset_id,to_asset_id,relation,created_at FROM asset_links WHERE organization_id=? ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetLink
	for rows.Next() {
		var l AssetLink
		var created string
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.FromAssetID, &l.ToAssetID, &l.Relation, &created); err != nil {
			return nil, err
		}
		l.CreatedAt = parseTime(created)
		out = append(out, l)
	}
	if out == nil {
		out = []AssetLink{}
	}
	return out, rows.Err()
}

func (s *Store) ListScheduledWorkOrders(ctx context.Context) ([]model.WorkOrder, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,site_id,incident_id,title,kind,priority,status,assignee,notes,checklist,schedule_cron,due_at,sla_due_at,created_at,updated_at,last_fired_at FROM work_orders WHERE schedule_cron<>''`)
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

func (s *Store) MarkWorkOrderFired(ctx context.Context, id string, at time.Time) error {
	_, err := s.exec(ctx, `UPDATE work_orders SET last_fired_at=?, updated_at=? WHERE id=?`, at.UTC().Format(time.RFC3339Nano), now(), id)
	return err
}

func (s *Store) LocationInOrg(ctx context.Context, orgID, id string) error {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM locations WHERE organization_id=? AND id=?`, orgID, id).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
