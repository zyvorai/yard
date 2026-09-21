package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) locationItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/locations/"), "/")
	id, extra, _ := strings.Cut(rest, "/")
	if err := s.Store.LocationInOrg(r.Context(), u.OrganizationID, id); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch extra {
	case "floorplan":
		s.locationFloorplan(w, r, u, id)
	case "pins":
		assets, err := s.Store.ListAssets(r.Context(), u.OrganizationID, "", "", "")
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		var pins []model.Asset
		for _, a := range assets {
			if a.LocationID != nil && *a.LocationID == id && (a.FloorX != nil || a.FloorY != nil) {
				pins = append(pins, a)
			}
		}
		if pins == nil {
			pins = []model.Asset{}
		}
		writeJSON(w, 200, pins)
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

func (s *Server) locationFloorplan(w http.ResponseWriter, r *http.Request, u *model.User, id string) {
	path, err := floorplanPath(s.Runtime.DataDir, u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid id"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		name, err := s.Store.LocationFloorplan(r.Context(), u.OrganizationID, id)
		if err != nil || name == "" {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		http.ServeFile(w, r, path)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		if err := r.ParseMultipartForm(maxAttachment); err != nil {
			writeJSON(w, 400, map[string]string{"error": "file required"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "file required"})
			return
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, maxAttachment+1))
		if err != nil || len(body) == 0 || len(body) > maxAttachment {
			writeJSON(w, 400, map[string]string{"error": "file must be 8 MiB or smaller"})
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if err := s.Store.SetLocationFloorplan(r.Context(), u.OrganizationID, id, header.Filename); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]string{"name": header.Filename})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func floorplanPath(dataDir, orgID, id string) (string, error) {
	if !safeID(orgID) || !safeID(id) {
		return "", os.ErrInvalid
	}
	return filepath.Join(dataDir, "floorplans", orgID, id), nil
}
