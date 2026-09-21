package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zyvorai/yard/internal/api"
	"github.com/zyvorai/yard/internal/config"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

//go:embed all:static
var staticRoot embed.FS

func newLogger() *slog.Logger {
	var handler slog.Handler
	if env("YARD_LOG_FORMAT", "text") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	return slog.New(handler)
}

func main() {
	logger := newLogger()
	addr := env("YARD_LISTEN", ":8080")
	dsn := api.DefaultDSN()
	if dir := filepath.Dir(fileFromDSN(dsn)); dir != "" && dir != "." && dir != "/" {
		_ = os.MkdirAll(dir, 0o755)
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(dsn)
	if err != nil {
		logger.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	res, err := seed.BootstrapWith(context.Background(), st, seed.Options{
		Mode:              cfg.Mode,
		BootstrapEmail:    os.Getenv("YARD_BOOTSTRAP_EMAIL"),
		BootstrapPassword: os.Getenv("YARD_BOOTSTRAP_PASSWORD"),
		PublicURL:         cfg.PublicURL,
	})
	if err != nil {
		logger.Error("bootstrap", "err", err)
		os.Exit(1)
	}
	logger.Info("bootstrap", "status", seed.FormatWelcome(res))
	if res != nil && res.IngestToken != "" {
		dataDir := cfg.DataDir
		_ = os.MkdirAll(dataDir, 0o755)
		_ = os.WriteFile(filepath.Join(dataDir, "ingest.token"), []byte(res.IngestToken+"\n"), 0o600)
		_ = os.WriteFile(filepath.Join(dataDir, "simulator.token"), []byte(res.SimulatorTok+"\n"), 0o600)
	}

	srv := api.NewWith(st, logger, cfg)
	if sub, err := fs.Sub(staticRoot, "static"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			srv.Static = sub
		}
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	srv.Engine.StartStaleTicker(runCtx, 30*time.Second)
	srv.Engine.StartActionSweeper(runCtx, 60*time.Second)
	srv.Engine.StartMaintenance(runCtx, 30*time.Second)
	srv.Engine.StartRetention(runCtx, time.Hour)
	srv.Engine.StartLiveRelay(runCtx)
	if srv.Queue != nil {
		srv.Queue.Start(runCtx, 2*time.Second)
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		logger.Info("yard listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	runCancel()
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
