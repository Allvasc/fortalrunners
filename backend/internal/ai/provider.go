package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNoProvider: sem chave configurada — o serviço cai no fallback determinístico.
var ErrNoProvider = errors.New("nenhum provedor de IA configurado")

// Provider é a porta do LLM (plano §17). O domínio nunca chama o SDK direto.
type Provider interface {
	// Complete recebe instrução (system) e dados/pergunta (user) já separados.
	Complete(ctx context.Context, system, user string) (text string, inTok, outTok int, err error)
	Name() string
}

// --- Anthropic (Claude) — provedor padrão ---

type anthropicProvider struct {
	key   string
	model string
	http  *http.Client
}

func newAnthropicProvider(key, model string) *anthropicProvider {
	if model == "" {
		model = "claude-sonnet-5"
	}
	return &anthropicProvider{key: key, model: model, http: &http.Client{Timeout: 20 * time.Second}}
}

func (p *anthropicProvider) Name() string { return "anthropic:" + p.model }

func (p *anthropicProvider) Complete(ctx context.Context, system, user string) (string, int, int, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      p.model,
		"max_tokens": 700,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", p.key)
	req.Header.Set("anthropic-version", "2023-06-01")

	res, err := p.http.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("anthropic %d", res.StatusCode)
	}

	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, 0, err
	}
	txt := ""
	for _, c := range out.Content {
		txt += c.Text
	}
	if txt == "" {
		return "", 0, 0, errors.New("resposta vazia do provedor")
	}
	return txt, out.Usage.InputTokens, out.Usage.OutputTokens, nil
}
