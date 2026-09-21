package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
	"github.com/zyvorai/yard/packs"
	"golang.org/x/crypto/bcrypt"
)

func TestAnswerPreviewPacksAndIdentity(t *testing.T) {
	st, err := store.Open("file:close?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(t.Context(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("operator-pass"), 4)
	other := &model.User{OrganizationID: res.User.OrganizationID, Email: "op2@yard.local", DisplayName: "Op", Role: "operator", PasswordHash: string(hash), Active: true}
	if err := st.CreateUser(t.Context(), other); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	admin := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	op := loginToken(t, ts.URL, "op2@yard.local", "operator-pass")

	hits := getJSON[map[string]any](t, ts.URL, "/api/v1/search/answer?q=pump", admin)
	if hits["answer"] == "" {
		t.Fatalf("answer %#v", hits)
	}
	prev := postJSON[store.Preview](t, ts.URL, "/api/v1/assistant/preview", admin, map[string]string{"title": "Replace bearing"}, 201)
	deny := authed(t, http.MethodPost, ts.URL+"/api/v1/assistant/preview/"+prev.ID+"/approve", admin, nil)
	if deny.StatusCode != 403 {
		t.Fatalf("requester approve %d", deny.StatusCode)
	}
	deny.Body.Close()
	postJSON[model.WorkOrder](t, ts.URL, "/api/v1/assistant/preview/"+prev.ID+"/approve", op, nil, 201)

	for _, name := range packs.Names {
		out := postJSON[map[string]any](t, ts.URL, "/api/v1/packs/"+name, admin, nil, 201)
		if out["work_order_id"] == "" {
			t.Fatalf("%s %#v", name, out)
		}
	}
	autos, err := st.ListAutomations(t.Context(), res.User.OrganizationID)
	if err != nil || len(autos) < 13 {
		t.Fatalf("automations %d %v", len(autos), err)
	}

	market := getJSON[[]packs.ConnectorFile](t, ts.URL, "/api/v1/connectors/marketplace", admin)
	if len(market) != 3 {
		t.Fatalf("marketplace %d", len(market))
	}

	wo := postJSON[model.WorkOrder](t, ts.URL, "/api/v1/work-orders", admin, map[string]string{"title": "Wire"}, 201)
	blocked := authed(t, http.MethodPatch, ts.URL+"/api/v1/work-orders/"+wo.ID, admin, map[string]string{"assignee": seed.DemoEmail, "required_skill": "electrician"})
	if blocked.StatusCode != 409 {
		t.Fatalf("skill %d", blocked.StatusCode)
	}
	blocked.Body.Close()
	if err := st.SetUserSkills(t.Context(), res.User.OrganizationID, res.User.ID, "electrician"); err != nil {
		t.Fatal(err)
	}
	ok := authed(t, http.MethodPatch, ts.URL+"/api/v1/work-orders/"+wo.ID, admin, map[string]string{"assignee": seed.DemoEmail, "required_skill": "electrician"})
	if ok.StatusCode != 200 {
		t.Fatalf("skilled %d", ok.StatusCode)
	}
	ok.Body.Close()

	start := postJSON[map[string]string](t, ts.URL, "/api/v1/auth/webauthn/register/start", admin, map[string]string{}, 200)
	postJSON[map[string]string](t, ts.URL, "/api/v1/auth/webauthn/register/finish", admin, map[string]string{
		"challenge": start["challenge"], "credential_id": "cred-1", "public_key": "pk",
	}, 201)
	login := postJSON[map[string]string](t, ts.URL, "/api/v1/auth/webauthn/login/start", "", map[string]string{"email": seed.DemoEmail}, 200)
	finish := authed(t, http.MethodPost, ts.URL+"/api/v1/auth/webauthn/login/finish", "", map[string]string{
		"email": seed.DemoEmail, "challenge": login["challenge"], "credential_id": "cred-1",
	})
	if finish.StatusCode != 200 {
		t.Fatalf("webauthn login %d", finish.StatusCode)
	}
	finish.Body.Close()

	t.Setenv("YARD_SCIM_TOKEN", "scim-test")
	scim := authed(t, http.MethodPost, ts.URL+"/api/v1/scim/v2/Users", "scim-test", map[string]string{"userName": "new@yard.local"})
	if scim.StatusCode != 201 {
		t.Fatalf("scim %d", scim.StatusCode)
	}
	scim.Body.Close()

	postJSON[map[string]string](t, ts.URL, "/api/v1/locations", admin, map[string]string{"name": "Hall"}, 201)
	exp := authed(t, http.MethodGet, ts.URL+"/api/v1/audit/export", admin, nil)
	if exp.StatusCode != 200 {
		t.Fatalf("export %d", exp.StatusCode)
	}
	exp.Body.Close()

	cert, sig := signEmail(t, seed.DemoEmail)
	t.Setenv("YARD_SAML_CERT", cert)
	xml := `<Attribute Name="email"><AttributeValue>` + seed.DemoEmail + `</AttributeValue></Attribute><Signature>` + sig + `</Signature>`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/saml/acs", strings.NewReader(xml))
	if err != nil {
		t.Fatal(err)
	}
	saml, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if saml.StatusCode != 200 {
		t.Fatalf("saml %d", saml.StatusCode)
	}
	saml.Body.Close()
}

func TestCustomRoleAndShift(t *testing.T) {
	st, err := store.Open("file:role?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(t.Context(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("role-pass"), 4)
	writer := &model.User{OrganizationID: res.User.OrganizationID, Email: "scribe@yard.local", DisplayName: "Scribe", Role: "scribe", PasswordHash: string(hash), Active: true}
	reader := &model.User{OrganizationID: res.User.OrganizationID, Email: "reader@yard.local", DisplayName: "Reader", Role: "reader", PasswordHash: string(hash), Active: true}
	if err := st.CreateUser(t.Context(), writer); err != nil || st.CreateUser(t.Context(), reader) != nil {
		t.Fatal(err)
	}
	if err := st.CreateRole(t.Context(), &store.Role{OrganizationID: res.User.OrganizationID, Name: "scribe", CanWrite: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateRole(t.Context(), &store.Role{OrganizationID: res.User.OrganizationID, Name: "reader", CanWrite: false}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	scribe := loginToken(t, ts.URL, "scribe@yard.local", "role-pass")
	postJSON[store.Geofence](t, ts.URL, "/api/v1/geofences", scribe, map[string]any{"name": "Dock", "latitude": 1, "longitude": 2, "radius_m": 50}, 201)
	ro := loginToken(t, ts.URL, "reader@yard.local", "role-pass")
	denied := authed(t, http.MethodPost, ts.URL+"/api/v1/geofences", ro, map[string]any{"name": "No", "latitude": 1, "longitude": 2, "radius_m": 50})
	if denied.StatusCode != 403 {
		t.Fatalf("reader %d", denied.StatusCode)
	}
	denied.Body.Close()
	admin := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	postJSON[store.Shift](t, ts.URL, "/api/v1/shifts", admin, map[string]string{"user_id": writer.ID, "starts_at": "2026-09-21T08:00:00Z", "ends_at": "2026-09-21T16:00:00Z"}, 201)
}

func signEmail(t *testing.T, email string) (certPEM, sig string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	sum := sha256.Sum256([]byte(email))
	raw, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return certPEM, base64.StdEncoding.EncodeToString(raw)
}
