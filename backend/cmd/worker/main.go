// Comando: territory-worker — consome a fila e roda o pipeline de território
// (detecção de laço, polígono, gate de zona de risco, gravação).
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/logging"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
	"github.com/Allvasc/fortalrunners/backend/internal/territory"
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

	proc := territory.NewProcessor(pool, log)

	sub, err := nc.QueueSubscribe(queue.SubjectRunUploaded, "territory-workers", func(m *nats.Msg) {
		var ev struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(m.Data, &ev); err != nil || ev.RunID == "" {
			log.Warn("evento malformado", "subject", m.Subject)
			return
		}
		proc.Process(context.Background(), ev.RunID)
	})
	if err != nil {
		log.Error("não foi possível assinar", "err", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe() //nolint:errcheck

	log.Info("worker pronto", "subject", queue.SubjectRunUploaded)
	<-ctx.Done()
	log.Info("worker desligando…")
}
