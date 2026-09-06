package weather

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/weather/current", h.current)
}

type WeatherReport struct {
	City        string   `json:"city"`
	TempC       float64  `json:"temp_c"`
	FeelsLikeC  float64  `json:"feels_like_c"`
	HumidityPct int      `json:"humidity_pct"`
	UVIndex     float64  `json:"uv_index"`
	WindKmh     float64  `json:"wind_kmh"`
	BestWindows []string `json:"best_windows"`
	Advice      string   `json:"advice"`
}

func (h *Handler) current(c echo.Context) error {
	report := WeatherReport{
		City:        "Fortaleza, CE",
		TempC:       28.5,
		FeelsLikeC:  31.0,
		HumidityPct: 74,
		UVIndex:     7.2,
		WindKmh:     19.5,
		BestWindows: []string{
			"05:30 – 07:30 (Temperatura amena e brisa marítima)",
			"17:15 – 19:00 (Sem incidência de raios UV e pôr do sol na orla)",
		},
		Advice: "Ótima brisa de sudeste na Beira-Mar. Recomenda-se protetor solar e hidratação regular a cada 20 minutos.",
	}

	return c.JSON(http.StatusOK, report)
}
