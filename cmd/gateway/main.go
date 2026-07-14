package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myproject/api-gateway/internal/config"
	"myproject/api-gateway/internal/logger"
	"myproject/api-gateway/internal/router"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		slog.Error("config not loaded", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.ServiceName)

	r, err := router.New(cfg, log)
	if err != nil {
		log.Error("router failed", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server died", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("started", "port", cfg.Port, "service", cfg.ServiceName)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	log.Info("stopped")
}
