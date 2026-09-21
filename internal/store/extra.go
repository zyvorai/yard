package store

import (
	"context"

	"github.com/zyvorai/yard/internal/idgen"
)

type Tariff struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	StartHour      int    `json:"start_hour"`
	EndHour        int    `json:"end_hour"`
	CentsPerKWh    int    `json:"cents_per_kwh"`
}

type Preview struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	RequestedBy    string `json:"requested_by"`
	Title          string `json:"title"`
	AssetID        string `json:"asset_id,omitempty"`
	Status         string `json:"status"`
}

func (s *Store) ReplaceTariffs(ctx context.Context, orgID string, rows []Tariff) error {
	if _, err := s.exec(ctx, `DELETE FROM tariffs WHERE organization_id=?`, orgID); err != nil {
		return err
	}
	for i := range rows {
		row := &rows[i]
		row.OrganizationID = orgID
		if row.ID == "" {
			row.ID = idgen.New("tar")
		}
		if _, err := s.exec(ctx, `INSERT INTO tariffs(id,organization_id,start_hour,end_hour,cents_per_kwh) VALUES(?,?,?,?,?)`,
			row.ID, orgID, row.StartHour, row.EndHour, row.CentsPerKWh); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListTariffs(ctx context.Context, orgID string) ([]Tariff, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,start_hour,end_hour,cents_per_kwh FROM tariffs WHERE organization_id=? ORDER BY start_hour`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tariff
	for rows.Next() {
		var t Tariff
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.StartHour, &t.EndHour, &t.CentsPerKWh); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []Tariff{}
	}
	return out, rows.Err()
}

func (s *Store) SetCarbonGrams(ctx context.Context, orgID string, grams int) error {
	_, err := s.exec(ctx, `UPDATE organizations SET carbon_grams_per_kwh=? WHERE id=?`, grams, orgID)
	return err
}

func (s *Store) CarbonGrams(ctx context.Context, orgID string) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT carbon_grams_per_kwh FROM organizations WHERE id=?`, orgID).Scan(&n)
	return n, err
}

func (s *Store) CreatePreview(ctx context.Context, p *Preview) error {
	if p.ID == "" {
		p.ID = idgen.New("prv")
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	_, err := s.exec(ctx, `INSERT INTO assistant_previews(id,organization_id,requested_by,title,asset_id,status,created_at) VALUES(?,?,?,?,?,?,?)`,
		p.ID, p.OrganizationID, p.RequestedBy, p.Title, p.AssetID, p.Status, now())
	return err
}

func (s *Store) PreviewByID(ctx context.Context, orgID, id string) (*Preview, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,requested_by,title,asset_id,status FROM assistant_previews WHERE organization_id=? AND id=?`, orgID, id)
	var p Preview
	if err := row.Scan(&p.ID, &p.OrganizationID, &p.RequestedBy, &p.Title, &p.AssetID, &p.Status); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) SetPreviewStatus(ctx context.Context, orgID, id, status string) error {
	_, err := s.exec(ctx, `UPDATE assistant_previews SET status=? WHERE organization_id=? AND id=?`, status, orgID, id)
	return err
}
