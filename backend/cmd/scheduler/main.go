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
	"github.com/Allvasc/fortalrunners/backend/internal/integration"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/logging"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/mapmatch"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
	"github.com/Allvasc/fortalrunners/backend/internal/shoe"
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

	pub := queue.Connect(cfg.NATSURL, log)
	defer pub.Close()
	box, err := crypto.NewBox(cfg.MFAEncKey)
	if err != nil {
		log.Error("chave de criptografia inválida", "err", err)
		os.Exit(1)
	}
	integSvc := integration.NewService(pool, box, integration.Config{
		JWTSecret: cfg.JWTSecret,
		Strava: integration.StravaConfig{
			ClientID: cfg.StravaClientID, ClientSecret: cfg.StravaClientSecret,
			RedirectURL: cfg.StravaRedirectURL, WebhookVerifyToken: cfg.StravaWebhookVerifyToken,
		},
	}, run.NewService(pool, pub, shoe.NewService(pool), mapmatch.New(cfg.OSRMURL)), log)

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
		if err := integSvc.RunPendingJobs(ctx); err != nil {
			log.Error("integration.RunPendingJobs", "err", err)
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
