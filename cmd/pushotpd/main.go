package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pushotp"
	"pushotp/httpapi"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	raw, err := os.ReadFile(*configPath)
	if err != nil {
		logger.Error("read config", "err", err)
		os.Exit(1)
	}
	fc, err := decodeConfig(raw)
	if err != nil {
		logger.Error("parse config", "err", err)
		os.Exit(1)
	}
	cfg, err := fc.toConfig()
	if err != nil {
		logger.Error("convert config", "err", err)
		os.Exit(1)
	}
	cfg = cfg.WithDefaults()

	v, err := pushotp.New(cfg)
	if err != nil {
		logger.Error("init verifier", "err", err)
		os.Exit(1)
	}
	defer v.Close()

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           httpapi.New(v, cfg.Server.APIKey, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("pushotpd listening", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}
