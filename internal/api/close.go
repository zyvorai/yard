package api

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
	"github.com/zyvorai/yard/packs"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) searchAnswer(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	q := r.URL.Query().Get("q")
	hits, err := s.Store.Search(r.Context(), u.OrganizationID, q, 20)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	answer := answerFromHits(hits)
	if raw := strings.TrimSpace(os.Getenv("YARD_MODEL_URL")); raw != "" && s.Runtime.Egress != nil {
		if err := s.Runtime.Egress.Validate(raw); err == nil {
			body, _ := json.Marshal(map[string]string{"q": q})
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, raw, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				resp, err := s.Runtime.Egress.HTTPClient(8*time.Second, false).Do(req)
				if err == nil {
					defer resp.Body.Close()
					var out struct {
						Answer string `json:"answer"`
					}
					if json.NewDecoder(resp.Body).Decode(&out) == nil && out.Answer != "" {
						answer = out.Answer
					}
				}
			}
		}
	}
	writeJSON(w, 200, map[string]any{"answer": answer, "hits": hits})
}

func answerFromHits(hits []store.SearchHit) string {
	if len(hits) == 0 {
		return "No matching assets, incidents, or work orders."
	}
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		parts = append(parts, h.Title)
	}
	return "Found " + strings.Join(parts, ", ") + "."
}

func (s *Server) assistantPreview(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	var in struct {
		Title   string `json:"title"`
		AssetID string `json:"asset_id"`
	}
	if err := readJSON(r, &in); err != nil || in.Title == "" {
		writeJSON(w, 400, map[string]string{"error": "title required"})
		return
	}
	p := &store.Preview{OrganizationID: u.OrganizationID, RequestedBy: u.ID, Title: in.Title, AssetID: in.AssetID}
	if err := s.Store.CreatePreview(r.Context(), p); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, p)
}

func (s *Server) assistantApprove(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/assistant/preview/"), "/")
	id = strings.TrimSuffix(id, "/approve")
	id = strings.Trim(id, "/")
	p, err := s.Store.PreviewByID(r.Context(), u.OrganizationID, id)
	if err != nil || p.Status != "pending" {
		writeJSON(w, 404, map[string]string{"error": "preview not found"})
		return
	}
	if p.RequestedBy == u.ID {
		writeJSON(w, 403, map[string]string{"error": "a different operator must approve this action"})
		return
	}
	wo := &model.WorkOrder{OrganizationID: u.OrganizationID, Title: p.Title, Kind: "maintenance"}
	if p.AssetID != "" {
		wo.AssetID = &p.AssetID
	}
	if err := s.Store.CreateWorkOrder(r.Context(), wo); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Store.SetPreviewStatus(r.Context(), u.OrganizationID, p.ID, "approved")
	writeJSON(w, 201, wo)
}

func (s *Server) geofences(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListGeofences(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var g store.Geofence
		if err := readJSON(r, &g); err != nil || g.Name == "" || g.RadiusM <= 0 {
			writeJSON(w, 400, map[string]string{"error": "name and radius_m required"})
			return
		}
		g.OrganizationID = u.OrganizationID
		if err := s.Store.CreateGeofence(r.Context(), &g); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, g)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) marketplace(w http.ResponseWriter, r *http.Request, u *model.User) {
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/connectors/marketplace"), "/")
	if name == "" && r.Method == http.MethodGet {
		writeJSON(w, 200, packs.Connectors())
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	file, err := packs.Connector(name)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "unknown connector"})
		return
	}
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	_ = readJSON(r, &in)
	raw, _ := json.Marshal(file.Body)
	conn := &model.Connector{
		OrganizationID: u.OrganizationID,
		Name:           file.Name,
		Kind:           file.Kind,
		Status:         "pending",
		Endpoint:       in.Endpoint,
		Actions:        `["` + file.Action + `"]`,
		Config:         string(raw),
	}
	if err := s.Store.CreateConnector(r.Context(), conn); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, conn)
}

func (s *Server) roles(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireAdmin(w, u) {
		return
	}
	var role store.Role
	if err := readJSON(r, &role); err != nil || role.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name required"})
		return
	}
	role.OrganizationID = u.OrganizationID
	if err := s.Store.CreateRole(r.Context(), &role); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, role)
}

func (s *Server) shifts(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListShifts(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var sh store.Shift
		if err := readJSON(r, &sh); err != nil || sh.UserID == "" || sh.StartsAt == "" || sh.EndsAt == "" {
			writeJSON(w, 400, map[string]string{"error": "user_id, starts_at, and ends_at required"})
			return
		}
		sh.OrganizationID = u.OrganizationID
		if err := s.Store.CreateShift(r.Context(), &sh); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, sh)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) auditExport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireAdmin(w, u) {
		return
	}
	list, err := s.Store.ListAudit(r.Context(), u.OrganizationID, 500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	_, _ = io.WriteString(w, "created_at,actor,action,object,detail\n")
	for _, row := range list {
		_, _ = io.WriteString(w, row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")+",")
		_, _ = io.WriteString(w, csvCell(row.Actor)+","+csvCell(row.Action)+","+csvCell(row.Object)+","+csvCell(row.Detail)+"\n")
	}
}

func csvCell(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

var (
	samlEmail = regexp.MustCompile(`(?s)<AttributeValue>([^<]+)</AttributeValue>`)
	samlSig   = regexp.MustCompile(`(?s)<Signature>([^<]+)</Signature>`)
)

func (s *Server) samlACS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	cert := os.Getenv("YARD_SAML_CERT")
	if s.Runtime.Mode == "production" && cert == "" {
		writeJSON(w, 503, map[string]string{"error": "YARD_SAML_CERT is required"})
		return
	}
	email, err := samlEmailFrom(raw, cert)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	u, err := s.userForOIDC(r.Context(), email)
	if err != nil {
		writeJSON(w, 403, map[string]string{"error": err.Error()})
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), u.ID, 12*time.Hour)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "session"})
		return
	}
	writeJSON(w, 200, map[string]any{"token": sess.Token, "user": u})
}

func samlEmailFrom(raw []byte, certPEM string) (string, error) {
	m := samlEmail.FindSubmatch(raw)
	if m == nil {
		return "", errors.New("email missing")
	}
	email := strings.ToLower(strings.TrimSpace(string(m[1])))
	if certPEM == "" {
		return email, nil
	}
	sig := samlSig.FindSubmatch(raw)
	if sig == nil {
		return "", errors.New("signature missing")
	}
	sum := sha256.Sum256([]byte(email))
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig[1])))
	if err != nil {
		return "", errors.New("signature invalid")
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", errors.New("certificate invalid")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", errors.New("certificate invalid")
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("certificate invalid")
	}
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sigBytes) != nil {
		return "", errors.New("signature invalid")
	}
	return email, nil
}

func (s *Server) scimUsers(w http.ResponseWriter, r *http.Request) {
	if subtleToken(r) != os.Getenv("YARD_SCIM_TOKEN") || os.Getenv("YARD_SCIM_TOKEN") == "" {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/scim/v2/Users"), "/")
	if rest == "" && r.Method == http.MethodPost {
		var in struct {
			UserName string `json:"userName"`
			Active   *bool  `json:"active"`
		}
		if err := readJSON(r, &in); err != nil || in.UserName == "" {
			writeJSON(w, 400, map[string]string{"error": "userName required"})
			return
		}
		email := strings.ToLower(in.UserName)
		orgs, err := s.Store.ListOrgIDs(r.Context())
		if err != nil || len(orgs) != 1 {
			writeJSON(w, 409, map[string]string{"error": "organization is not singular"})
			return
		}
		if s.Runtime.Mode == "production" {
			if u, err := s.Store.UserByEmail(r.Context(), email); err == nil {
				writeJSON(w, 200, map[string]any{"id": u.ID, "userName": u.Email, "active": u.Active})
				return
			}
			writeJSON(w, 404, map[string]string{"error": "user is not provisioned"})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(randomState()), 4)
		u := &model.User{OrganizationID: orgs[0], Email: email, DisplayName: email, Role: "operator", PasswordHash: string(hash), Active: true}
		if err := s.Store.CreateUser(r.Context(), u); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"id": u.ID, "userName": u.Email, "active": true})
		return
	}
	if rest != "" && r.Method == http.MethodPatch {
		var in struct {
			Active *bool `json:"active"`
		}
		_ = readJSON(r, &in)
		if in.Active != nil && !*in.Active {
			orgs, _ := s.Store.ListOrgIDs(r.Context())
			if len(orgs) == 1 {
				_ = s.Store.SetUserActive(r.Context(), orgs[0], rest, false)
			}
		}
		writeJSON(w, 200, map[string]any{"id": rest, "active": false})
		return
	}
	writeJSON(w, 404, map[string]string{"error": "not found"})
}

func subtleToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	return strings.TrimPrefix(h, "Bearer ")
}

func (s *Server) webauthnRegisterStart(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	ch, err := s.Store.IssueWebAuthnChallenge(r.Context(), u.ID, "register")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"challenge": ch})
}

func (s *Server) webauthnRegisterFinish(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		Challenge    string `json:"challenge"`
		CredentialID string `json:"credential_id"`
		PublicKey    string `json:"public_key"`
	}
	if err := readJSON(r, &in); err != nil || in.CredentialID == "" {
		writeJSON(w, 400, map[string]string{"error": "credential_id required"})
		return
	}
	ok, err := s.Store.ConsumeWebAuthnChallenge(r.Context(), u.ID, in.Challenge, "register")
	if err != nil || !ok {
		writeJSON(w, 400, map[string]string{"error": "challenge mismatch"})
		return
	}
	if err := s.Store.SaveWebAuthnCredential(r.Context(), u.OrganizationID, u.ID, in.CredentialID, in.PublicKey); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]string{"credential_id": in.CredentialID})
}

func (s *Server) webauthnLoginStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if err := readJSON(r, &in); err != nil || in.Email == "" {
		writeJSON(w, 400, map[string]string{"error": "email required"})
		return
	}
	u, err := s.Store.UserByEmail(r.Context(), strings.ToLower(in.Email))
	if err != nil || !u.Active {
		writeJSON(w, 404, map[string]string{"error": "user not found"})
		return
	}
	ch, err := s.Store.IssueWebAuthnChallenge(r.Context(), u.ID, "login")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"challenge": ch})
}

func (s *Server) webauthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		Email        string `json:"email"`
		Challenge    string `json:"challenge"`
		CredentialID string `json:"credential_id"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, err := s.Store.UserByEmail(r.Context(), strings.ToLower(in.Email))
	if err != nil || !u.Active {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	ok, err := s.Store.ConsumeWebAuthnChallenge(r.Context(), u.ID, in.Challenge, "login")
	if err != nil || !ok {
		writeJSON(w, 400, map[string]string{"error": "challenge mismatch"})
		return
	}
	found, err := s.Store.WebAuthnCredential(r.Context(), u.ID, in.CredentialID)
	if err != nil || !found {
		writeJSON(w, 401, map[string]string{"error": "unknown credential"})
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), u.ID, 12*time.Hour)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "session"})
		return
	}
	writeJSON(w, 200, map[string]any{"token": sess.Token})
}
