// Package queue abstrai a fila de mensagens (NATS JetStream em produção).
// A interface deixa o domínio independente do broker.
//
// Sem NATS (ex.: deploy de instância única), Connect devolve um barramento
// EM PROCESSO: publish entrega de forma assíncrona aos handlers registrados no
// mesmo processo. É o que mantém o pipeline de território rodando quando a API
// sobe sozinha, sem um worker separado.
package queue

import (
	"context"
	"log/slog"
	"sync"

	"github.com/nats-io/nats.go"
)

// Publisher publica um evento num assunto (subject).
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
	Close()
}

// Subscriber registra um consumidor local (só o barramento em processo suporta).
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte))
}

// Assuntos conhecidos.
const (
	SubjectRunUploaded = "run.uploaded"
)

type natsPub struct {
	nc  *nats.Conn
	log *slog.Logger
}

// Connect abre a conexão com o NATS. Se falhar, devolve um barramento em processo.
func Connect(url string, log *slog.Logger) Publisher {
	nc, err := nats.Connect(url, nats.Timeout(nats.DefaultTimeout), nats.MaxReconnects(-1))
	if err != nil {
		log.Warn("queue: NATS indisponível — barramento em processo (API single-instance)", "err", err)
		return NewInProc(log)
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

// --- barramento em processo ---

type InProc struct {
	log  *slog.Logger
	mu   sync.RWMutex
	subs map[string][]func([]byte)
	wg   sync.WaitGroup
}

func NewInProc(log *slog.Logger) *InProc {
	return &InProc{log: log, subs: map[string][]func([]byte){}}
}

func (b *InProc) Subscribe(subject string, handler func(data []byte)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[subject] = append(b.subs[subject], handler)
}

func (b *InProc) Publish(_ context.Context, subject string, data []byte) error {
	b.mu.RLock()
	handlers := b.subs[subject]
	b.mu.RUnlock()

	cp := make([]byte, len(data))
	copy(cp, data)
	for _, h := range handlers {
		h := h
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					b.log.Error("queue: handler em processo entrou em pânico", "subject", subject, "recover", r)
				}
			}()
			h(cp)
		}()
	}
	return nil
}

func (b *InProc) Close() { b.wg.Wait() }

// Noop descarta tudo. Mantido para testes.
type Noop struct{}

func (Noop) Publish(context.Context, string, []byte) error { return nil }
func (Noop) Close()                                        {}
