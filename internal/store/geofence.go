package store

import (
	"context"
	"database/sql"

	"github.com/zyvorai/yard/internal/idgen"
)

type Geofence struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Name           string  `json:"name"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	RadiusM        float64 `json:"radius_m"`
}

func (s *Store) CreateGeofence(ctx context.Context, g *Geofence) error {
	if g.ID == "" {
		g.ID = idgen.New("geo")
	}
	_, err := s.exec(ctx, `INSERT INTO geofences(id,organization_id,name,latitude,longitude,radius_m,created_at) VALUES(?,?,?,?,?,?,?)`,
		g.ID, g.OrganizationID, g.Name, g.Latitude, g.Longitude, g.RadiusM, now())
	return err
}

func (s *Store) ListGeofences(ctx context.Context, orgID string) ([]Geofence, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,latitude,longitude,radius_m FROM geofences WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Geofence
	for rows.Next() {
		var g Geofence
		if err := rows.Scan(&g.ID, &g.OrganizationID, &g.Name, &g.Latitude, &g.Longitude, &g.RadiusM); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if out == nil {
		out = []Geofence{}
	}
	return out, rows.Err()
}

func (s *Store) GeofenceInside(ctx context.Context, orgID, assetID, geofenceID string) (bool, bool, error) {
	var inside int
	err := s.queryRow(ctx, `SELECT inside FROM geofence_state WHERE organization_id=? AND asset_id=? AND geofence_id=?`, orgID, assetID, geofenceID).Scan(&inside)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return inside == 1, true, nil
}

func (s *Store) SetGeofenceInside(ctx context.Context, orgID, assetID, geofenceID string, inside bool) error {
	n := 0
	if inside {
		n = 1
	}
	_, err := s.exec(ctx, `INSERT INTO geofence_state(organization_id,asset_id,geofence_id,inside) VALUES(?,?,?,?)
ON CONFLICT(organization_id, asset_id, geofence_id) DO UPDATE SET inside=excluded.inside`, orgID, assetID, geofenceID, n)
	return err
}

func (s *Store) SetAssetFloor(ctx context.Context, orgID, id string, x, y *float64) error {
	_, err := s.exec(ctx, `UPDATE assets SET floor_x=?, floor_y=?, updated_at=? WHERE organization_id=? AND id=?`, x, y, now(), orgID, id)
	return err
}

func (s *Store) SetLocationFloorplan(ctx context.Context, orgID, id, name string) error {
	res, err := s.exec(ctx, `UPDATE locations SET floorplan=? WHERE organization_id=? AND id=?`, name, orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) LocationFloorplan(ctx context.Context, orgID, id string) (string, error) {
	var name string
	err := s.queryRow(ctx, `SELECT floorplan FROM locations WHERE organization_id=? AND id=?`, orgID, id).Scan(&name)
	return name, err
}
