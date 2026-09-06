// Package httpx monta o servidor HTTP (Echo) com os middlewares padrão:
// request id, logging estruturado, recover, CORS e rate limit.
package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// New devolve um *echo.Echo configurado. As rotas são registradas pelo caller.
func New(log *slog.Logger, corsOrigins []string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.RequestIDWithConfig(middleware.RequestIDConfig{
		TargetHeader: echo.HeaderXRequestID,
	}))
	e.Use(requestLogger(log))
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderXRequestID, "Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20))) // 20 req/s por IP — ajustar por rota

	e.HTTPErrorHandler = errorHandler(log)
	return e
}

func requestLogger(log *slog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURI:       true,
		LogMethod:    true,
		LogLatency:   true,
		LogRequestID: true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			lvl := slog.LevelInfo
			if v.Status >= 500 || v.Error != nil {
				lvl = slog.LevelError
			}
			log.LogAttrs(context.Background(), lvl, "http",
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency.Round(time.Millisecond)),
				slog.String("request_id", v.RequestID),
			)
			return nil
		},
	})
}

// errorHandler devolve JSON consistente e nunca vaza stack para o cliente.
func errorHandler(log *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := http.StatusInternalServerError
		msg := "erro interno"
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if m, ok := he.Message.(string); ok {
				msg = m
			}
		}
		if code >= 500 {
			log.Error("http erro", "err", err, "path", c.Path())
			msg = "erro interno"
			if hub := sentry.CurrentHub(); hub != nil && hub.Client() != nil {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetTag("path", c.Path())
					scope.SetTag("method", c.Request().Method)
					scope.SetTag("request_id", c.Response().Header().Get(echo.HeaderXRequestID))
					hub.CaptureException(err)
				})
			}
		}
		_ = c.JSON(code, map[string]any{"error": msg})
	}
}
