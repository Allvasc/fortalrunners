// Package heatmap serve o mapa de calor ("onde se corre") a partir da tabela
// heat_agg, e mantém essa agregação atualizada por um job do scheduler.
//
// Fase 1: só o mapa de calor pessoal (scope=me). Amigos/clube/cidade + o
// k-anonimato da cidade entram na Fase 2/3 (ver plano §7 e §19).
package heatmap

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// cityKAnon: numa célula de cidade, só servir com pelo menos N corredores
// distintos (lição do vazamento do Strava 2018). Não se aplica a scope=me.
const cityKAnon = 3

var ErrScopeUnsupported = errors.New("escopo de mapa de calor ainda não disponível")

type Service struct {
	store *store
	log   *slog.Logger
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{store: newStore(pool), log: log.With("svc", "heatmap")}
}

// Refresh recalcula o heat_agg dos corredores com corrida nova. Chamado pelo
// scheduler.
func (s *Service) Refresh(ctx context.Context) error {
	users, err := s.store.usersToRefresh(ctx, 200)
	if err != nil {
		return err
	}
	for _, u := range users {
		if err := s.store.refreshUser(ctx, u); err != nil {
			s.log.Error("refreshUser", "user_id", u, "err", err)
			continue
		}
	}
	if len(users) > 0 {
		s.log.Info("heatmap atualizado", "users", len(users))
	}
	return nil
}

// Query devolve o GeoJSON do mapa de calor. scope: "me" (Fase 1).
func (s *Service) Query(ctx context.Context, userID, scope, period, activity string, bbox [4]float64) (string, error) {
	if period == "" {
		period = "all"
	}
	if activity == "" {
		activity = "run"
	}
	switch scope {
	case "", "me":
		return s.store.featureCollection(ctx, "user:"+userID, period, activity, bbox, 1)
	default:
		// friends / city: Fase 2/3.
		return "", ErrScopeUnsupported
	}
}
