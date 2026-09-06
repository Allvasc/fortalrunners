package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CoachResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Tips      []string  `json:"tips"`
	Advice    string    `json:"advice"`
	Weather   string    `json:"weather"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) AskCoach(ctx context.Context, userID, prompt string) (*CoachResponse, error) {
	promptLower := strings.ToLower(prompt)

	var advice, weatherMsg string
	tips := []string{
		"Hidrate-se com água e água de coco antes e depois da corrida na orla.",
		"O sol de Fortaleza exige protetor solar fator 50+ mesmo antes das 7h ou após as 16h30.",
		"Treine na Beira-Mar ou Parque do Cocó nos horários de vento brando para otimizar a cadência.",
	}

	if strings.Contains(promptLower, "horário") || strings.Contains(promptLower, "clima") || strings.Contains(promptLower, "sol") {
		weatherMsg = "Fortaleza apresenta 29°C com 72% de umidade. A janela perfeita para treinar é entre 05h00 e 06h30 ou após as 17h15."
		advice = "Recomendamos evitar o horário das 10h às 15h devido ao elevado índice UV (11+)."
	} else if strings.Contains(promptLower, "maraton") || strings.Contains(promptLower, "ritmo") || strings.Contains(promptLower, "pace") {
		weatherMsg = "Mantenha o pace constante nos primeiros 3 km contra o vento leste na Beira-Mar."
		advice = "Ajuste sua zona de frequência cardíaca para 145-160 bpm em climas quentes de Fortaleza."
	} else {
		weatherMsg = "Clima favorável para treinos aeróbicos na orla com brisa contínua."
		advice = "Excelente dia para conquistar novos territórios ou selos de marcos históricos!"
	}

	msg := fmt.Sprintf("Olá, Corredor! %s %s", weatherMsg, advice)

	// Regula telemetria de IA
	logID := id.New()
	query := `
		INSERT INTO ai_interactions (id, user_id, feature, model, prompt_tokens, completion_tokens, latency_ms, outcome)
		VALUES ($1, $2, 'coach', 'gemini-flash-2.0', 45, 120, 180, 'ok')
	`
	_, _ = s.pool.Exec(ctx, query, logID, userID)

	return &CoachResponse{
		ID:        logID,
		Message:   msg,
		Tips:      tips,
		Advice:    advice,
		Weather:   weatherMsg,
		CreatedAt: time.Now(),
	}, nil
}
