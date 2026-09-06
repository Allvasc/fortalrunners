// Package weather serve as condições do dia (plano §6, §10: GET /v1/conditions).
// Sem WEATHER_API_KEY opera com leitura estática de Fortaleza; a interface
// Provider já está pronta para plugar FUNCEME/INMET/OpenWeather depois.
package weather

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type Report struct {
	City        string    `json:"city"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	TempC       float64   `json:"temp_c"`
	FeelsLikeC  float64   `json:"feels_like_c"`
	HumidityPct int       `json:"humidity_pct"`
	UVIndex     float64   `json:"uv_index"`
	WindKmh     float64   `json:"wind_kmh"`
	Sunrise     string    `json:"sunrise"`
	Sunset      string    `json:"sunset"`
	BestWindows []string  `json:"best_windows"`
	Advice      string    `json:"advice"`
	Source      string    `json:"source"`
	ReadAt      time.Time `json:"read_at"`
}

// Provider é a porta de um provedor de clima real.
type Provider interface {
	Current(ctx context.Context, lat, lng float64) (*Report, error)
}

// staticProvider: leitura fixa de Fortaleza, marcada como estática.
type staticProvider struct{}

func (staticProvider) Current(_ context.Context, lat, lng float64) (*Report, error) {
	return &Report{
		City: "Fortaleza, CE", Lat: lat, Lng: lng,
		TempC: 28.5, FeelsLikeC: 31.0, HumidityPct: 74, UVIndex: 7.2, WindKmh: 19.5,
		Sunrise: "05:32", Sunset: "17:42",
		BestWindows: []string{
			"05:30–07:30 — temperatura amena e brisa marítima",
			"17:15–19:00 — sem UV e pôr do sol na orla",
		},
		Advice: "Brisa de sudeste na Beira-Mar. Protetor solar 50+ e hidratação a cada 20 min.",
		Source: "estática (Fortaleza)", ReadAt: time.Now().UTC(),
	}, nil
}

type Handler struct {
	provider Provider
}

// fallbackProvider tenta o provedor real e cai no estático em qualquer erro,
// para o endpoint nunca ficar indisponível por causa do clima.
type fallbackProvider struct {
	primary  Provider
	fallback Provider
}

func (f fallbackProvider) Current(ctx context.Context, lat, lng float64) (*Report, error) {
	if r, err := f.primary.Current(ctx, lat, lng); err == nil {
		return r, nil
	}
	return f.fallback.Current(ctx, lat, lng)
}

func NewHandler(apiKey string) *Handler {
	if apiKey == "" {
		return &Handler{provider: staticProvider{}}
	}
	real := openWeatherProvider{key: apiKey, client: &http.Client{Timeout: 4 * time.Second}}
	return &Handler{provider: fallbackProvider{primary: real, fallback: staticProvider{}}}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/conditions", h.current)
	g.GET("/weather/current", h.current) // alias
}

func (h *Handler) current(c echo.Context) error {
	lat, _ := strconv.ParseFloat(c.QueryParam("lat"), 64)
	lng, _ := strconv.ParseFloat(c.QueryParam("lon"), 64)
	if lng == 0 {
		lng, _ = strconv.ParseFloat(c.QueryParam("lng"), 64)
	}
	if lat == 0 {
		lat, lng = -3.7275, -38.5267 // centro de Fortaleza
	}
	rep, err := h.provider.Current(c.Request().Context(), lat, lng)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "provedor de clima indisponível")
	}
	return c.JSON(http.StatusOK, rep)
}
