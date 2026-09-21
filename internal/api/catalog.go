package api

import (
	"net/http"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func (s *Server) assetTemplates(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListAssetTemplates(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var in store.AssetTemplate
		if err := readJSON(r, &in); err != nil || in.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		in.OrganizationID = u.OrganizationID
		if err := s.Store.CreateAssetTemplate(r.Context(), &in); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "template.create", in.ID, in.Name)
		writeJSON(w, 201, in)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) assetLinks(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListAssetLinks(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var in store.AssetLink
		if err := readJSON(r, &in); err != nil || in.FromAssetID == "" || in.ToAssetID == "" || in.Relation == "" {
			writeJSON(w, 400, map[string]string{"error": "from_asset_id, to_asset_id, and relation required"})
			return
		}
		if _, err := s.Store.AssetByID(r.Context(), u.OrganizationID, in.FromAssetID); err != nil {
			writeJSON(w, 404, map[string]string{"error": "from asset not found"})
			return
		}
		if _, err := s.Store.AssetByID(r.Context(), u.OrganizationID, in.ToAssetID); err != nil {
			writeJSON(w, 404, map[string]string{"error": "to asset not found"})
			return
		}
		in.OrganizationID = u.OrganizationID
		if err := s.Store.CreateAssetLink(r.Context(), &in); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.link", in.ID, in.Relation)
		writeJSON(w, 201, in)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}
