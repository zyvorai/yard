package jobs

import (
	"context"
	"math"

	"github.com/zyvorai/yard/internal/model"
)

func (e *Engine) noteGeofence(ctx context.Context, orgID string, asset *model.Asset, lat, lng float64) {
	list, err := e.Store.ListGeofences(ctx, orgID)
	if err != nil {
		return
	}
	for _, g := range list {
		nowIn := meters(lat, lng, g.Latitude, g.Longitude) <= g.RadiusM
		was, ok, err := e.Store.GeofenceInside(ctx, orgID, asset.ID, g.ID)
		if err != nil {
			continue
		}
		if ok && was == nowIn {
			continue
		}
		if !ok && !nowIn {
			continue
		}
		kind := "geofence.exit"
		if nowIn {
			kind = "geofence.enter"
		}
		_, _ = e.Store.InsertEvent(ctx, &model.Event{
			OrganizationID: orgID,
			AssetID:        &asset.ID,
			Kind:           kind,
			Severity:       "info",
			Title:          g.Name,
		})
		_ = e.Store.SetGeofenceInside(ctx, orgID, asset.ID, g.ID, nowIn)
	}
}

func meters(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371000.0
	p1 := lat1 * math.Pi / 180
	p2 := lat2 * math.Pi / 180
	dphi := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dphi/2)*math.Sin(dphi/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * earth * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
