// Package realtime é o hub WebSocket em processo (plano §10: WS /v1/ws).
// Numa instância única (deploy atual) entrega eventos de domínio — corrida
// processada, feed — para as conexões abertas daquele usuário. Com múltiplas
// instâncias seria preciso um fan-out via NATS/Redis; a interface não muda.
package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

type Event struct {
	Type string `json:"type"` // run.processed | feed.new | challenge.period_closed
	Data any    `json:"data"`
	At   int64  `json:"at"`
}

type client struct {
	send chan []byte
}

type Hub struct {
	log *slog.Logger
	mu  sync.RWMutex
	// userID -> conexões abertas
	conns map[string]map[*client]struct{}
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{log: log, conns: map[string]map[*client]struct{}{}}
}

func (h *Hub) add(userID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[userID] == nil {
		h.conns[userID] = map[*client]struct{}{}
	}
	h.conns[userID][c] = struct{}{}
}

func (h *Hub) remove(userID string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.conns[userID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.conns, userID)
		}
	}
}

// Publish entrega um evento a todas as conexões do usuário (não bloqueia:
// conexão lenta perde a mensagem).
func (h *Hub) Publish(userID, evType string, data any) {
	payload, err := json.Marshal(Event{Type: evType, Data: data, At: time.Now().Unix()})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.conns[userID] {
		select {
		case c.send <- payload:
		default:
		}
	}
}
