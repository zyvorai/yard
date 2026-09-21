package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/yard/internal/egress"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func TestEnterpriseSyncAndOAuth(t *testing.T) {
	st, err := store.Open("file:entsync?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, err := st.CreateOrganization(t.Context(), "Ops", "ops-sync")
	if err != nil {
		t.Fatal(err)
	}
	var sawSync, sawOAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			sawOAuth = true
			_, _ = w.Write([]byte(`{"access_token":"tok-1"}`))
			return
		}
		if r.Method == http.MethodGet {
			sawSync = true
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				http.Error(w, "auth", 401)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_ = body
		w.WriteHeader(204)
	}))
	defer srv.Close()
	cfg, _ := json.Marshal(map[string]string{
		"oauth_token_url": srv.URL, "oauth_client_id": "id", "oauth_client_secret": "secret",
	})
	conn := &model.Connector{OrganizationID: org.ID, Name: "snow", Kind: "servicenow", Endpoint: srv.URL, Config: string(cfg)}
	if err := st.CreateConnector(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	d := &Dispatcher{Store: st, Policy: egress.Demo(), Mode: "demo"}
	status, result, err := d.Execute(context.Background(), org.ID, conn, "sync", "")
	if err != nil || status != "completed" || result == "" || !sawSync || !sawOAuth {
		t.Fatalf("sync %s %s %v oauth=%v sync=%v", status, result, err, sawOAuth, sawSync)
	}
}
