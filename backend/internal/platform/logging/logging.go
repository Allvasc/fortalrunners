// Package logging configura o logger estruturado (slog) do processo.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New devolve um *slog.Logger com saída JSON e nível a partir de LOG_LEVEL.
func New(level, env string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	var h slog.Handler
	opts := &slog.HandlerOptions{Level: lvl}
	if env == "dev" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h).With("service", "fortalrunners", "env", env)
}
