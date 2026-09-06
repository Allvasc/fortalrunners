import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api, ApiError, type OrgEventInput } from "../lib/api";

/** Portal de organizadores (plano §12). Requer role organizer/admin + 2FA. */
export function Organizer() {
  const orgs = useQuery({ queryKey: ["orgMe"], queryFn: api.orgMe });
  const [eventId, setEventId] = useState<string | null>(null);
  const [msg, setMsg] = useState("");

  if (orgs.isLoading) return <div className="page"><p className="muted">Carregando…</p></div>;

  if (orgs.error instanceof ApiError && orgs.error.status === 404) {
    return (
      <div className="page">
        <header className="page-head"><h1>Organizadores</h1></header>
        <p className="muted">Esta área é só para contas com perfil <code>organizer</code>.</p>
      </div>
    );
  }

  const noOrg = (orgs.data?.organizers ?? []).length === 0;

  return (
    <div className="page">
      <header className="page-head">
        <h1>Portal do organizador</h1>
        <p className="muted">
          Crie eventos, monte lotes e estações, gere cupons e credenciais de scanner,
          acompanhe os inscritos. Eventos nascem em rascunho e vão para aprovação do admin.
        </p>
      </header>

      {msg && <p className="err" style={{ borderColor: "var(--teal)", color: "var(--ink)" }}>{msg}</p>}

      {noOrg ? (
        <p className="muted">
          Você ainda não tem nenhuma organização. Peça ao admin para criar uma organização
          vinculada à sua conta.
        </p>
      ) : (
        <section>
          <h2>Suas organizações</h2>
          <ul className="lb tight">
            {orgs.data!.organizers.map((o) => (
              <li key={o.id}>
                {o.name} <span className="muted small">· {o.kind} · {o.status}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {!noOrg && (
        <>
          <CreateEvent onCreated={(id) => { setEventId(id); setMsg("Evento criado em rascunho."); }} onErr={setMsg} />
          {eventId && <EventTools eventId={eventId} onMsg={setMsg} />}
        </>
      )}
    </div>
  );
}

function CreateEvent({ onCreated, onErr }: { onCreated: (id: string) => void; onErr: (m: string) => void }) {
  const [f, setF] = useState<OrgEventInput>({
    slug: "", title: "", description: "", type: "race",
    starts_at: "", ends_at: "", location_name: "Fortaleza, CE",
  });
  const create = useMutation({
    mutationFn: () =>
      api.orgCreateEvent({
        ...f,
        starts_at: f.starts_at ? new Date(f.starts_at).toISOString() : "",
        ends_at: f.ends_at ? new Date(f.ends_at).toISOString() : "",
      }),
    onSuccess: (r) => onCreated(r.id),
    onError: (e) => onErr(e instanceof ApiError ? e.message : "Erro ao criar evento"),
  });

  const set = (k: keyof OrgEventInput) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF((s) => ({ ...s, [k]: e.target.value }));

  return (
    <section>
      <h2>Novo evento</h2>
      <form
        className="card"
        style={{ display: "grid", gap: "0.6rem" }}
        onSubmit={(e) => { e.preventDefault(); create.mutate(); }}
      >
        <label>slug<input value={f.slug} onChange={set("slug")} placeholder="beira-mar-night-2026" required /></label>
        <label>título<input value={f.title} onChange={set("title")} required /></label>
        <label>descrição<textarea value={f.description} onChange={set("description")} rows={2} /></label>
        <div style={{ display: "flex", gap: "0.6rem", flexWrap: "wrap" }}>
          <label style={{ flex: 1 }}>início<input type="datetime-local" value={f.starts_at} onChange={set("starts_at")} required /></label>
          <label style={{ flex: 1 }}>fim<input type="datetime-local" value={f.ends_at} onChange={set("ends_at")} required /></label>
        </div>
        <label>local<input value={f.location_name} onChange={set("location_name")} /></label>
        <button className="btn primary" disabled={create.isPending}>
          {create.isPending ? "…" : "Criar rascunho"}
        </button>
      </form>
    </section>
  );
}

function EventTools({ eventId, onMsg }: { eventId: string; onMsg: (m: string) => void }) {
  const participants = useQuery({ queryKey: ["orgParticipants", eventId], queryFn: () => api.orgParticipants(eventId) });
  const [token, setToken] = useState("");

  const ok = (m: string) => onMsg(m);
  const err = (e: unknown) => onMsg(e instanceof ApiError ? e.message : "Erro");

  return (
    <section>
      <h2>Configurar evento</h2>
      <p className="muted small">id: <code>{eventId}</code></p>

      <div className="card" style={{ display: "grid", gap: "0.9rem" }}>
        <MiniForm
          title="Lote de inscrição"
          fields={[["name", "Lote 1 · 5k"], ["category", "5k"], ["amount_cents", "4990"], ["quota", "200"]]}
          onSubmit={(v) =>
            api.orgAddPrice(eventId, {
              name: v.name, category: v.category,
              amount_cents: Number(v.amount_cents) || 0, quota: Number(v.quota) || 100,
            }).then(() => ok("Lote adicionado")).catch(err)
          }
        />
        <MiniForm
          title="Estação (kit / largada / CP / chegada)"
          fields={[["name", "Largada"], ["role", "start"], ["ord", "0"], ["lat", "-3.7197"], ["lng", "-38.5133"], ["radius_m", "30"]]}
          onSubmit={(v) =>
            api.orgAddStation(eventId, {
              name: v.name, role: v.role, ord: Number(v.ord) || 0,
              lat: Number(v.lat), lng: Number(v.lng), radius_m: Number(v.radius_m) || 30,
            }).then(() => ok("Estação adicionada")).catch(err)
          }
        />
        <MiniForm
          title="Cupom"
          fields={[["code", "LANCAMENTO"], ["discount_type", "percent"], ["value", "20"], ["max_uses", "100"]]}
          onSubmit={(v) =>
            api.orgAddCoupon(eventId, {
              code: v.code, discount_type: v.discount_type,
              value: Number(v.value) || 0, max_uses: Number(v.max_uses) || 0,
            }).then(() => ok("Cupom criado")).catch(err)
          }
        />
        <MiniForm
          title="Credencial de scanner (token mostrado uma vez)"
          fields={[["label", "Chegada — notebook 1"], ["role", "finish"]]}
          onSubmit={(v) =>
            api.orgScannerCredential(eventId, { label: v.label, role: v.role })
              .then((r) => { setToken(r.token); ok("Credencial gerada — copie o token agora"); })
              .catch(err)
          }
        />
        {token && (
          <p className="mono small" style={{ wordBreak: "break-all", color: "var(--sun)" }}>
            token: {token}
          </p>
        )}
      </div>

      <h3 style={{ marginTop: "1.4rem" }}>Inscritos ({participants.data?.participants.length ?? 0})</h3>
      <ul className="lb tight">
        {(participants.data?.participants ?? []).map((p, i) => (
          <li key={i}>
            {p.bib_number || "—"} · {p.username} · {p.category}
            {p.completed ? " · concluiu" : ""}
          </li>
        ))}
      </ul>
    </section>
  );
}

function MiniForm({
  title, fields, onSubmit,
}: {
  title: string;
  fields: [string, string][];
  onSubmit: (v: Record<string, string>) => void;
}) {
  const [v, setV] = useState<Record<string, string>>({});
  return (
    <form
      style={{ borderTop: "1px solid var(--line)", paddingTop: "0.7rem", display: "grid", gap: "0.4rem" }}
      onSubmit={(e) => { e.preventDefault(); onSubmit(v); }}
    >
      <strong className="small">{title}</strong>
      <div style={{ display: "flex", gap: "0.4rem", flexWrap: "wrap" }}>
        {fields.map(([k, ph]) => (
          <input
            key={k}
            style={{ flex: "1 1 120px", minWidth: 0 }}
            placeholder={ph}
            value={v[k] ?? ""}
            onChange={(e) => setV((s) => ({ ...s, [k]: e.target.value }))}
          />
        ))}
        <button className="btn sm">Adicionar</button>
      </div>
    </form>
  );
}
