// Package queue abstrai a fila de mensagens (NATS JetStream em produção).
// A interface deixa o domínio independente do broker; há uma implementação
// noop para testes e para subir sem o NATS.
package queue

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// Publisher publica um evento num assunto (subject).
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
	Close()
}

// Assuntos conhecidos.
const (
	SubjectRunUploaded = "run.uploaded"
)

type natsPub struct {
	nc  *nats.Conn
	log *slog.Logger
}

// Connect abre a conexão com o NATS. Se falhar, devolve um Noop e loga um aviso
// (Fase 0: a fila não é crítica para subir a API).
func Connect(url string, log *slog.Logger) Publisher {
	nc, err := nats.Connect(url, nats.Timeout(nats.DefaultTimeout), nats.MaxReconnects(-1))
	if err != nil {
		log.Warn("queue: NATS indisponível, usando publisher noop", "err", err)
		return Noop{}
	}
	log.Info("queue: conectado ao NATS", "url", url)
	return &natsPub{nc: nc, log: log}
}

func (p *natsPub) Publish(_ context.Context, subject string, data []byte) error {
	return p.nc.Publish(subject, data)
}

func (p *natsPub) Close() {
	if p.nc != nil {
		_ = p.nc.Drain()
	}
}

// Noop descarta tudo. Útil em testes e no boot sem broker.
type Noop struct{}

func (Noop) Publish(context.Context, string, []byte) error { return nil }
func (Noop) Close()                                        {}
