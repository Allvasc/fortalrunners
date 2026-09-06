import React, { useState } from "react";
import { api, AICoachResponse } from "../lib/api";

export const AICoachPage: React.FC = () => {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<AICoachResponse | null>(null);
  const [err, setErr] = useState("");

  async function handleAsk(e: React.FormEvent) {
    e.preventDefault();
    if (!prompt.trim()) return;
    setLoading(true);
    setErr("");
    try {
      const res = await api.askAICoach(prompt.trim());
      setResponse(res.coach);
    } catch (e: any) {
      setErr(e?.message ?? "Coach indisponível no momento.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page">
      <header className="page-head">
        <h1>Coach IA</h1>
        <p className="muted">
          Pergunte sobre o seu treino em Fortaleza — clima, horários, ritmo. As respostas
          usam <strong>só os seus dados</strong> e são sugestões, não prescrição.
        </p>
      </header>

      <form
        onSubmit={handleAsk}
        className="card"
        style={{ flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: "0.6rem" }}
      >
        <input
          type="text"
          value={prompt}
          maxLength={1000}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="Ex.: qual o melhor horário pra correr na Beira-Mar hoje?"
          style={{ flex: "1 1 240px", minWidth: 0 }}
        />
        <button type="submit" className="btn primary" disabled={loading}>
          {loading ? "Consultando…" : "Perguntar"}
        </button>
      </form>

      {err && <p className="err">{err}</p>}

      {response && (
        <article className="card" style={{ display: "grid", gap: "0.9rem" }}>
          <p style={{ margin: 0, lineHeight: 1.6 }}>{response.message}</p>

          {response.tips?.length > 0 && (
            <div>
              <p className="small mono muted" style={{ textTransform: "uppercase", letterSpacing: "0.08em" }}>
                Dicas
              </p>
              <ul style={{ margin: "0.3rem 0 0", paddingLeft: "1.1rem" }}>
                {response.tips.map((t, i) => (
                  <li key={i} style={{ fontSize: "0.9rem" }}>{t}</li>
                ))}
              </ul>
            </div>
          )}

          {response.disclaimer && (
            <p className="small muted" style={{ borderLeft: "3px solid var(--sun)", paddingLeft: "0.7rem" }}>
              {response.disclaimer}
            </p>
          )}

          <p className="small mono muted" style={{ display: "flex", justifyContent: "space-between", gap: "1rem" }}>
            <span>{response.source === "ia" ? "gerado por IA" : "resposta padrão (IA indisponível)"}</span>
            <span>{new Date(response.created_at).toLocaleTimeString("pt-BR")}</span>
          </p>
        </article>
      )}
    </div>
  );
};
