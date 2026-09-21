package connectors

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/yard/internal/model"
)

func TestCatalogIncludesWebhook(t *testing.T) {
	var saw bool
	for _, k := range Catalog() {
		if k.Name == "webhook" {
			saw = true
		}
	}
	if !saw {
		t.Fatal("webhook kind missing")
	}
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		got = string(buf)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	d := NewDispatcher(nil)
	st, res, err := d.Execute(t.Context(), "org", &model.Connector{Kind: "webhook", Endpoint: srv.URL}, "post", `{"ok":true}`)
	if err != nil || st != "completed" {
		t.Fatalf("%s %s %v", st, res, err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("body %s", got)
	}
}
