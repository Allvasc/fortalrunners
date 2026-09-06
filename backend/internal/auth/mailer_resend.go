package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ResendMailer envia e-mail transacional pela API da Resend (https://resend.com).
// Plano gratuito cobre o volume de MVP. Sem apiKey → NewMailer devolve nil e o
// Service cai no logMailer.
type ResendMailer struct {
	apiKey string
	from   string
	client *http.Client
}

// NewMailer devolve um Mailer real quando apiKey != "", senão nil.
// `from` deve ser um remetente verificado na Resend (ex.: "FortalRunners <nao-responda@fortalrunners.com>").
func NewMailer(apiKey, from string) Mailer {
	if apiKey == "" {
		return nil
	}
	if from == "" {
		from = "FortalRunners <onboarding@resend.dev>"
	}
	return &ResendMailer{apiKey: apiKey, from: from, client: &http.Client{Timeout: 8 * time.Second}}
}

func (m *ResendMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	subject := "Redefinição de senha — FortalRunners"
	text := "Você pediu para redefinir sua senha.\n\nAbra este link (vale por 30 minutos):\n" + link +
		"\n\nSe não foi você, ignore este e-mail — sua senha continua a mesma."
	html := fmt.Sprintf(`<div style="font-family:system-ui,sans-serif;max-width:480px;margin:0 auto;color:#16262A">
<h2 style="color:#0C7F86">Redefinição de senha</h2>
<p>Você pediu para redefinir sua senha no FortalRunners.</p>
<p><a href="%s" style="display:inline-block;background:#0C7F86;color:#fff;padding:10px 18px;border-radius:8px;text-decoration:none">Criar nova senha</a></p>
<p style="color:#7B8688;font-size:13px">O link vale por 30 minutos. Se não foi você, ignore este e-mail.</p>
</div>`, link)

	body, _ := json.Marshal(map[string]any{
		"from": m.from, "to": []string{to}, "subject": subject, "text": text, "html": html,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("resend: status %d", res.StatusCode)
	}
	return nil
}
