package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digitalis-io/url-shortner/internal/auth"
	"github.com/digitalis-io/url-shortner/internal/config"
	"github.com/digitalis-io/url-shortner/internal/httpapi"
	"github.com/digitalis-io/url-shortner/internal/shorturl"
	"github.com/digitalis-io/url-shortner/internal/storage/cassandra"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}

	store, err := cassandra.Connect(cfg)
	if err != nil {
		logger.Error("connect cassandra failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	authn, err := auth.New(cfg)
	if err != nil {
		logger.Error("configure auth failed", "err", err)
		os.Exit(1)
	}
	service := shorturl.NewService(store, cfg.PublicBaseURL, cfg.CodeLength)
	api := httpapi.New(cfg, authn, service, store, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr, "public_base_url", cfg.PublicBaseURL, "admin_base_url", cfg.AdminBaseURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
