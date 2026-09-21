package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestOIDCCallbackIssuesSession(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authorize" {
			q := r.URL.Query()
			http.Redirect(w, r, q.Get("redirect_uri")+"?code=abc&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
			return
		}
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"email": seed.DemoEmail})
	}))
	defer idp.Close()
	t.Setenv("YARD_OIDC_ISSUER", idp.URL)
	t.Setenv("YARD_OIDC_CLIENT_ID", "yard")
	t.Setenv("YARD_OIDC_CLIENT_SECRET", "secret")

	st, err := store.Open("file:oidc?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	resp, err := client.Get(ts.URL + "/api/v1/auth/oidc/start")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback %d %s", resp.StatusCode, b)
	}
	var out struct {
		Token string
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		t.Fatal("missing token")
	}
}
