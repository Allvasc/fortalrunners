package integration

import "testing"

func sampleStreams(n int) stravaStreams {
	var s stravaStreams
	lat := -3.73
	for i := 0; i < n; i++ {
		s.LatLng = append(s.LatLng, []float64{lat, -38.5})
		s.Time = append(s.Time, i*10)
		s.Altitude = append(s.Altitude, 10+float64(i))
		lat += 0.0009
	}
	return s
}

func TestMapActivityRun(t *testing.T) {
	a := stravaActivity{
		ID: 42, Type: "Run", StartDate: "2026-09-05T10:00:00Z",
		Elapsed: 600, Moving: 590, Distance: 2000,
	}
	in, ok := mapActivity(a, sampleStreams(20))
	if !ok {
		t.Fatal("Run com GPS deveria mapear")
	}
	if in.DataSource != "import" {
		t.Fatalf("data_source = %q", in.DataSource)
	}
	if len(in.Points) != 20 {
		t.Fatalf("pontos = %d", len(in.Points))
	}
	if in.Points[0].Alt == nil || *in.Points[0].Alt != 10 {
		t.Fatal("altitude não veio do stream")
	}
	// t do primeiro ponto = start em ms (2026-09-05T10:00:00Z)
	if in.Points[0].T != 1788602400000 {
		t.Fatalf("t[0] = %d", in.Points[0].T)
	}
	// 10 s entre pontos
	if in.Points[1].T-in.Points[0].T != 10000 {
		t.Fatalf("delta t = %d", in.Points[1].T-in.Points[0].T)
	}
	if in.EndedAt.Sub(in.StartedAt).Seconds() != 600 {
		t.Fatal("janela != elapsed")
	}
}

func TestMapActivityRejects(t *testing.T) {
	base := stravaActivity{ID: 1, Type: "Run", StartDate: "2026-09-05T10:00:00Z", Elapsed: 100}

	if _, ok := mapActivity(stravaActivity{ID: 2, Type: "Ride", StartDate: base.StartDate}, sampleStreams(10)); ok {
		t.Error("Ride não deveria mapear")
	}
	if _, ok := mapActivity(base, sampleStreams(1)); ok {
		t.Error("1 ponto não deveria mapear")
	}
	manual := base
	manual.Manual = true
	if _, ok := mapActivity(manual, sampleStreams(10)); ok {
		t.Error("atividade manual (sem GPS) não deveria mapear")
	}
	// sport_type = Run mesmo com type vazio
	if _, ok := mapActivity(stravaActivity{ID: 3, SportType: "Run", StartDate: base.StartDate, Elapsed: 100}, sampleStreams(10)); !ok {
		t.Error("sport_type Run deveria mapear")
	}
}

func TestStravaConfigEnabled(t *testing.T) {
	if (StravaConfig{}).enabled() {
		t.Error("config vazia não está habilitada")
	}
	if !(StravaConfig{ClientID: "x", ClientSecret: "y"}).enabled() {
		t.Error("config com id+secret está habilitada")
	}
}

func TestStravaAuthURL(t *testing.T) {
	c := newStrava(StravaConfig{ClientID: "123", RedirectURL: "https://fr.com/cb"}, nil)
	u := c.authURL("st4te")
	for _, w := range []string{"strava.com/oauth/authorize", "client_id=123", "activity%3Aread_all", "state=st4te"} {
		if !contains(u, w) {
			t.Errorf("authURL sem %q: %s", w, u)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
