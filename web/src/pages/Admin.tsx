import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type FlaggedRun } from "../lib/api";
import { clock, date, km, pace } from "../lib/format";

export function Admin() {
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });

  if (me.isLoading) {
    return <div className="page"><p className="muted">Carregando…</p></div>;
  }
  if (!me.data || (me.data.role !== "admin" && me.data.role !== "moderator")) {
    return (
      <div className="page">
        <header className="page-head"><h1>Admin</h1></header>
        <p className="muted">
          Esta área é só para contas com perfil <code>admin</code> ou <code>moderator</code>.
        </p>
      </div>
    );
  }

  return (
    <div className="page">
      <header className="page-head">
        <h1>Admin</h1>
        <p className="muted">
          Namespace isolado, exige 2FA. Toda ação fica na trilha de auditoria.
        </p>
      </header>
      <FlaggedRuns />
      <Moderation />
      <Financeiro />
      <RiskZones />
      <Users />
      <GameConfig />
      <Audit />
    </div>
  );
}

function Moderation() {
  const qc = useQueryClient();
  const checkins = useQuery({ queryKey: ["adm", "checkins"], queryFn: api.adminPendingCheckins });
  const reviews = useQuery({ queryKey: ["adm", "reviews"], queryFn: api.adminPendingReviews });
  const reports = useQuery({ queryKey: ["adm", "reports"], queryFn: api.adminReports });
  const hazards = useQuery({ queryKey: ["adm", "hazards"], queryFn: api.adminHazards });

  const inval = () => qc.invalidateQueries({ queryKey: ["adm"] });
  const mCheckin = useMutation({ mutationFn: (v: { id: string; d: "approve" | "reject" }) => api.adminModerateCheckin(v.id, v.d), onSuccess: inval });
  const mReview = useMutation({ mutationFn: (v: { id: string; d: "approve" | "reject" }) => api.adminModerateReview(v.id, v.d), onSuccess: inval });
  const mReport = useMutation({ mutationFn: (v: { id: string; o: "actioned" | "dismissed" }) => api.adminResolveReport(v.id, v.o), onSuccess: inval });
  const mHazard = useMutation({ mutationFn: (id: string) => api.adminRemoveHazard(id), onSuccess: inval });

  const cn = checkins.data?.checkins ?? [];
  const rv = reviews.data?.reviews ?? [];
  const rp = reports.data?.reports ?? [];
  const hz = (hazards.data?.hazards ?? []).filter((h) => h.status === "active");

  return (
    <section>
      <h2>Moderação</h2>

      <h3 className="muted small">Check-ins de marco ({cn.length})</h3>
      {cn.length === 0 && <p className="muted">Fila vazia.</p>}
      {cn.map((c) => (
        <div className="mod-row" key={c.id}>
          <span>
            <strong>{c.landmark_name}</strong> · {c.username} · {date(c.taken_at)}
          </span>
          <span className="mod-actions">
            <button className="btn sm primary" onClick={() => mCheckin.mutate({ id: c.id, d: "approve" })}>Aprovar</button>
            <button className="btn sm" onClick={() => mCheckin.mutate({ id: c.id, d: "reject" })}>Rejeitar</button>
          </span>
        </div>
      ))}

      <h3 className="muted small">Avaliações de rota ({rv.length})</h3>
      {rv.length === 0 && <p className="muted">Fila vazia.</p>}
      {rv.map((r) => (
        <div className="mod-row" key={r.id}>
          <span>
            <strong>{r.route_name}</strong> · {r.username} · ⭐{r.rating} · {r.body || "(sem texto)"}
          </span>
          <span className="mod-actions">
            <button className="btn sm primary" onClick={() => mReview.mutate({ id: r.id, d: "approve" })}>Aprovar</button>
            <button className="btn sm" onClick={() => mReview.mutate({ id: r.id, d: "reject" })}>Rejeitar</button>
          </span>
        </div>
      ))}

      <h3 className="muted small">Denúncias ({rp.length})</h3>
      {rp.length === 0 && <p className="muted">Nenhuma denúncia aberta.</p>}
      {rp.map((r) => (
        <div className="mod-row" key={r.id}>
          <span>
            <code>{r.target_type}</code> {r.target_id} · {r.reason} {r.detail && `— ${r.detail}`}
          </span>
          <span className="mod-actions">
            <button className="btn sm primary" onClick={() => mReport.mutate({ id: r.id, o: "actioned" })}>Tratada</button>
            <button className="btn sm" onClick={() => mReport.mutate({ id: r.id, o: "dismissed" })}>Descartar</button>
          </span>
        </div>
      ))}

      <h3 className="muted small">Marcações da via ativas ({hz.length})</h3>
      {hz.length === 0 && <p className="muted">Nenhuma.</p>}
      {hz.map((h) => (
        <div className="mod-row" key={h.id}>
          <span>
            <code>{h.type}</code> sev.{h.severity} · 👍{h.confirms} 👎{h.disputes} · {h.note || "(sem nota)"}
          </span>
          <span className="mod-actions">
            <button className="btn sm" onClick={() => mHazard.mutate(h.id)}>Remover</button>
          </span>
        </div>
      ))}
    </section>
  );
}

function Financeiro() {
  const refunds = useQuery({ queryKey: ["adm", "refunds"], queryFn: api.adminRefunds });
  const m = useMutation({
    mutationFn: (v: { id: string; d: "approve" | "deny" }) => api.adminDecideRefund(v.id, v.d),
    onSuccess: () => refunds.refetch(),
  });
  const list = refunds.data?.refunds ?? [];
  return (
    <section>
      <h2>Financeiro — reembolsos</h2>
      {list.length === 0 && <p className="muted">Nenhum reembolso solicitado.</p>}
      {list.map((r) => (
        <div className="mod-row" key={r.id}>
          <span>
            pedido <code>{r.order_id}</code> · R$ {(r.amount_cents / 100).toFixed(2)} · {r.reason || "sem motivo"} · {date(r.created_at)}
          </span>
          <span className="mod-actions">
            <button className="btn sm primary" disabled={m.isPending} onClick={() => m.mutate({ id: r.id, d: "approve" })}>Aprovar</button>
            <button className="btn sm" disabled={m.isPending} onClick={() => m.mutate({ id: r.id, d: "deny" })}>Negar</button>
          </span>
        </div>
      ))}
      
    </section>
  );
}

function RiskZones() {
  const zones = useQuery({ queryKey: ["adm", "zones"], queryFn: api.adminRiskZones });
  const cfg = useQuery({ queryKey: ["adm", "config"], queryFn: api.adminConfig });
  const del = useMutation({ mutationFn: (id: string) => api.adminDeleteRiskZone(id), onSuccess: () => zones.refetch() });
  const setFlag = useMutation({
    mutationFn: (on: boolean) => api.adminSetConfig("risk_zone_blocking", on),
    onSuccess: () => cfg.refetch(),
  });
  const list = zones.data?.features?.map((f) => f.properties) ?? [];
  const blocking = cfg.data?.risk_zone_blocking !== false;
  return (
    <section>
      <h2>Zonas de risco</h2>
      <label className="sw-row">
        <input type="checkbox" checked={blocking} onChange={(e) => setFlag.mutate(e.target.checked)} />
        <span>
          <code>RISK_ZONE_BLOCKING</code> — subtrair/recusar conquista em zona de risco
        </span>
      </label>
      {list.length === 0 && <p className="muted">Nenhuma zona ativa. (O CRUD completo — desenhar polígono — ainda é via API.)</p>}
      {list.map((z) => (
        <div className="mod-row" key={z.id}>
          <span>
            <code>{z.id.slice(0, 8)}</code> · severidade {z.severity} · {z.note || "(sem nota)"}
          </span>
          <span className="mod-actions">
            <button className="btn sm" onClick={() => del.mutate(z.id)}>Remover</button>
          </span>
        </div>
      ))}
    </section>
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
                <td className="mono">
                  {r.fraud_score.toFixed(2)}
                  {r.fraud_flags?.length > 0 && (
                    <span className="muted small"> · {r.fraud_flags.join(", ")}</span>
                  )}
                </td>
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
                  {u.role === "runner" && (
                    <button
                      className="btn quiet sm"
                      onClick={() => {
                        const name = prompt(`Nome da organização de ${u.username}:`);
                        if (!name) return;
                        const email = prompt("E-mail de contato:") ?? "";
                        api
                          .adminCreateOrganizer(u.id, name, email)
                          .then(() => qc.invalidateQueries({ queryKey: ["admin", "users"] }))
                          .catch((e) => alert(e?.message ?? "erro"));
                      }}
                    >
                      Tornar organizador
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
