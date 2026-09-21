package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) ListDashboards(ctx context.Context, orgID string) ([]model.Dashboard, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,panels,created_at,updated_at FROM dashboards WHERE organization_id=? ORDER BY updated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Dashboard
	for rows.Next() {
		d, err := scanDashboard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []model.Dashboard{}
	}
	return out, rows.Err()
}

func (s *Store) CreateDashboard(ctx context.Context, d *model.Dashboard) error {
	if d.ID == "" {
		d.ID = idgen.New("dsh")
	}
	now := time.Now().UTC()
	d.CreatedAt, d.UpdatedAt = now, now
	if d.Panels == nil {
		d.Panels = []model.DashboardPanel{}
	}
	raw, err := json.Marshal(d.Panels)
	if err != nil {
		return err
	}
	_, err = s.exec(ctx, `INSERT INTO dashboards(id,organization_id,name,panels,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		d.ID, d.OrganizationID, d.Name, string(raw), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpdateDashboard(ctx context.Context, d *model.Dashboard) error {
	raw, err := json.Marshal(d.Panels)
	if err != nil {
		return err
	}
	d.UpdatedAt = time.Now().UTC()
	res, err := s.exec(ctx, `UPDATE dashboards SET name=?, panels=?, updated_at=? WHERE organization_id=? AND id=?`,
		d.Name, string(raw), d.UpdatedAt.Format(time.RFC3339Nano), d.OrganizationID, d.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteDashboard(ctx context.Context, orgID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM dashboards WHERE organization_id=? AND id=?`, orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type dashboardScanner interface {
	Scan(dest ...any) error
}

func scanDashboard(rows dashboardScanner) (model.Dashboard, error) {
	var d model.Dashboard
	var panels, created, updated string
	if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &panels, &created, &updated); err != nil {
		return d, err
	}
	if panels != "" {
		_ = json.Unmarshal([]byte(panels), &d.Panels)
	}
	if d.Panels == nil {
		d.Panels = []model.DashboardPanel{}
	}
	d.CreatedAt = parseTime(created)
	d.UpdatedAt = parseTime(updated)
	return d, nil
}
