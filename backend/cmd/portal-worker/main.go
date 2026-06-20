package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/tokenlive/tokenlive-portal/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("portal-worker started env=%s", cfg.Env)
	<-ctx.Done()
	log.Printf("portal-worker stopped")
}
