package secrets

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchVaultKey(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString(make([]byte, 32))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "vault-token" || r.URL.Path != "/v1/secret/data/yard" {
			http.Error(w, "no", 404)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"data":{"key":"` + raw + `"}}}`))
	}))
	defer srv.Close()
	got, err := FetchVaultKey(srv.URL, "vault-token", "")
	if err != nil || got != raw {
		t.Fatalf("%q %v", got, err)
	}
	key, err := ParseKey(got)
	if err != nil || len(key) != 32 {
		t.Fatal(err)
	}
}
