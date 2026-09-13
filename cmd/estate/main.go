package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zyvorai/estate/internal/api"
	"github.com/zyvorai/estate/internal/seed"
	"github.com/zyvorai/estate/internal/store"
)

//go:embed all:static
var staticRoot embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	addr := env("ESTATE_LISTEN", ":8080")
	dsn := api.DefaultDSN()
	if dir := filepath.Dir(fileFromDSN(dsn)); dir != "" && dir != "." && dir != "/" {
		_ = os.MkdirAll(dir, 0o755)
	}
	st, err := store.Open(dsn)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if res != nil && res.IngestToken != "" {
		log.Printf("bootstrap %s", seed.FormatWelcome(res))
		_ = os.WriteFile("data/ingest.token", []byte(res.IngestToken+"\n"), 0o600)
		_ = os.WriteFile("data/simulator.token", []byte(res.SimulatorTok+"\n"), 0o600)
	}

	srv := api.New(st, log.Default())
	if sub, err := fs.Sub(staticRoot, "static"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			srv.Static = sub
		}
	}

	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("estate listening on %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func fileFromDSN(dsn string) string {
	if len(dsn) > 5 && dsn[:5] == "file:" {
		p := dsn[5:]
		if i := indexQ(p); i >= 0 {
			p = p[:i]
		}
		return p
	}
	return dsn
}

func indexQ(s string) int {
	for i, c := range s {
		if c == '?' {
			return i
		}
	}
	return -1
}
