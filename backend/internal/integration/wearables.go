package integration

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
)

// A ponte de saúde (Apple Health / Health Connect) roda NO APARELHO: o app lê o
// treino — incluindo o trajeto quando disponível — e o envia por aqui. Passa pelo
// MESMO pipeline de território das corridas nativas, marcado data_source=import,
// com dedup por import_ref (plano §7).

type healthPoint struct {
	Lat float64  `json:"lat"`
	Lng float64  `json:"lng"`
	Alt *float64 `json:"alt,omitempty"`
	T   int64    `json:"t"` // epoch ms
}

type HealthSyncReq struct {
	Provider   string        `json:"provider"` // apple_health | health_connect
	ExternalID string        `json:"external_id"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at"`
	Points     []healthPoint `json:"points"`
	Cadence    []run.Sample  `json:"cadence,omitempty"`
	HeartRate  []run.Sample  `json:"heart_rate,omitempty"`
}

// IngestHealth encaminha um treino de saúde para o pipeline de corrida.
func (s *Service) IngestHealth(c echo.Context) error {
	userID := auth.UserID(c)
	var req HealthSyncReq
	if err := c.Bind(&req); err != nil || req.Provider == "" || req.ExternalID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider, external_id e trajeto são obrigatórios")
	}
	if req.Provider != "apple_health" && req.Provider != "health_connect" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider não suportado")
	}
	if len(req.Points) < 10 {
		return echo.NewHTTPError(http.StatusUnprocessableEntity,
			"treino sem trajeto GPS — não gera território (registre a corrida pelo app)")
	}

	pts := make([]run.Point, 0, len(req.Points))
	for _, p := range req.Points {
		pts = append(pts, run.Point{Lat: p.Lat, Lon: p.Lng, Alt: p.Alt, T: p.T / 1000})
	}

	in := run.IngestInput{
		StartedAt:  req.StartedAt,
		EndedAt:    req.EndedAt,
		DataSource: "import",
		GNSSMode:   "import",
		Points:     pts,
		Cadence:    req.Cadence,
		HeartRate:  req.HeartRate,
		ImportRef:  req.Provider + ":" + req.ExternalID,
	}

	v, err := s.runs.Ingest(c.Request().Context(), userID, in)
	if errors.Is(err, run.ErrDuplicateRun) {
		return c.JSON(http.StatusOK, map[string]any{"status": "duplicate", "run_id": ""})
	}
	if errors.Is(err, run.ErrTooFewPoints) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "trajeto insuficiente")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao importar treino")
	}
	return c.JSON(http.StatusAccepted, map[string]any{"status": "imported", "run": v})
}
