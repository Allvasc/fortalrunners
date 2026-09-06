// Package obs concentra observabilidade opcional. Sem SENTRY_DSN, InitSentry
// é no-op e o resto do código (sentry.CaptureException) continua seguro —
// o SDK usa um cliente noop até Init.
package obs

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

// InitSentry liga o Sentry quando dsn != "". Devolve uma função de flush para
// rodar no shutdown. Erro de init não derruba o processo — só registra.
func InitSentry(dsn, env, release string, log *slog.Logger) func() {
	if dsn == "" {
		return func() {}
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          release,
		EnableTracing:    false,
		AttachStacktrace: true,
		SampleRate:       1.0,
	})
	if err != nil {
		log.Warn("sentry: init falhou, seguindo sem", "err", err)
		return func() {}
	}
	log.Info("sentry ativo", "env", env)
	return func() { sentry.Flush(2 * time.Second) }
}
