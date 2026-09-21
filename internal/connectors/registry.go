package connectors

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

// Kind is one connector implementation the process can run.
type Kind struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
	exec    func(d *Dispatcher, ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error)
}

var kinds []Kind

func register(name string, actions []string, exec func(*Dispatcher, context.Context, string, *model.Connector, string, string) (string, string, error)) {
	kinds = append(kinds, Kind{Name: name, Actions: actions, exec: exec})
}

func init() {
	register("device-agent", []string{"inventory.refresh", "diagnostics.read"}, (*Dispatcher).deviceAgent)
	register("http", []string{}, (*Dispatcher).recorded)
	register("simulator", []string{}, (*Dispatcher).recorded)
	register("nodra", []string{"telemetry.receive", "sync"}, (*Dispatcher).nodra)
	register("fleet", []string{"lifecycle.request", "desired.progress", "sync"}, (*Dispatcher).fleet)
	register("ota", []string{"campaign.list", "update.delegate", "sync"}, (*Dispatcher).ota)
	register("webhook", []string{"post"}, (*Dispatcher).webhook)
	register("servicenow", []string{"incident.create", "sync"}, (*Dispatcher).enterprise)
	register("maximo", []string{"workorder.create", "sync"}, (*Dispatcher).enterprise)
	register("sap", []string{"notification.create", "sync"}, (*Dispatcher).enterprise)
}

// Catalog returns the registered connector kinds without their execute funcs.
func Catalog() []Kind {
	out := make([]Kind, len(kinds))
	for i, k := range kinds {
		out[i] = Kind{Name: k.Name, Actions: append([]string(nil), k.Actions...)}
	}
	return out
}

func (d *Dispatcher) recorded(context.Context, string, *model.Connector, string, string) (string, string, error) {
	return "recorded", "Ingest-only connector; use HTTP ingest endpoints.", nil
}

func (d *Dispatcher) webhook(ctx context.Context, _ string, conn *model.Connector, action, payload string) (string, string, error) {
	if conn.Endpoint == "" {
		return "failed", "", fmt.Errorf("connector endpoint required")
	}
	if err := d.policy().Validate(conn.Endpoint); err != nil {
		return "failed", "", err
	}
	if payload == "" {
		payload = "{}"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conn.Endpoint, bytes.NewReader([]byte(payload)))
	if err != nil {
		return "failed", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Yard-Action", action)
	if bearer, err := d.oauthBearer(ctx, conn); err == nil && bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client, err := d.httpClient(conn.Config)
	if err != nil {
		return "failed", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "failed", "", err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "failed", string(buf), fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return "completed", string(buf), nil
}

func (d *Dispatcher) enterprise(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	if action == "sync" {
		return d.enterpriseSync(ctx, conn)
	}
	if payload == "" || payload == "{}" {
		payload = conn.Config
	}
	return d.webhook(ctx, orgID, conn, action, payload)
}

func (d *Dispatcher) enterpriseSync(ctx context.Context, conn *model.Connector) (string, string, error) {
	endpoint, token, err := d.endpointAndToken(ctx, conn, "")
	if err != nil {
		return "failed", "", err
	}
	if endpoint == "" {
		return "failed", "", fmt.Errorf("connector endpoint required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "failed", "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if bearer, err := d.oauthBearer(ctx, conn); err == nil && bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client, err := d.httpClient(conn.Config)
	if err != nil {
		return "failed", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "failed", "", err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "failed", string(buf), fmt.Errorf("sync status %d", resp.StatusCode)
	}
	return "completed", string(buf), nil
}

func (d *Dispatcher) httpClient(cfg string) (*http.Client, error) {
	skip := d.tlsRequested(cfg) && d.allowSkipTLS()
	tlsCfg := &tls.Config{InsecureSkipVerify: skip} //nolint:gosec
	certPEM := configString(cfg, "client_cert_pem")
	keyPEM := configString(cfg, "client_key_pem")
	if certPEM != "" && keyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	if ca := configString(cfg, "ca_cert_pem"); ca != "" {
		pool := x509.NewCertPool()
		if block, _ := pem.Decode([]byte(ca)); block != nil {
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				pool.AddCert(c)
				tlsCfg.RootCAs = pool
			}
		}
	}
	return &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg}}, nil
}

func (d *Dispatcher) oauthBearer(ctx context.Context, conn *model.Connector) (string, error) {
	if tok := configString(conn.Config, "oauth_access_token"); tok != "" {
		return tok, nil
	}
	tokenURL := configString(conn.Config, "oauth_token_url")
	clientID := configString(conn.Config, "oauth_client_id")
	clientSecret := configString(conn.Config, "oauth_client_secret")
	if tokenURL == "" || clientID == "" || clientSecret == "" {
		return "", nil
	}
	if err := d.policy().Validate(tokenURL); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.policy().HTTPClient(8*time.Second, false).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("oauth status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &out) != nil || out.AccessToken == "" {
		return "", fmt.Errorf("oauth response missing access_token")
	}
	return out.AccessToken, nil
}
