// Comando: API HTTP do FortalRunners.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Allvasc/fortalrunners/backend/internal/app"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/db"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/logging"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/obs"
)

func main() {
	_ = godotenv.Load() // .env em dev; sem efeito se o arquivo não existir

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logging.New(cfg.LogLevel, cfg.Env)

	flushSentry := obs.InitSentry(cfg.SentryDSN, cfg.Env, os.Getenv("RENDER_GIT_COMMIT"), log)
	defer flushSentry()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	for attempts := 1; attempts <= 10; attempts++ {
		pool, err = db.Open(ctx, cfg.DatabaseURL)
		if err == nil {
			break
		}
		log.Warn("aguardando banco de dados...", "tentativa", attempts, "err", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Error("não foi possível abrir o banco após várias tentativas", "err", err)
		os.Exit(1)
	}

	if os.Getenv("AUTO_MIGRATE") != "false" {
		if err := db.Migrate(ctx, pool); err != nil {
			log.Error("migração falhou", "err", err)
			os.Exit(1)
		}
		log.Info("migrações aplicadas")
	}

	a, err := app.New(ctx, cfg, log, pool)
	if err != nil {
		log.Error("não foi possível montar a API", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           a.Echo,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("API ouvindo", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("desligando…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
