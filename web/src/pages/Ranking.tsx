import { useQuery } from "@tanstack/react-query";
import { api, type LeaderEntry } from "../lib/api";
import { area, int } from "../lib/format";

export function Ranking() {
  const q = useQuery({ queryKey: ["leaderboard", "global"], queryFn: api.leaderboardGlobal });
  const cov = useQuery({ queryKey: ["coverage"], queryFn: api.coverage });

  return (
    <div className="page">
      <header className="page-head">
        <h1>Ranking</h1>
        <p className="muted">
          Classificação geral de Fortaleza por área total coberta. Território é permanente — a
          posição reflete tudo que você já conquistou, não uma semana.
        </p>
      </header>

      {cov.data && (
        <section>
          <h2>Sua cobertura de Fortaleza</h2>
          <p className="muted">
            {cov.data.city.pct}% da cidade — {int(cov.data.city.covered_cells)} de{" "}
            {int(cov.data.city.total_cells)} células.
          </p>
          <ul className="lb tight">
            {[...cov.data.neighborhoods]
              .sort((a, b) => b.pct - a.pct)
              .map((n) => (
                <li key={n.neighborhood_id}>
                  <div className="lb-row">
                    <span className="nm">{n.name}</span>
                    <span className="mv">{n.pct}%</span>
                  </div>
                </li>
              ))}
          </ul>
        </section>
      )}

      {q.isLoading && <p className="muted">Carregando…</p>}
      {q.isError && <p className="err">Não foi possível carregar o ranking.</p>}

      {q.data && (
        <>
          {q.data.me && !q.data.entries.some((e) => e.user_id === q.data!.me!.user_id) && (
            <Row e={q.data.me} mine />
          )}
          <ol className="lb">
            {q.data.entries.map((e) => (
              <li key={e.user_id}>
                <Row e={e} mine={e.user_id === q.data!.me?.user_id} />
              </li>
            ))}
          </ol>
          {q.data.entries.length === 0 && (
            <p className="muted">Ninguém conquistou território ainda. Seja o primeiro.</p>
          )}
        </>
      )}
    </div>
  );
}

function Row({ e, mine }: { e: LeaderEntry; mine?: boolean }) {
  return (
    <div className={`lb-row${mine ? " mine" : ""}`}>
      <span className="rk">{e.rank}</span>
      <span className="nm">
        {e.username}
        {mine && <span className="tag">você</span>}
      </span>
      <span className="blocks muted small">{int(e.blocks)} quart.</span>
      <span className="mv">{area(e.area_m2)}</span>
    </div>
  );
}
