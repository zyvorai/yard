package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ParseKey accepts a 32-byte key as standard base64 or hex.
func ParseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty key")
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
		return b, nil
	}
	return nil, fmt.Errorf("YARD_SECRET_KEY must be 32 bytes, base64 or hex")
}

// RandomKey returns a new AES-256 key.
func RandomKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// LoadOrCreate uses envKey when set, otherwise reads or writes dir/secret.key (mode 0600).
func LoadOrCreate(dir, envKey string) ([]byte, error) {
	if strings.TrimSpace(envKey) == "" && os.Getenv("YARD_REQUIRE_CMK") == "1" {
		return nil, fmt.Errorf("YARD_SECRET_KEY is required when YARD_REQUIRE_CMK=1")
	}
	if strings.TrimSpace(envKey) != "" {
		return ParseKey(envKey)
	}
	if dir == "" {
		dir = "data"
	}
	path := filepath.Join(dir, "secret.key")
	if b, err := os.ReadFile(path); err == nil {
		return ParseKey(strings.TrimSpace(string(b)))
	}
	key, err := RandomKey()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	enc := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(enc+"\n"), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// FetchVaultKey reads a 32-byte key from a Vault KV path. YARD_VAULT_ADDR is
// the server origin. The token is sent as X-Vault-Token. KV v2 nests the
// key under data.data; KV v1 uses data.key.
func FetchVaultKey(addr, token, path string) (string, error) {
	addr = strings.TrimRight(strings.TrimSpace(addr), "/")
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		path = "secret/data/yard"
	}
	if addr == "" || token == "" {
		return "", fmt.Errorf("vault address and token are required")
	}
	req, err := http.NewRequest(http.MethodGet, addr+"/v1/"+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", token)
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("vault status %d", resp.StatusCode)
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return "", fmt.Errorf("vault response is not json")
	}
	data, _ := payload["data"].(map[string]any)
	if nested, ok := data["data"].(map[string]any); ok {
		data = nested
	}
	key, _ := data["key"].(string)
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("vault response has no key")
	}
	return key, nil
}

// Encrypt seals plaintext with AES-256-GCM. The result is standard base64 of nonce||ciphertext.
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func Decrypt(key []byte, blob string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(blob))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Hint is a non-secret marker safe to return from the API.
func Hint(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "set"
	}
	return "…" + secret[len(secret)-4:]
}
