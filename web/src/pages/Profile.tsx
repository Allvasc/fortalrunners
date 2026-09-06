import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import QRCode from "qrcode";
import { api, ApiError, type Lifetime, type PersonalRecord } from "../lib/api";
import { area, date, hours, int, km, recordLabel, recordValue } from "../lib/format";

export function Profile() {
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const life = useQuery({ queryKey: ["lifetime"], queryFn: api.lifetime });
  const recs = useQuery({ queryKey: ["records"], queryFn: api.records });

  return (
    <div className="page">
      <header className="page-head">
        <h1>{me.data?.username ?? "Perfil"}</h1>
        <p className="muted">
          {me.data && (
            <>
              {me.data.athlete_id} · {me.data.email} · perfil {me.data.role}
            </>
          )}
        </p>
      </header>

      <section>
        <h2>Números de sempre</h2>
        {life.isLoading && <p className="muted">Carregando…</p>}
        {life.data && <LifetimeGrid l={life.data} />}
      </section>

      <section>
        <h2>Recordes pessoais</h2>
        {recs.data && <Records records={recs.data.records} />}
        {recs.data && recs.data.records.length === 0 && (
          <p className="muted">Ainda sem recordes — eles aparecem depois da primeira corrida válida.</p>
        )}
      </section>

      <section>
        <h2>Contas conectadas</h2>
        <ConnectionsPanel />
      </section>

      <section>
        <h2>Verificação em duas etapas</h2>
        <MFAPanel />
      </section>
    </div>
  );
}

function ConnectionsPanel() {
  const [msg, setMsg] = useState("");
  return (
    <div className="card" style={{ display: "grid", gap: "0.6rem" }}>
      <p className="muted small" style={{ margin: 0 }}>
        Vincule Google ou Apple à sua conta, ou conecte o Strava para importar o histórico
        de corridas (mesmo pipeline de território, marcado como importado).
      </p>
      <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
        <button className="btn" onClick={() => api.linkOAuth("google").catch(() => setMsg("Google não configurado no servidor."))}>
          Vincular Google
        </button>
        <button className="btn" onClick={() => api.linkOAuth("apple").catch(() => setMsg("Apple não configurado no servidor."))}>
          Vincular Apple
        </button>
        <button className="btn" onClick={() => api.connectStrava().catch(() => setMsg("Strava não configurado no servidor."))}>
          Conectar Strava
        </button>
      </div>
      {msg && <p className="err">{msg}</p>}
    </div>
  );
}

function LifetimeGrid({ l }: { l: Lifetime }) {
  const tiles: Array<[string, string]> = [
    ["Corridas", int(l.run_count)],
    ["Distância", km(l.total_distance_m)],
    ["Tempo em movimento", hours(l.total_moving_s)],
    ["Passos", int(l.total_steps)],
    ["Elevação", `${int(l.elevation_gain_m)} m`],
    ["Território", area(l.territory_area_m2)],
    ["Dias ativos", int(l.active_days)],
    ["Sequência atual", `${int(l.current_streak_days)} d`],
    ["Maior sequência", `${int(l.longest_streak_days)} d`],
    ["Última corrida", date(l.last_run_date)],
  ];
  return (
    <div className="tiles">
      {tiles.map(([label, value]) => (
        <div className="tile" key={label}>
          <span className="v">{value}</span>
          <span className="l">{label}</span>
        </div>
      ))}
    </div>
  );
}

function Records({ records }: { records: PersonalRecord[] }) {
  return (
    <ul className="rec-list">
      {records.map((r) => (
        <li key={r.key}>
          <span>{recordLabel(r.key)}</span>
          <span className="mono">{recordValue(r.key, r.value)}</span>
          <span className="muted small">{date(r.achieved_at)}</span>
        </li>
      ))}
    </ul>
  );
}

function MFAPanel() {
  const qc = useQueryClient();
  const status = useQuery({ queryKey: ["mfa"], queryFn: api.mfaStatus });
  const [phase, setPhase] = useState<"idle" | "setup" | "recovery" | "disable">("idle");
  const [secret, setSecret] = useState("");
  const [qr, setQr] = useState("");
  const [code, setCode] = useState("");
  const [recovery, setRecovery] = useState<string[]>([]);

  const refresh = () => qc.invalidateQueries({ queryKey: ["mfa"] });

  const setup = useMutation({
    mutationFn: api.mfaSetup,
    onSuccess: async (r) => {
      setSecret(r.secret);
      setQr(await QRCode.toDataURL(r.otpauth_url, { margin: 1, width: 200 }));
      setPhase("setup");
    },
  });

  const activate = useMutation({
    mutationFn: () => api.mfaActivate(code.trim()),
    onSuccess: (r) => {
      setRecovery(r.recovery_codes);
      setCode("");
      setPhase("recovery");
      refresh();
    },
  });

  const disable = useMutation({
    mutationFn: () => api.mfaDisable(code.trim()),
    onSuccess: () => {
      setCode("");
      setPhase("idle");
      refresh();
    },
  });

  useEffect(() => {
    if (phase === "idle") {
      setSecret("");
      setQr("");
      setRecovery([]);
    }
  }, [phase]);

  if (status.isLoading) return <p className="muted">Carregando…</p>;

  const enabled = status.data?.enabled;

  return (
    <div className="card mfa">
      <div className="mfa-head">
        <div>
          <strong>{enabled ? "Ativa" : "Desativada"}</strong>
          {enabled && (
            <span className="muted small">
              {" "}
              · {status.data?.recovery_codes_left} códigos de recuperação restantes
            </span>
          )}
        </div>
        {!enabled && phase === "idle" && (
          <button className="btn primary sm" onClick={() => setup.mutate()} disabled={setup.isPending}>
            {setup.isPending ? "…" : "Ativar 2FA"}
          </button>
        )}
        {enabled && phase === "idle" && (
          <button className="btn ghost sm" onClick={() => setPhase("disable")}>
            Desativar
          </button>
        )}
      </div>

      {phase === "setup" && (
        <form
          className="mfa-setup"
          onSubmit={(e) => {
            e.preventDefault();
            activate.mutate();
          }}
        >
          <p className="muted small">
            Escaneie no Google Authenticator, Authy, 1Password ou Aegis. Ou digite a chave manualmente.
          </p>
          {qr && <img src={qr} alt="QR code do TOTP" width={200} height={200} className="qr" />}
          <code className="secret">{secret}</code>
          <label>
            Código do app
            <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="123456" autoFocus />
          </label>
          {activate.isError && (
            <p className="err">
              {activate.error instanceof ApiError ? activate.error.message : "Código inválido"}
            </p>
          )}
          <div className="row-btns">
            <button className="btn primary sm" disabled={activate.isPending}>
              {activate.isPending ? "…" : "Confirmar"}
            </button>
            <button type="button" className="btn quiet sm" onClick={() => setPhase("idle")}>
              Cancelar
            </button>
          </div>
        </form>
      )}

      {phase === "recovery" && (
        <div className="mfa-recovery">
          <p>
            <strong>Guarde estes códigos de recuperação.</strong> Cada um serve uma vez, se você perder o
            app. Eles não serão mostrados de novo.
          </p>
          <ul className="codes">
            {recovery.map((c) => (
              <li key={c} className="mono">
                {c}
              </li>
            ))}
          </ul>
          <button className="btn primary sm" onClick={() => setPhase("idle")}>
            Guardei os códigos
          </button>
        </div>
      )}

      {phase === "disable" && (
        <form
          className="mfa-setup"
          onSubmit={(e) => {
            e.preventDefault();
            disable.mutate();
          }}
        >
          <p className="muted small">Confirme com um código do app (ou de recuperação) para desativar.</p>
          <label>
            Código
            <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="123456" autoFocus />
          </label>
          {disable.isError && (
            <p className="err">
              {disable.error instanceof ApiError ? disable.error.message : "Código inválido"}
            </p>
          )}
          <div className="row-btns">
            <button className="btn ghost sm" disabled={disable.isPending}>
              {disable.isPending ? "…" : "Desativar 2FA"}
            </button>
            <button type="button" className="btn quiet sm" onClick={() => setPhase("idle")}>
              Cancelar
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
