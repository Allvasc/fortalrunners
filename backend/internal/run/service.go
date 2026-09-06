// Package run recebe corridas, valida o essencial, grava runs + run_tracks e
// enfileira o processamento pesado (território, H3, anti-fraude) para o worker.
package run

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
)

var (
	ErrTooFewPoints = errors.New("traçado insuficiente para registrar a corrida")
	ErrBadWindow    = errors.New("started_at/ended_at inválidos")
	ErrShoeNotYours = errors.New("esse par de tênis não é seu")
)

// ShoeChecker confirma a posse de um par de tênis (implementado por shoe.Service).
type ShoeChecker interface {
	OwnedBy(ctx context.Context, userID, shoeID string) (bool, error)
}

type Service struct {
	store *store
	pub   queue.Publisher
	shoes ShoeChecker
}

func NewService(pool *pgxpool.Pool, pub queue.Publisher, shoes ShoeChecker) *Service {
	return &Service{store: newStore(pool), pub: pub, shoes: shoes}
}

// Ingest grava a corrida e publica run.uploaded.
func (s *Service) Ingest(ctx context.Context, userID string, in IngestInput) (View, error) {
	if in.StartedAt.IsZero() || in.EndedAt.IsZero() || !in.EndedAt.After(in.StartedAt) {
		return View{}, ErrBadWindow
	}
	if in.EndedAt.Sub(in.StartedAt) > 24*time.Hour {
		return View{}, ErrBadWindow
	}

	if in.ShoeID != "" && s.shoes != nil {
		ok, err := s.shoes.OwnedBy(ctx, userID, in.ShoeID)
		if err != nil {
			return View{}, err
		}
		if !ok {
			return View{}, ErrShoeNotYours
		}
	}

	c, ok := clean(in.Points)
	if !ok {
		return View{}, ErrTooFewPoints
	}

	metrics := computeMetrics(c.points, in.Cadence, in.HeartRate)
	fraud := scoreFraud(in, c, metrics)

	status := "processing"
	if fraud.flagged() {
		status = "flagged"
	}

	v, err := s.store.create(ctx, createArgs{
		ID: id.New(), UserID: userID, In: in, Clean: c, Metrics: metrics,
		FraudScore: fraud.Score, FraudFlags: fraud.Flags, Status: status,
	})
	if err != nil {
		return View{}, err
	}

	if status == "processing" {
		payload, _ := json.Marshal(map[string]string{"run_id": v.ID, "user_id": userID})
		_ = s.pub.Publish(ctx, queue.SubjectRunUploaded, payload)
	}
	return v, nil
}

func (s *Service) Get(ctx context.Context, id, userID string) (View, error) {
	return s.store.get(ctx, id, userID)
}

// Metrics devolve o pacote de precisão de uma corrida (splits, elevação, GAP).
func (s *Service) Metrics(ctx context.Context, id, userID string) (MetricsView, error) {
	return s.store.metrics(ctx, id, userID)
}

func (s *Service) List(ctx context.Context, userID string, limit int) ([]View, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.store.list(ctx, userID, limit)
}
