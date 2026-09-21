package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) assetTwin(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	points, err := s.Store.LatestTelemetry(r.Context(), u.OrganizationID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var mine []model.TelemetryPoint
	for _, p := range points {
		if p.AssetID == a.ID {
			mine = append(mine, p)
		}
	}
	if mine == nil {
		mine = []model.TelemetryPoint{}
	}
	writeJSON(w, 200, map[string]any{
		"asset_id": a.ID,
		"health":   a.Health,
		"desired":  a.DesiredState,
		"reported": mine,
	})
}

func (s *Server) assetBlast(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	list, err := s.Store.BlastRadius(r.Context(), u.OrganizationID, a.ID, 3)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func readAssetPatch(r *http.Request) (model.Asset, *string, *int, *int, *float64, *float64, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return model.Asset{}, nil, nil, nil, nil, nil, err
	}
	var in model.Asset
	if err := json.Unmarshal(raw, &in); err != nil {
		return model.Asset{}, nil, nil, nil, nil, nil, err
	}
	var extra struct {
		DesiredState         *string  `json:"desired_state"`
		DowntimeCentsPerHour *int     `json:"downtime_cents_per_hour"`
		ReplacementCostCents *int     `json:"replacement_cost_cents"`
		FloorX               *float64 `json:"floor_x"`
		FloorY               *float64 `json:"floor_y"`
	}
	_ = json.Unmarshal(raw, &extra)
	return in, extra.DesiredState, extra.DowntimeCentsPerHour, extra.ReplacementCostCents, extra.FloorX, extra.FloorY, nil
}
