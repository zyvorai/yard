package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
)

type Permit struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	WorkOrderID    string    `json:"work_order_id"`
	Status         string    `json:"status"`
	ApprovedBy     string    `json:"approved_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreatePermit(ctx context.Context, p *Permit) error {
	if p.ID == "" {
		p.ID = idgen.New("prm")
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	p.CreatedAt = time.Now().UTC()
	_, err := s.exec(ctx, `INSERT INTO permits(id,organization_id,work_order_id,status,approved_by,created_at) VALUES(?,?,?,?,?,?)`,
		p.ID, p.OrganizationID, p.WorkOrderID, p.Status, p.ApprovedBy, p.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListPermits(ctx context.Context, orgID, workOrderID string) ([]Permit, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,work_order_id,status,approved_by,created_at FROM permits WHERE organization_id=? AND work_order_id=? ORDER BY created_at`, orgID, workOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Permit
	for rows.Next() {
		var p Permit
		var created string
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.WorkOrderID, &p.Status, &p.ApprovedBy, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = parseTime(created)
		out = append(out, p)
	}
	if out == nil {
		out = []Permit{}
	}
	return out, rows.Err()
}

func (s *Store) ApprovePermit(ctx context.Context, orgID, id, by string) error {
	res, err := s.exec(ctx, `UPDATE permits SET status='approved', approved_by=? WHERE organization_id=? AND id=? AND status='pending'`, by, orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) HasApprovedPermit(ctx context.Context, orgID, workOrderID string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM permits WHERE organization_id=? AND work_order_id=? AND status='approved'`, orgID, workOrderID).Scan(&n)
	return n > 0, err
}
