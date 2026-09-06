package challenge

import "time"

// fortaleza — UTC-3 fixo (o Ceará não tem horário de verão desde 2019).
var fortaleza = time.FixedZone("America/Fortaleza", -3*3600)

// window é o intervalo semiaberto [Start, End) de um período, com seu número.
type window struct {
	No    int
	Start time.Time
	End   time.Time
}

// periodFor devolve o período que contém `at` para a cadência dada.
// `anchor` só é usado no caso quinzenal.
func periodFor(cadence string, anchor time.Time, at time.Time) (window, bool) {
	at = at.In(fortaleza)
	switch cadence {
	case "weekly":
		// segunda-feira 00:00 local
		wd := (int(at.Weekday()) + 6) % 7 // 0 = segunda
		start := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, fortaleza).AddDate(0, 0, -wd)
		end := start.AddDate(0, 0, 7)
		epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, fortaleza) // segunda
		return window{No: int(start.Sub(epoch).Hours()) / (24 * 7), Start: start, End: end}, true
	case "biweekly":
		a := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, fortaleza)
		days := int(at.Sub(a).Hours()) / 24
		if days < 0 {
			days = 0
		}
		no := days / 14
		start := a.AddDate(0, 0, no*14)
		return window{No: no, Start: start, End: start.AddDate(0, 0, 14)}, true
	case "monthly":
		start := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, fortaleza)
		end := start.AddDate(0, 1, 0)
		return window{No: at.Year()*12 + int(at.Month()) - 1, Start: start, End: end}, true
	default: // oneoff: não é gerado automaticamente
		return window{}, false
	}
}
