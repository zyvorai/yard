package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

const maxAttachment = 8 << 20

func (s *Server) assetAttachments(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset, parts []string) {
	switch {
	case len(parts) == 2 && r.Method == http.MethodGet:
		list, err := s.Store.ListAttachments(r.Context(), u.OrganizationID, a.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case len(parts) == 2 && r.Method == http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		s.createAttachment(w, r, u, a)
	case len(parts) == 3 && parts[2] != "" && r.Method == http.MethodGet:
		s.downloadAttachment(w, r, u, a, parts[2])
	case len(parts) == 3 && parts[2] != "" && r.Method == http.MethodDelete:
		if !s.requireWrite(w, u) {
			return
		}
		s.removeAttachment(w, r, u, a, parts[2])
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) createAttachment(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAttachment+512)
	if err := r.ParseMultipartForm(maxAttachment); err != nil {
		writeJSON(w, 400, map[string]string{"error": "file must be 8 MiB or smaller"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "file required"})
		return
	}
	defer file.Close()
	name := cleanFileName(header.Filename)
	if name == "" {
		writeJSON(w, 400, map[string]string{"error": "file name required"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(file, maxAttachment+1))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "read failed"})
		return
	}
	if len(body) == 0 || len(body) > maxAttachment {
		writeJSON(w, 400, map[string]string{"error": "file must be 8 MiB or smaller"})
		return
	}
	att := &store.Attachment{
		OrganizationID: u.OrganizationID,
		AssetID:        a.ID,
		Name:           name,
		ContentType:    cleanContentType(header.Header.Get("Content-Type")),
		SizeBytes:      int64(len(body)),
		CreatedBy:      u.Email,
	}
	if err := s.Store.CreateAttachment(r.Context(), att); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	path, err := attachmentPath(s.Runtime.DataDir, u.OrganizationID, att.ID)
	if err != nil {
		_ = s.Store.DeleteAttachment(r.Context(), u.OrganizationID, a.ID, att.ID)
		writeJSON(w, 500, map[string]string{"error": "storage"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		_ = s.Store.DeleteAttachment(r.Context(), u.OrganizationID, a.ID, att.ID)
		writeJSON(w, 500, map[string]string{"error": "storage"})
		return
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		_ = s.Store.DeleteAttachment(r.Context(), u.OrganizationID, a.ID, att.ID)
		writeJSON(w, 500, map[string]string{"error": "storage"})
		return
	}
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "attachment.create", att.ID, name)
	writeJSON(w, 201, att)
}

func (s *Server) downloadAttachment(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset, id string) {
	att, err := s.Store.AttachmentByID(r.Context(), u.OrganizationID, a.ID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	path, err := attachmentPath(s.Runtime.DataDir, u.OrganizationID, att.ID)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(att.Name, `"`, "")+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, att.Name, att.CreatedAt, f)
}

func (s *Server) removeAttachment(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset, id string) {
	att, err := s.Store.AttachmentByID(r.Context(), u.OrganizationID, a.ID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err := s.Store.DeleteAttachment(r.Context(), u.OrganizationID, a.ID, id); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if path, err := attachmentPath(s.Runtime.DataDir, u.OrganizationID, att.ID); err == nil {
		_ = os.Remove(path)
	}
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "attachment.delete", att.ID, att.Name)
	writeJSON(w, 200, map[string]string{"deleted": att.ID})
}

func attachmentPath(dataDir, orgID, id string) (string, error) {
	if !safeID(orgID) || !safeID(id) {
		return "", os.ErrInvalid
	}
	dir := filepath.Join(dataDir, "attachments", orgID)
	return filepath.Join(dir, id), nil
}

func safeID(id string) bool {
	if id == "" || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func cleanFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	if name == "." || name == ".." {
		return ""
	}
	if len(name) > 180 {
		name = name[:180]
	}
	return strings.TrimSpace(name)
}

func cleanContentType(v string) string {
	v = strings.TrimSpace(strings.Split(v, ";")[0])
	if v == "" || strings.ContainsAny(v, "\r\n") {
		return "application/octet-stream"
	}
	return v
}
