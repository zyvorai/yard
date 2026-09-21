package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestAssetLabelAndLookup(t *testing.T) {
	st, err := store.Open("file:label?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	admin, err := st.UserByEmail(context.Background(), seed.DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := st.ListAssets(context.Background(), admin.OrganizationID, "", "", "")
	if err != nil || len(assets) == 0 {
		t.Fatal(err)
	}
	sess, _ := st.CreateSession(context.Background(), admin.ID, 12*time.Hour)
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets/"+assets[0].ID+"/label", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("label %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var magic [4]byte
	if _, err := resp.Body.Read(magic[:]); err != nil || string(magic[:]) != "\x89PNG" {
		t.Fatalf("png magic %q %v", magic, err)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets/lookup?q=yard:asset:"+assets[0].ID, nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("lookup %d", resp.StatusCode)
	}
	if got := assetLabelPayload("https://yard.example", "ast_1"); got != "https://yard.example/assets?focus=ast_1" {
		t.Fatal(got)
	}
}
