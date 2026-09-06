package run

// Anti-fraude v1 (plano §8): física do movimento + sensores cruzados + sinais do
// device + padrões do traçado. Score 0..1; acima do limiar a corrida entra em
// revisão humana (status flagged) — nunca é rejeitada automaticamente.

const (
	fraudFlagThreshold = 0.5 // >= isto => flagged
	suspiciousPaceS    = 150 // < 2:30/km sustentado
	minStrideM         = 0.4
	maxStrideM         = 3.0
)

type fraudResult struct {
	Score float64  `json:"score"`
	Flags []string `json:"flags"`
}

func (f fraudResult) flagged() bool { return f.Score >= fraudFlagThreshold }

// scoreFraud combina os sinais. Cada regra soma no score (saturado em 1).
func scoreFraud(in IngestInput, c cleaned, m Metrics) fraudResult {
	var r fraudResult
	add := func(w float64, flag string) {
		r.Score += w
		r.Flags = append(r.Flags, flag)
	}

	// 1. Física — pace sustentado impossível a pé.
	if c.distM > 0 {
		pace := c.movingS / (c.distM / 1000.0)
		if pace > 0 && pace < suspiciousPaceS {
			add(0.8, "pace_impossible")
		}
	}

	// 2. Sinal do device — localização simulada.
	if in.MockLocation {
		add(0.9, "mock_location")
	}

	// 3. Física — muitos saltos de teleporte (spoofing "salta" entre pontos).
	if c.teleports >= 3 || (c.rawKept > 0 && float64(c.teleports)/float64(c.rawKept) > 0.1) {
		add(0.4, "teleport_jumps")
	}

	// 4. Sensores cruzados — passada implícita fora da faixa humana.
	if m.HasCadence && m.StepCount > 0 {
		stride := c.distM / float64(m.StepCount)
		if stride < minStrideM || stride > maxStrideM {
			add(0.3, "stride_out_of_range")
		}
	}

	// 5. Padrão — traçado reto demais para uma corrida real de rua.
	if c.distM > 2000 {
		if diag := c.bboxDiagonalM(); diag > 0 && c.distM/diag < 1.15 {
			add(0.25, "unnaturally_straight")
		}
	}

	// 6. Integridade — corrida de celular sem attestation do app.
	if in.DataSource == "phone" && in.Attestation == "" {
		add(0.1, "no_attestation")
	}

	if r.Score > 1 {
		r.Score = 1
	}
	return r
}
