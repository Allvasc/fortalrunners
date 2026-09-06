// Comando: scheduler — jobs periódicos do FortalRunners.
// Fase 1: garante os períodos de desafio e fecha os que encerraram (com prêmios).
// Depois: refresh de heat_agg, reconstrução de risk_zones, conciliação, etc.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Allvasc/fortalrunners/backend/internal/challenge"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/logging"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logging.New(cfg.LogLevel, cfg.Env).With("proc", "scheduler")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("banco indisponível", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	chl := challenge.NewService(pool, log)

	tick := func() {
		if err := chl.EnsurePeriods(ctx); err != nil {
			log.Error("EnsurePeriods", "err", err)
		}
		if err := chl.ClosePeriods(ctx); err != nil {
			log.Error("ClosePeriods", "err", err)
		}
	}

	tick() // uma vez no boot
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	log.Info("scheduler pronto (tick a cada 5 min)")
	for {
		select {
		case <-ctx.Done():
			log.Info("scheduler desligando…")
			return
		case <-t.C:
			tick()
		}
	}
}
