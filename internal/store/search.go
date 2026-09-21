package store

import (
	"context"
	"strings"
)

type SearchHit struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (s *Store) Search(ctx context.Context, orgID, q string, limit int) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []SearchHit{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := s.query(ctx, `
SELECT kind, id, title FROM (
  SELECT 'asset' AS kind, id, name AS title FROM assets WHERE organization_id=? AND lower(name) LIKE ?
  UNION ALL
  SELECT 'incident', id, title FROM incidents WHERE organization_id=? AND lower(title) LIKE ?
  UNION ALL
  SELECT 'work_order', id, title FROM work_orders WHERE organization_id=? AND lower(title) LIKE ?
) t LIMIT ?`, orgID, like, orgID, like, orgID, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Kind, &h.ID, &h.Title); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if out == nil {
		out = []SearchHit{}
	}
	return out, rows.Err()
}
