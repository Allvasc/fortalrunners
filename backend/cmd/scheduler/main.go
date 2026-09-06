// Comando: scheduler — jobs periódicos do FortalRunners.
// Fase 1: garante/fecha os períodos de desafio (com prêmios) e faz o refresh do
// mapa de calor pessoal (heat_agg).
// Depois: reconstrução de risk_zones, conciliação Asaas, etc.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Allvasc/fortalrunners/backend/internal/challenge"
	"github.com/Allvasc/fortalrunners/backend/internal/heatmap"
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
	heat := heatmap.NewService(pool, log)

	tick := func() {
		if err := chl.EnsurePeriods(ctx); err != nil {
			log.Error("EnsurePeriods", "err", err)
		}
		if err := chl.ClosePeriods(ctx); err != nil {
			log.Error("ClosePeriods", "err", err)
		}
		if err := heat.Refresh(ctx); err != nil {
			log.Error("heatmap.Refresh", "err", err)
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
