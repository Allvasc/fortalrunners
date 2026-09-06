// Package app junta os módulos e monta o roteador HTTP da API.
package app

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/admin"
	"github.com/Allvasc/fortalrunners/backend/internal/ai"
	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/challenge"
	"github.com/Allvasc/fortalrunners/backend/internal/club"
	"github.com/Allvasc/fortalrunners/backend/internal/event"
	"github.com/Allvasc/fortalrunners/backend/internal/hazard"
	"github.com/Allvasc/fortalrunners/backend/internal/health"
	"github.com/Allvasc/fortalrunners/backend/internal/heatmap"
	"github.com/Allvasc/fortalrunners/backend/internal/integration"
	"github.com/Allvasc/fortalrunners/backend/internal/landmark"
	"github.com/Allvasc/fortalrunners/backend/internal/organizer"
	"github.com/Allvasc/fortalrunners/backend/internal/payment"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/config"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/crypto"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/httpx"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
	"github.com/Allvasc/fortalrunners/backend/internal/poi"
	"github.com/Allvasc/fortalrunners/backend/internal/profile"
	"github.com/Allvasc/fortalrunners/backend/internal/qr"
	"github.com/Allvasc/fortalrunners/backend/internal/ranking"
	"github.com/Allvasc/fortalrunners/backend/internal/report"
	"github.com/Allvasc/fortalrunners/backend/internal/route"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
	"github.com/Allvasc/fortalrunners/backend/internal/safety"
	"github.com/Allvasc/fortalrunners/backend/internal/shoe"
	"github.com/Allvasc/fortalrunners/backend/internal/social"
	"github.com/Allvasc/fortalrunners/backend/internal/territory"
	"github.com/Allvasc/fortalrunners/backend/internal/weather"
)

// API mantém as dependências vivas do processo da API.
type API struct {
	Echo  *echo.Echo
	Pool  *pgxpool.Pool
	Queue queue.Publisher
}

// New monta a API: pool, fila, middlewares e rotas.
func New(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) (*API, error) {
	pub := queue.Connect(cfg.NATSURL, log)

	e := httpx.New(log, cfg.CORSOrigins)

	// --- infra ---
	health.NewHandler(pool).Register(e)

	// --- v1 ---
	v1 := e.Group("/v1")

	authSvc, err := auth.NewService(auth.Deps{
		Pool:       pool,
		JWTSecret:  cfg.JWTSecret,
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
		MFAEncKey:  cfg.MFAEncKey,
		OAuth: auth.OAuthConfig{
			GoogleClientID: cfg.GoogleClientID, GoogleClientSecret: cfg.GoogleClientSecret,
			GoogleRedirectURL: cfg.GoogleRedirectURL,
			AppleClientID:     cfg.AppleClientID, AppleTeamID: cfg.AppleTeamID, AppleKeyID: cfg.AppleKeyID,
			AppleP8Key: cfg.AppleP8Key, AppleRedirectURL: cfg.AppleRedirectURL,
		},
	})
	if err != nil {
		return nil, err
	}
	authH := auth.NewHandler(authSvc)
	authH.Register(v1) // /v1/auth/*  (público)

	// grupo protegido: exige Bearer token
	secured := v1.Group("", authH.Middleware())
	secured.GET("/me", authH.MeHandler)
	authH.RegisterSecured(secured) // /v1/auth/mfa/*

	// cofre de campo (AES-256-GCM) — cifra segredos em repouso (TOTP, tokens, telefones).
	box, err := crypto.NewBox(cfg.MFAEncKey)
	if err != nil {
		return nil, err
	}
	publicWebURL := ""
	if len(cfg.CORSOrigins) > 0 {
		publicWebURL = cfg.CORSOrigins[0]
	}

	shoeSvc := shoe.NewService(pool)
	shoe.NewHandler(shoeSvc).Register(secured)
	runSvc := run.NewService(pool, pub, shoeSvc)
	run.NewHandler(runSvc).Register(secured)
	territory.NewHandler(pool).Register(secured)
	ranking.NewHandler(pool).Register(secured)
	challenge.NewHandler(challenge.NewService(pool, log)).Register(secured)
	heatmap.NewHandler(heatmap.NewService(pool, log)).Register(secured)
	landmarkSvc := landmark.NewService(pool)
	landmark.NewHandler(landmarkSvc).Register(secured)
	routeSvc := route.NewService(route.NewStore(pool))
	route.NewHandler(routeSvc).Register(secured)
	poi.NewHandler(poi.NewStore(pool)).Register(secured)
	social.NewHandler(social.NewService(pool)).Register(secured)
	club.NewHandler(club.NewService(pool)).Register(secured)
	safetyH := safety.NewHandler(safety.NewService(pool, box, publicWebURL, log))
	safetyH.Register(secured)
	safetyH.RegisterPublic(e)
	weather.NewHandler(cfg.WeatherAPIKey).Register(secured)
	event.NewHandler(event.NewService(pool)).RegisterRoutes(secured)
	qr.NewHandler(qr.NewService(pool, cfg.JWTSecret)).RegisterRoutes(secured)
	report.NewHandler(report.NewService(pool)).Register(secured)
	hazard.NewHandler(hazard.NewService(pool)).Register(secured)
	profile.NewHandler(profile.NewService(pool)).Register(secured)
	organizer.NewHandler(organizer.NewService(pool)).Register(secured)
	paymentH := payment.NewHandler(payment.NewService(pool, payment.Config{
		AsaasAPIKey:        cfg.AsaasAPIKey,
		AsaasWebhookSecret: cfg.AsaasWebhookSecret,
	}))
	paymentH.RegisterSecured(secured)
	paymentH.RegisterPublic(v1)
	ai.NewHandler(ai.NewService(ai.Config{APIKey: cfg.AnthropicAPIKey, Model: cfg.AIModel}, pool, log)).RegisterRoutes(secured)
	admin.NewHandler(pool, pub, landmarkSvc, routeSvc).Register(secured) // /v1/admin/* (role admin/moderator + 2FA)

	// integrações (Strava) — reusa o mesmo cofre de campo.
	integSvc := integration.NewService(pool, box, integration.Config{
		JWTSecret: cfg.JWTSecret,
		Strava: integration.StravaConfig{
			ClientID: cfg.StravaClientID, ClientSecret: cfg.StravaClientSecret,
			RedirectURL: cfg.StravaRedirectURL, WebhookVerifyToken: cfg.StravaWebhookVerifyToken,
		},
	}, runSvc, log)
	integH := integration.NewHandler(integSvc)
	integH.RegisterSecured(secured)
	integH.RegisterPublic(v1)

	return &API{Echo: e, Pool: pool, Queue: pub}, nil
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
