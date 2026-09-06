import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type FlaggedRun } from "../lib/api";
import { clock, date, km, pace } from "../lib/format";

export function Admin() {
  return (
    <div className="page">
      <header className="page-head">
        <h1>Admin</h1>
        <p className="muted">
          Namespace isolado, exige 2FA. Toda ação fica na trilha de auditoria.
        </p>
      </header>
      <FlaggedRuns />
      <Users />
      <GameConfig />
      <Audit />
    </div>
  );
}

function FlaggedRuns() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["admin", "flagged"], queryFn: api.adminFlaggedRuns });
  const review = useMutation({
    mutationFn: (v: { id: string; decision: "valid" | "rejected"; voidT: boolean }) =>
      api.adminReviewRun(v.id, v.decision, v.voidT),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin"] }),
  });

  return (
    <section>
      <h2>Corridas sinalizadas</h2>
      {q.isLoading && <p className="muted">Carregando…</p>}
      {q.data?.runs.length === 0 && <p className="muted">Nada na fila.</p>}
      <div className="tw">
        <table>
          <thead>
            <tr>
              <th>Quando</th>
              <th>Corredor</th>
              <th>Dist.</th>
              <th>Pace</th>
              <th>Fraude</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {q.data?.runs.map((r: FlaggedRun) => (
              <tr key={r.id}>
                <td>{date(r.started_at)}</td>
                <td>{r.username}</td>
                <td className="mono">{km(r.distance_m)}</td>
                <td className="mono">
                  {pace(r.avg_pace_s)} <span className="muted">/ {clock(r.moving_s)}</span>
                </td>
                <td className="mono">{r.fraud_score.toFixed(2)}</td>
                <td className="row-btns">
                  <button
                    className="btn quiet sm"
                    onClick={() => review.mutate({ id: r.id, decision: "valid", voidT: false })}
                  >
                    Liberar
                  </button>
                  <button
                    className="btn ghost sm"
                    onClick={() => review.mutate({ id: r.id, decision: "rejected", voidT: true })}
                  >
                    Rejeitar
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function Users() {
  const qc = useQueryClient();
  const [term, setTerm] = useState("");
  const [q, setQ] = useState("");
  const list = useQuery({
    queryKey: ["admin", "users", q],
    queryFn: () => api.adminUsers(q),
    enabled: q.length > 0,
  });
  const setStatus = useMutation({
    mutationFn: (v: { id: string; status: string }) =>
      api.adminSetUserStatus(v.id, v.status, "via painel"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "users"] }),
  });

  return (
    <section>
      <h2>Usuários</h2>
      <form
        className="row-btns"
        onSubmit={(e) => {
          e.preventDefault();
          setQ(term.trim());
        }}
      >
        <input value={term} onChange={(e) => setTerm(e.target.value)} placeholder="username, e-mail ou FR-…" />
        <button className="btn primary sm">Buscar</button>
      </form>
      <div className="tw">
        <table>
          <thead>
            <tr>
              <th>Atleta</th>
              <th>Username</th>
              <th>Role</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {list.data?.users.map((u) => (
              <tr key={u.id}>
                <td className="mono">{u.athlete_id}</td>
                <td>{u.username}</td>
                <td>{u.role}</td>
                <td>{u.status}</td>
                <td className="row-btns">
                  {u.status === "active" ? (
                    <button
                      className="btn ghost sm"
                      onClick={() => setStatus.mutate({ id: u.id, status: "suspended" })}
                    >
                      Suspender
                    </button>
                  ) : (
                    <button
                      className="btn quiet sm"
                      onClick={() => setStatus.mutate({ id: u.id, status: "active" })}
                    >
                      Reativar
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function GameConfig() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["admin", "config"], queryFn: api.adminConfig });
  const set = useMutation({
    mutationFn: (v: { key: string; value: unknown }) => api.adminSetConfig(v.key, v.value),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "config"] }),
  });

  return (
    <section>
      <h2>Config do jogo</h2>
      {set.isError && (
        <p className="err">{set.error instanceof ApiError ? set.error.message : "falha ao salvar"}</p>
      )}
      <ul className="rec-list">
        {q.data &&
          Object.entries(q.data).map(([k, v]) => (
            <li key={k}>
              <span className="mono">{k}</span>
              <span className="mono">{JSON.stringify(v)}</span>
              {typeof v === "boolean" ? (
                <button className="btn quiet sm" onClick={() => set.mutate({ key: k, value: !v })}>
                  alternar
                </button>
              ) : (
                <button
                  className="btn quiet sm"
                  onClick={() => {
                    const n = prompt(`Novo valor para ${k}`, String(v));
                    if (n !== null && n.trim() !== "") set.mutate({ key: k, value: Number(n) });
                  }}
                >
                  editar
                </button>
              )}
            </li>
          ))}
      </ul>
    </section>
  );
}

function Audit() {
  const q = useQuery({ queryKey: ["admin", "audit"], queryFn: api.adminAudit });
  return (
    <section>
      <h2>Auditoria</h2>
      <ul className="rec-list">
        {q.data?.entries.map((e) => (
          <li key={e.id}>
            <span className="mono">{e.action}</span>
            <span className="muted small">
              {e.target_type} {e.target_id?.slice(0, 8) ?? ""}
            </span>
            <span className="muted small">{date(e.created_at)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
