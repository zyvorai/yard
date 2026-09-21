package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) oidcStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	issuer := strings.TrimRight(strings.TrimSpace(os.Getenv("YARD_OIDC_ISSUER")), "/")
	clientID := strings.TrimSpace(os.Getenv("YARD_OIDC_CLIENT_ID"))
	if issuer == "" || clientID == "" {
		writeJSON(w, 404, map[string]string{"error": "oidc is not configured"})
		return
	}
	state := randomState()
	http.SetCookie(w, &http.Cookie{Name: "yard_oidc_state", Value: state, Path: "/api/v1/auth/oidc", HttpOnly: true, MaxAge: 300, SameSite: http.SameSiteLaxMode})
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid email")
	q.Set("state", state)
	q.Set("redirect_uri", s.oidcRedirect(r))
	http.Redirect(w, r, issuer+"/authorize?"+q.Encode(), http.StatusFound)
}

func (s *Server) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		writeJSON(w, 401, map[string]string{"error": errMsg})
		return
	}
	cookie, err := r.Cookie("yard_oidc_state")
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		writeJSON(w, 400, map[string]string{"error": "state mismatch"})
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeJSON(w, 400, map[string]string{"error": "code required"})
		return
	}
	email, err := s.exchangeOIDC(r.Context(), code, s.oidcRedirect(r))
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
	writeJSON(w, 200, map[string]any{"token": sess.Token, "user": u, "expires_at": sess.ExpiresAt})
}

func (s *Server) exchangeOIDC(ctx context.Context, code, redirect string) (string, error) {
	issuer := strings.TrimRight(strings.TrimSpace(os.Getenv("YARD_OIDC_ISSUER")), "/")
	clientID := strings.TrimSpace(os.Getenv("YARD_OIDC_CLIENT_ID"))
	secret := os.Getenv("YARD_OIDC_CLIENT_SECRET")
	if issuer == "" || clientID == "" || secret == "" {
		return "", errors.New("oidc is not configured")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", clientID)
	form.Set("client_secret", secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.DefaultClient
	if s.Runtime.Egress != nil {
		if err := s.Runtime.Egress.Validate(issuer + "/token"); err != nil {
			return "", err
		}
		client = s.Runtime.Egress.HTTPClient(8*time.Second, false)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tok struct {
		Email   string `json:"email"`
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || resp.StatusCode >= 300 {
		return "", errors.New("token exchange failed")
	}
	if tok.Email != "" {
		return strings.ToLower(tok.Email), nil
	}
	email := emailFromIDToken(tok.IDToken)
	if email == "" {
		return "", errors.New("email missing")
	}
	return email, nil
}

func emailFromIDToken(raw string) string {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return ""
	}
	buf, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(buf, &claims) != nil {
		return ""
	}
	return strings.ToLower(claims.Email)
}

func (s *Server) userForOIDC(ctx context.Context, email string) (*model.User, error) {
	u, err := s.Store.UserByEmail(ctx, email)
	if err == nil && u.Active {
		return u, nil
	}
	if s.Runtime.Mode == "production" || os.Getenv("YARD_OIDC_PROVISION") != "1" {
		return nil, errors.New("user is not provisioned")
	}
	orgs, err := s.Store.ListOrgIDs(ctx)
	if err != nil || len(orgs) != 1 {
		return nil, errors.New("user is not provisioned")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(randomState()), 10)
	if err != nil {
		return nil, err
	}
	created := &model.User{OrganizationID: orgs[0], Email: email, DisplayName: email, Role: "operator", PasswordHash: string(hash), Active: true}
	if err := s.Store.CreateUser(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Server) oidcRedirect(r *http.Request) string {
	if s.Runtime.PublicURL != "" {
		return strings.TrimRight(s.Runtime.PublicURL, "/") + "/api/v1/auth/oidc/callback"
	}
	return "http://" + r.Host + "/api/v1/auth/oidc/callback"
}

func randomState() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
