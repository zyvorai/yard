package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/zyvorai/yard/internal/egress"
	"github.com/zyvorai/yard/internal/mail"
	"github.com/zyvorai/yard/internal/secrets"
)

// Config is the process runtime selected by YARD_MODE.
type Config struct {
	Mode             string
	PublicURL        string
	SecretKey        []byte
	AllowInsecureTLS bool
	CORSOrigins      []string
	DataDir          string
	Egress           *egress.Policy
}

// Load reads process environment. Production refuses to start without a
// public URL and a 32-byte master key.
func Load() (Config, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("YARD_MODE")))
	if mode == "" {
		mode = "demo"
	}
	if mode != "demo" && mode != "production" {
		return Config{}, fmt.Errorf("YARD_MODE must be demo or production")
	}
	dataDir := os.Getenv("YARD_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("YARD_PUBLIC_URL")), "/")
	allowInsecure := os.Getenv("YARD_ALLOW_INSECURE_TLS") == "1"
	cors := splitCSV(os.Getenv("YARD_CORS_ORIGINS"))
	allow := splitCSV(os.Getenv("YARD_EGRESS_ALLOWLIST"))

	var key []byte
	var err error
	if mode == "production" {
		if publicURL == "" {
			return Config{}, fmt.Errorf("YARD_PUBLIC_URL is required in production")
		}
		u, perr := url.ParseRequestURI(publicURL)
		if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Config{}, fmt.Errorf("YARD_PUBLIC_URL must be an absolute http(s) url")
		}
		key, err = secrets.ParseKey(os.Getenv("YARD_SECRET_KEY"))
		if err != nil {
			return Config{}, fmt.Errorf("production requires YARD_SECRET_KEY: %w", err)
		}
	} else {
		key, err = secrets.LoadOrCreate(dataDir, os.Getenv("YARD_SECRET_KEY"))
		if err != nil {
			return Config{}, err
		}
	}
	pol := egress.Demo()
	if mode == "production" {
		pol = egress.Production(allow)
	}
	return Config{
		Mode:             mode,
		PublicURL:        publicURL,
		SecretKey:        key,
		AllowInsecureTLS: allowInsecure,
		CORSOrigins:      cors,
		DataDir:          dataDir,
		Egress:           pol,
	}, nil
}

// DemoConfig is an in-memory demo runtime for tests.
func DemoConfig() Config {
	key, err := secrets.RandomKey()
	if err != nil {
		key = make([]byte, 32)
	}
	return Config{Mode: "demo", SecretKey: key, DataDir: "data", Egress: egress.Demo()}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SMTPConfigured reports whether invite and reset mail can be sent.
func SMTPConfigured() bool { return mail.Configured() }
