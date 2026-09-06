import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type ChallengeView } from "../lib/api";
import {
  cadenceLabel,
  metricLabel,
  metricValue,
  relativeDeadline,
} from "../lib/format";

export function Challenges() {
  const q = useQuery({ queryKey: ["challenges"], queryFn: api.challenges });

  return (
    <div className="page">
      <header className="page-head">
        <h1>Desafios</h1>
        <p className="muted">
          Competições com janela fechada — semana, quinzena, mês. Rendem selo e posição.{" "}
          <strong>Não mexem no seu território.</strong>
        </p>
      </header>

      {q.isLoading && <p className="muted">Carregando…</p>}
      {q.isError && <p className="err">Não foi possível carregar os desafios.</p>}

      <div className="chl-grid">
        {q.data?.challenges.map((c) => <ChallengeCard key={c.slug} c={c} />)}
      </div>
      {q.data && q.data.challenges.length === 0 && (
        <p className="muted">Nenhum desafio ativo no momento.</p>
      )}
    </div>
  );
}

function ChallengeCard({ c }: { c: ChallengeView }) {
  const [open, setOpen] = useState(false);
  const pct = c.goal ? Math.min(100, Math.round((c.my_value / c.goal) * 100)) : null;

  return (
    <article className="chl-card">
      <div className="chl-top">
        <span className="chip">{cadenceLabel(c.cadence)}</span>
        <span className="muted small">{relativeDeadline(c.ends_at)}</span>
      </div>
      <h3>{c.title}</h3>
      <p className="muted small">{c.description}</p>

      <div className="chl-mine">
        <div>
          <span className="v">{metricValue(c.metric, c.my_value)}</span>
          <span className="l muted small">
            seu total · {metricLabel(c.metric)}
            {c.my_rank > 0 && ` · ${c.my_rank}º`}
          </span>
        </div>
        {c.my_completed && <span className="chip done">meta batida</span>}
      </div>

      {pct !== null && (
        <div className="bar" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
          <i style={{ width: `${pct}%` }} />
        </div>
      )}

      <button className="btn quiet sm" onClick={() => setOpen((v) => !v)}>
        {open ? "Ocultar ranking" : "Ver ranking do período"}
      </button>
      {open && <ChallengeBoard slug={c.slug} />}
    </article>
  );
}

function ChallengeBoard({ slug }: { slug: string }) {
  const q = useQuery({
    queryKey: ["challenge-lb", slug],
    queryFn: () => api.challengeLeaderboard(slug),
  });

  if (q.isLoading) return <p className="muted small">Carregando…</p>;
  if (q.isError || !q.data) return <p className="err small">Falha ao carregar.</p>;

  return (
    <div className="chl-board">
      <p className="muted small">
        Período {q.data.period_no} · {q.data.closed ? "encerrado" : "em andamento"}
      </p>
      <ol className="lb tight">
        {q.data.entries.slice(0, 10).map((s) => (
          <li key={s.user_id}>
            <div className="lb-row">
              <span className="rk">{s.rank}</span>
              <span className="nm">{s.username}</span>
              {s.completed && <span className="tag">✓</span>}
            </div>
          </li>
        ))}
      </ol>
      {q.data.entries.length === 0 && <p className="muted small">Ninguém pontuou ainda.</p>}
    </div>
  );
}
