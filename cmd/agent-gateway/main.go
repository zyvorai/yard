package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/connectors/deviceagent"
)

// Gateway connector: pull Device Agent locally and push normalized inventory
// and observations to Yard. No inbound hole-punch to remote devices.
func main() {
	agent := strings.TrimRight(env("DEVICE_AGENT_URL", "http://127.0.0.1:9188"), "/")
	yard := strings.TrimRight(env("YARD_URL", "http://127.0.0.1:8080"), "/")
	tok := strings.TrimSpace(env("YARD_INGEST_TOKEN", readFile(env("YARD_INGEST_TOKEN_FILE", "data/ingest.token"))))
	if tok == "" {
		log.Fatal("YARD_INGEST_TOKEN is required")
	}
	log.Printf("device-agent gateway %s → %s", agent, yard)
	client := deviceagent.New(agent, yard, tok)
	for {
		if err := client.Sync(context.Background()); err != nil {
			log.Printf("cycle: %v", err)
		}
		time.Sleep(15 * time.Second)
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
