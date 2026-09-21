package api

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zyvorai/yard/internal/ingest"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) ingestRemoteWrite(w http.ResponseWriter, r *http.Request, c *model.Connector) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "body"})
		return
	}
	series, err := ingest.DecodeRemoteWrite(body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid remote write"})
		return
	}
	s.writeIngest(w, r, c, ingest.ObservationsFromRemoteWrite(series))
}

func (s *Server) ingestOTLP(w http.ResponseWriter, r *http.Request, c *model.Connector) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "body"})
		return
	}
	batch, err := ingest.ObservationsFromOTLP(body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid otlp json"})
		return
	}
	s.writeIngest(w, r, c, batch)
}

func (s *Server) writeIngest(w http.ResponseWriter, r *http.Request, c *model.Connector, batch []model.IngestObservation) {
	accepted, skipped := 0, 0
	for _, in := range batch {
		obs, inserted, err := s.Engine.IngestObservation(r.Context(), c.OrganizationID, in, c.Kind)
		if err != nil || !inserted {
			skipped++
			continue
		}
		accepted++
		s.Hub.Publish(c.OrganizationID, "observation", obs)
	}
	s.metrics.inc(&s.metrics.ingestOK)
	writeJSON(w, 202, map[string]int{"accepted": accepted, "duplicate_or_unknown": skipped})
}

func (s *Server) dashboards(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListDashboards(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var in model.Dashboard
		if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.OrganizationID = u.OrganizationID
		if err := s.cleanPanels(r, u.OrganizationID, in.Panels); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.Store.CreateDashboard(r.Context(), &in); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "dashboard.create", in.ID, in.Name)
		writeJSON(w, 201, in)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) dashboardItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/dashboards/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		if !s.requireWrite(w, u) {
			return
		}
		var in model.Dashboard
		if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		in.ID = id
		in.Name = strings.TrimSpace(in.Name)
		in.OrganizationID = u.OrganizationID
		if err := s.cleanPanels(r, u.OrganizationID, in.Panels); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := s.Store.UpdateDashboard(r.Context(), &in); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, 404, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, in)
	case http.MethodDelete:
		if !s.requireWrite(w, u) {
			return
		}
		if err := s.Store.DeleteDashboard(r.Context(), u.OrganizationID, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, 404, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"deleted": id})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) cleanPanels(r *http.Request, orgID string, panels []model.DashboardPanel) error {
	if len(panels) > 12 {
		return fmt.Errorf("at most 12 panels")
	}
	for _, p := range panels {
		if p.AssetID == "" || p.Capability == "" {
			return fmt.Errorf("panel needs asset_id and capability")
		}
		a, err := s.Store.AssetByID(r.Context(), orgID, p.AssetID)
		if err != nil || a == nil {
			return fmt.Errorf("unknown asset")
		}
	}
	return nil
}
