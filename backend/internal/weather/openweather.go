package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// openWeatherProvider usa a API clássica 2.5 da OpenWeather (tier grátis, sem
// assinatura extra): /weather para o tempo atual e /uvi para o índice UV.
// Em qualquer falha o Handler cai no staticProvider.
type openWeatherProvider struct {
	key    string
	client *http.Client
}

type owWeather struct {
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"` // m/s
	} `json:"wind"`
	Sys struct {
		Sunrise int64 `json:"sunrise"`
		Sunset  int64 `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"` // offset em segundos
	Name     string `json:"name"`
}

type owUVI struct {
	Value float64 `json:"value"`
}

func (p openWeatherProvider) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("openweather status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (p openWeatherProvider) Current(ctx context.Context, lat, lng float64) (*Report, error) {
	var w owWeather
	if err := p.getJSON(ctx, fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?lat=%.4f&lon=%.4f&appid=%s&units=metric&lang=pt_br",
		lat, lng, p.key,
	), &w); err != nil {
		return nil, err
	}

	// UV é um endpoint separado; se falhar, segue sem UV (0).
	var uvi owUVI
	_ = p.getJSON(ctx, fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/uvi?lat=%.4f&lon=%.4f&appid=%s", lat, lng, p.key,
	), &uvi)

	loc := time.FixedZone("local", w.Timezone)
	hhmm := func(unix int64) string {
		if unix == 0 {
			return ""
		}
		return time.Unix(unix, 0).In(loc).Format("15:04")
	}

	windKmh := round1(w.Wind.Speed * 3.6)
	feels := round1(w.Main.FeelsLike)
	uv := round1(uvi.Value)

	return &Report{
		City: "Fortaleza, CE", Lat: lat, Lng: lng,
		TempC:       round1(w.Main.Temp),
		FeelsLikeC:  feels,
		HumidityPct: w.Main.Humidity,
		UVIndex:     uv,
		WindKmh:     windKmh,
		Sunrise:     hhmm(w.Sys.Sunrise),
		Sunset:      hhmm(w.Sys.Sunset),
		BestWindows: bestWindows(hhmm(w.Sys.Sunrise), hhmm(w.Sys.Sunset)),
		Advice:      advice(feels, uv, windKmh),
		Source:      "OpenWeather", ReadAt: time.Now().UTC(),
	}, nil
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
