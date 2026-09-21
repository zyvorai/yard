package api

import (
	"net/http"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/score"
)

func (s *Server) assetScore(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	incs, err := s.Store.ListIncidents(r.Context(), u.OrganizationID, "open")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	open := 0
	var evidenceIDs []string
	for _, inc := range incs {
		if inc.AssetID != nil && *inc.AssetID == a.ID {
			open++
			evidenceIDs = append(evidenceIDs, inc.ID)
		}
	}
	stale := a.Health == "stale"
	if a.LastSeenAt != nil && a.StaleAfterSec > 0 && time.Since(*a.LastSeenAt) > time.Duration(a.StaleAfterSec)*time.Second {
		stale = true
	}
	n, evidence := score.Asset(a.Health, open, stale)
	if evidenceIDs == nil {
		evidenceIDs = []string{}
	}
	writeJSON(w, 200, map[string]any{"asset_id": a.ID, "score": n, "evidence": evidence, "incident_ids": evidenceIDs})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	hits, err := s.Store.Search(r.Context(), u.OrganizationID, r.URL.Query().Get("q"), 20)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, hits)
}
