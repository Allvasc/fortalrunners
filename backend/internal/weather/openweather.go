package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// openWeatherProvider consome a One Call API 3.0 da OpenWeather (tier grátis:
// 1000 chamadas/dia). Em qualquer falha o Handler cai no staticProvider.
type openWeatherProvider struct {
	key    string
	client *http.Client
}

type owResponse struct {
	Current struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
		UVI       float64 `json:"uvi"`
		WindSpeed float64 `json:"wind_speed"` // m/s
		Sunrise   int64   `json:"sunrise"`
		Sunset    int64   `json:"sunset"`
	} `json:"current"`
	Timezone string `json:"timezone"`
}

func (p openWeatherProvider) Current(ctx context.Context, lat, lng float64) (*Report, error) {
	url := fmt.Sprintf(
		"https://api.openweathermap.org/data/3.0/onecall?lat=%.4f&lon=%.4f&appid=%s&units=metric&exclude=minutely,hourly,daily,alerts",
		lat, lng, p.key,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openweather status %d", res.StatusCode)
	}

	var ow owResponse
	if err := json.NewDecoder(res.Body).Decode(&ow); err != nil {
		return nil, err
	}

	loc, _ := time.LoadLocation(ow.Timezone)
	if loc == nil {
		loc = time.UTC
	}
	hhmm := func(unix int64) string {
		if unix == 0 {
			return ""
		}
		return time.Unix(unix, 0).In(loc).Format("15:04")
	}

	windKmh := round1(ow.Current.WindSpeed * 3.6)
	rep := &Report{
		City: "Fortaleza, CE", Lat: lat, Lng: lng,
		TempC:       round1(ow.Current.Temp),
		FeelsLikeC:  round1(ow.Current.FeelsLike),
		HumidityPct: ow.Current.Humidity,
		UVIndex:     round1(ow.Current.UVI),
		WindKmh:     windKmh,
		Sunrise:     hhmm(ow.Current.Sunrise),
		Sunset:      hhmm(ow.Current.Sunset),
		BestWindows: bestWindows(hhmm(ow.Current.Sunrise), hhmm(ow.Current.Sunset)),
		Advice:      advice(round1(ow.Current.FeelsLike), round1(ow.Current.UVI), windKmh),
		Source:      "OpenWeather", ReadAt: time.Now().UTC(),
	}
	return rep, nil
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

func bestWindows(sunrise, sunset string) []string {
	sr, ss := sunrise, sunset
	if sr == "" {
		sr = "05:30"
	}
	if ss == "" {
		ss = "17:40"
	}
	return []string{
		sr + "–07:30 — temperatura amena e brisa marítima",
		"até " + ss + " o sol ainda castiga; depois, sem UV e pôr do sol na orla",
	}
}

func advice(feels, uv, wind float64) string {
	switch {
	case feels >= 34:
		return "Sensação alta: encurte o treino, hidrate a cada 15 min e evite o sol das 10h às 16h."
	case uv >= 8:
		return "UV muito alto. Protetor 50+, boné e óculos; prefira trechos sombreados."
	case wind >= 30:
		return "Vento forte na orla. Comece contra o vento e volte a favor."
	default:
		return "Condição boa para correr. Protetor solar e hidratação a cada 20 min."
	}
}
