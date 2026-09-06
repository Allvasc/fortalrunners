// Package territory contém o pipeline que transforma o traçado de uma corrida
// em território dominado (detecção de laço → polígono → gate de zona de risco →
// gravação). Roda no territory-worker, fora do caminho da requisição.
//
// Fase 1: cerco de área em terreno neutro. Resolução de conflito entre
// corredores e indexação H3 (cobertura %) entram na Fase 2.
package territory

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

type Processor struct {
	store *store
	log   *slog.Logger
}

func NewProcessor(pool *pgxpool.Pool, log *slog.Logger) *Processor {
	return &Processor{store: newStore(pool), log: log}
}

// Process consome um evento run.uploaded.
func (p *Processor) Process(ctx context.Context, runID string) {
	log := p.log.With("run_id", runID)

	ri, err := p.store.runInfo(ctx, runID)
	if err != nil {
		log.Error("territory: run não encontrado", "err", err)
		return
	}

	wkb, areaM2, parts, ok, err := p.store.polygonize(ctx, runID, p.store.riskConfig(ctx))
	if err != nil {
		log.Error("territory: polygonize falhou", "err", err)
		_ = p.store.finishRun(ctx, runID, "valid", 0, 0, "polygonize: "+err.Error())
		return
	}

	if !ok {
		// corrida válida, mas sem laço fechado (ou área abaixo do mínimo).
		_ = p.store.finishRun(ctx, runID, "valid", 0, 0, "")
		log.Info("territory: sem conquista", "area_m2", areaM2)
		return
	}

	terrID := id.New()
	if err := p.store.insertTerritory(ctx, terrID, ri.UserID, runID, wkb, areaM2); err != nil {
		log.Error("territory: insert falhou", "err", err)
		_ = p.store.finishRun(ctx, runID, "valid", 0, 0, "insert: "+err.Error())
		return
	}

	// indexa na grade de cobertura (h3_cells) — não fatal se falhar.
	if err := p.store.polyfillCells(ctx, terrID, ri.UserID); err != nil {
		log.Warn("territory: polyfill de cobertura falhou", "err", err)
	}

	if err := p.store.finishRun(ctx, runID, "valid", areaM2, parts, ""); err != nil {
		log.Error("territory: finishRun falhou", "err", err)
		return
	}

	log.Info("território conquistado",
		"territory_id", terrID, "area_m2", int(areaM2), "blocos", parts)
}
