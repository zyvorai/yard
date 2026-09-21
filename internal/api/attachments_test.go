package api

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestAssetAttachmentRoundTrip(t *testing.T) {
	st, err := store.Open("file:files?mode=memory&cache=shared")
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
	srv.Runtime.DataDir = t.TempDir()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "manual.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("pump manual"))
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/assets/"+assets[0].ID+"/attachments", &body)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create %d %s", resp.StatusCode, b)
	}
	listReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets/"+assets[0].ID+"/attachments", nil)
	listReq.Header.Set("Authorization", "Bearer "+sess.Token)
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	raw, _ := io.ReadAll(listResp.Body)
	if listResp.StatusCode != 200 || !bytes.Contains(raw, []byte("manual.txt")) {
		t.Fatalf("list %d %s", listResp.StatusCode, raw)
	}
}
