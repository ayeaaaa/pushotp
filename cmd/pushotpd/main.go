package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
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

	if cfg.Server.APIKey == "" {
		host, _, err := net.SplitHostPort(cfg.Server.Addr)
		if err == nil && !isLoopbackHost(host) {
			logger.Warn("api_key is empty and server may be reachable from other hosts; /send and /verify will accept unauthenticated requests", "addr", cfg.Server.Addr)
		}
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           httpapi.New(v, cfg.Server.APIKey, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", cfg.Server.Addr)
	if err != nil {
		logger.Error("listen", "err", err)
		_ = v.Close()
		os.Exit(1)
	}
	logger.Info("pushotpd listening", "addr", ln.Addr().String())
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
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

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
