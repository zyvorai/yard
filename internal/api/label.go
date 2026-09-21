package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/skip2/go-qrcode"
	"github.com/zyvorai/yard/internal/model"
)

func assetLabelPayload(publicURL string, assetID string) string {
	base := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if base == "" {
		return "yard:asset:" + assetID
	}
	return base + "/assets?focus=" + url.QueryEscape(assetID)
}

func labelCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "yard:asset:") {
		return strings.TrimPrefix(raw, "yard:asset:")
	}
	if i := strings.Index(raw, "focus="); i >= 0 {
		raw = raw[i+len("focus="):]
		if amp := strings.IndexByte(raw, '&'); amp >= 0 {
			raw = raw[:amp]
		}
		if decoded, err := url.QueryUnescape(raw); err == nil {
			return decoded
		}
	}
	return raw
}

func (s *Server) assetLabel(w http.ResponseWriter, a *model.Asset) {
	payload := assetLabelPayload(s.Runtime.PublicURL, a.ID)
	png, err := qrcode.Encode(payload, qrcode.Medium, 256)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "label"})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (s *Server) assetLookup(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	code := labelCode(r.URL.Query().Get("q"))
	if code == "" {
		writeJSON(w, 400, map[string]string{"error": "q required"})
		return
	}
	a, err := s.Store.AssetByID(r.Context(), u.OrganizationID, code)
	if err != nil {
		a, err = s.Store.AssetByRef(r.Context(), u.OrganizationID, code)
	}
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "asset not found"})
		return
	}
	writeJSON(w, 200, a)
}
