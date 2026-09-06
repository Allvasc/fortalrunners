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
	"github.com/Allvasc/fortalrunners/backend/internal/platform/mapmatch"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
)

var (
	ErrTooFewPoints = errors.New("traçado insuficiente para registrar a corrida")
	ErrBadWindow    = errors.New("started_at/ended_at inválidos")
	ErrShoeNotYours = errors.New("esse par de tênis não é seu")
	ErrDuplicateRun = errors.New("corrida duplicada (já importada ou já registrada nativamente)")
)

// ShoeChecker confirma a posse de um par de tênis (implementado por shoe.Service).
type ShoeChecker interface {
	OwnedBy(ctx context.Context, userID, shoeID string) (bool, error)
}

type Service struct {
	store   *store
	pub     queue.Publisher
	shoes   ShoeChecker
	matcher *mapmatch.Matcher
}

func NewService(pool *pgxpool.Pool, pub queue.Publisher, shoes ShoeChecker, matcher *mapmatch.Matcher) *Service {
	if matcher == nil {
		matcher = mapmatch.New("")
	}
	return &Service{store: newStore(pool), pub: pub, shoes: shoes, matcher: matcher}
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

	// Zona de ocultação (plano §14): o traçado dentro do raio de casa/trabalho
	// nunca é gravado. Filtra antes de calcular métricas e de gravar a geometria.
	if hlat, hlng, hr, has := s.store.privacyZone(ctx, userID); has {
		c.points = dropWithinRadius(c.points, hlat, hlng, hr)
		if len(c.points) < 2 {
			return View{}, ErrTooFewPoints
		}
	}

	// Corrida importada: descarta se já existe (mesma ref) ou se colide no tempo
	// com uma corrida nativa (mesma sessão gravada nos dois lugares).
	if in.ImportRef != "" {
		dup, err := s.store.duplicateImport(ctx, userID, in.ImportRef, in.StartedAt)
		if err != nil {
			return View{}, err
		}
		if dup {
			return View{}, ErrDuplicateRun
		}
	}

	// Map-matching (plano §11): encaixa o traçado na malha viária para o
	// território seguir os quarteirões reais. Só afeta a geometria gravada —
	// métricas e anti-fraude continuam sobre o traçado cru do GPS. Falha é
	// silenciosa (segue com o traçado cru).
	if s.matcher.Enabled() {
		src := make([]mapmatch.LonLat, len(c.points))
		for i, p := range c.points {
			src[i] = mapmatch.LonLat{p.Lon, p.Lat}
		}
		if snapped, _ := s.matcher.Match(ctx, src); len(snapped) >= 2 {
			c.matched = make([]Point, len(snapped))
			for i, ll := range snapped {
				c.matched[i] = Point{Lon: ll[0], Lat: ll[1]}
			}
		}
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
