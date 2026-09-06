// Package app junta os módulos e monta o roteador HTTP da API.
package app

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/health"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/httpx"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
	"github.com/Allvasc/fortalrunners/backend/internal/ranking"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
	"github.com/Allvasc/fortalrunners/backend/internal/shoe"
	"github.com/Allvasc/fortalrunners/backend/internal/territory"
)

// API mantém as dependências vivas do processo da API.
type API struct {
	Echo  *echo.Echo
	Pool  *pgxpool.Pool
	Queue queue.Publisher
}

// New monta a API: pool, fila, middlewares e rotas.
func New(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) *API {
	pub := queue.Connect(cfg.NATSURL, log)

	e := httpx.New(log, cfg.CORSOrigins)

	// --- infra ---
	health.NewHandler(pool).Register(e)

	// --- v1 ---
	v1 := e.Group("/v1")

	authSvc := auth.NewService(auth.Deps{
		Pool:       pool,
		JWTSecret:  cfg.JWTSecret,
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
	})
	authH := auth.NewHandler(authSvc)
	authH.Register(v1) // /v1/auth/*  (público)

	// grupo protegido: exige Bearer token
	secured := v1.Group("", authH.Middleware())
	secured.GET("/me", authH.MeHandler)

	shoeSvc := shoe.NewService(pool)
	shoe.NewHandler(shoeSvc).Register(secured)
	run.NewHandler(run.NewService(pool, pub, shoeSvc)).Register(secured)
	territory.NewHandler(pool).Register(secured)
	ranking.NewHandler(pool).Register(secured)

	return &API{Echo: e, Pool: pool, Queue: pub}
}

// Close libera os recursos do processo.
func (a *API) Close() {
	if a.Queue != nil {
		a.Queue.Close()
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
}
