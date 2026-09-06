// Comando: territory-worker — consome a fila e processa corridas
// (limpeza, map-matching, detecção de laço, gate de risco, resolução de
// conflito, indexação H3, anti-fraude, rollups).
//
// Fase 0: apenas conecta na fila e loga os eventos recebidos. A lógica de
// processamento entra na Fase 1.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/logging"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logging.New(cfg.LogLevel, cfg.Env).With("proc", "worker")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("banco indisponível", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	nc, err := nats.Connect(cfg.NATSURL, nats.MaxReconnects(-1))
	if err != nil {
		log.Error("NATS indisponível — o worker precisa da fila", "err", err)
		os.Exit(1)
	}
	defer nc.Drain()

	sub, err := nc.Subscribe(queue.SubjectRunUploaded, func(m *nats.Msg) {
		log.Info("evento recebido", "subject", m.Subject, "bytes", len(m.Data))
		// TODO(fase-1): pipeline de território.
	})
	if err != nil {
		log.Error("não foi possível assinar", "err", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	log.Info("worker pronto", "subject", queue.SubjectRunUploaded)
	<-ctx.Done()
	log.Info("worker desligando…")
}
