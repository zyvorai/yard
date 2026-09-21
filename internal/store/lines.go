package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
)

type WorkOrderLine struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	WorkOrderID    string    `json:"work_order_id"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	Quantity       float64   `json:"quantity"`
	Unit           string    `json:"unit"`
	UnitCostCents  int64     `json:"unit_cost_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreateWorkOrderLine(ctx context.Context, line *WorkOrderLine) error {
	if line.ID == "" {
		line.ID = idgen.New("line")
	}
	if line.CreatedAt.IsZero() {
		line.CreatedAt = time.Now().UTC()
	}
	if line.Quantity <= 0 {
		line.Quantity = 1
	}
	_, err := s.exec(ctx, `INSERT INTO work_order_lines(id,organization_id,work_order_id,kind,name,quantity,unit,unit_cost_cents,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		line.ID, line.OrganizationID, line.WorkOrderID, line.Kind, line.Name, line.Quantity, line.Unit, line.UnitCostCents, line.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListWorkOrderLines(ctx context.Context, orgID, workOrderID string) ([]WorkOrderLine, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,work_order_id,kind,name,quantity,unit,unit_cost_cents,created_at FROM work_order_lines WHERE organization_id=? AND work_order_id=? ORDER BY created_at`, orgID, workOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkOrderLine
	for rows.Next() {
		line, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *line)
	}
	if out == nil {
		out = []WorkOrderLine{}
	}
	return out, rows.Err()
}

func scanLine(sc interface{ Scan(...any) error }) (*WorkOrderLine, error) {
	var line WorkOrderLine
	var created string
	if err := sc.Scan(&line.ID, &line.OrganizationID, &line.WorkOrderID, &line.Kind, &line.Name, &line.Quantity, &line.Unit, &line.UnitCostCents, &created); err != nil {
		return nil, err
	}
	line.CreatedAt = parseTime(created)
	return &line, nil
}

func (s *Store) DeleteWorkOrderLine(ctx context.Context, orgID, workOrderID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM work_order_lines WHERE organization_id=? AND work_order_id=? AND id=?`, orgID, workOrderID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
