package store

import (
	"context"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) SetAssetDesired(ctx context.Context, orgID, id, desired string, downtime, replacement *int) error {
	if desired == "" {
		desired = "{}"
	}
	q := `UPDATE assets SET desired_state=?, updated_at=?`
	args := []any{desired, now()}
	if downtime != nil {
		q += `, downtime_cents_per_hour=?`
		args = append(args, *downtime)
	}
	if replacement != nil {
		q += `, replacement_cost_cents=?`
		args = append(args, *replacement)
	}
	q += ` WHERE organization_id=? AND id=?`
	args = append(args, orgID, id)
	_, err := s.exec(ctx, q, args...)
	return err
}

func (s *Store) BlastRadius(ctx context.Context, orgID, assetID string, depth int) ([]model.Asset, error) {
	if depth <= 0 || depth > 3 {
		depth = 3
	}
	rows, err := s.query(ctx, `SELECT from_asset_id, to_asset_id FROM asset_links WHERE organization_id=?`, orgID)
	if err != nil {
		return nil, err
	}
	adj := map[string][]string{}
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			rows.Close()
			return nil, err
		}
		adj[from] = append(adj[from], to)
		adj[to] = append(adj[to], from)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	seen := map[string]bool{assetID: true}
	frontier := []string{assetID}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, id := range frontier {
			for _, nb := range adj[id] {
				if seen[nb] {
					continue
				}
				seen[nb] = true
				next = append(next, nb)
			}
		}
		frontier = next
	}
	var out []model.Asset
	for id := range seen {
		if id == assetID {
			continue
		}
		a, err := s.AssetByID(ctx, orgID, id)
		if err != nil || a == nil {
			continue
		}
		out = append(out, *a)
	}
	if out == nil {
		out = []model.Asset{}
	}
	return out, nil
}
