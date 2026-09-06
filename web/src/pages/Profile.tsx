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

      {me.data && me.data.email_verified === false && <VerifyEmailBanner />}

      <section>
        <h2>Contas conectadas</h2>
        <ConnectionsPanel />
      </section>

      <section>
        <h2>Privacidade & dados</h2>
        <PrivacyPanel />
      </section>

      <section>
        <h2>Verificação em duas etapas</h2>
        <MFAPanel />
      </section>
    </div>
  );
}

function VerifyEmailBanner() {
  const m = useMutation({ mutationFn: api.resendEmailVerification });
  return (
    <section>
      <h2>Confirme seu e-mail</h2>
      <div className="card">
        <p className="muted small">
          Enviamos um link de confirmação quando você criou a conta. Não achou? Reenvie.
        </p>
        {m.isSuccess ? (
          <p className="ok small">E-mail reenviado. Confira sua caixa (e o spam).</p>
        ) : (
          <button className="btn quiet" onClick={() => m.mutate()} disabled={m.isPending}>
            {m.isPending ? "Enviando…" : "Reenviar e-mail de confirmação"}
          </button>
        )}
        {m.isError && (
          <p className="err small">
            {m.error instanceof ApiError ? m.error.message : "Não foi possível reenviar."}
          </p>
        )}
      </div>
    </section>
  );
}

function PrivacyPanel() {
  const [radius, setRadius] = useState(200);
  const [msg, setMsg] = useState("");

  async function setHomeZone() {
    setMsg("Obtendo sua posição…");
    navigator.geolocation.getCurrentPosition(
      async (pos) => {
        try {
          await api.updateMe({
            home: { lat: pos.coords.latitude, lng: pos.coords.longitude, radius_m: radius },
          });
          setMsg(`Zona de ocultação de ${radius} m salva na sua posição atual. O traçado dentro dela não é mais gravado.`);
        } catch (e: any) {
          setMsg(e?.message ?? "Erro ao salvar");
        }
      },
      () => setMsg("Não foi possível obter sua localização."),
    );
  }

  async function exportData() {
    setMsg("Preparando o arquivo…");
    const data = await api.exportMyData();
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "fortalrunners-meus-dados.json";
    a.click();
    URL.revokeObjectURL(a.href);
    setMsg("Download iniciado.");
  }

  async function del() {
    if (!confirm("Excluir a conta anonimiza seus dados pessoais imediatamente e não pode ser desfeito. Continuar?")) return;
    try {
      await api.deleteAccount();
      await api.logout();
      location.href = "/";
    } catch (e: any) {
      setMsg(e?.message ?? "Erro ao excluir");
    }
  }

  return (
    <div className="card" style={{ display: "grid", gap: "0.8rem" }}>
      <div>
        <p className="small mono muted" style={{ textTransform: "uppercase", letterSpacing: "0.08em", margin: "0 0 0.3rem" }}>
          Zona de ocultação
        </p>
        <p className="small muted" style={{ margin: "0 0 0.5rem" }}>
          Um raio em torno de casa/trabalho onde o traçado nunca é gravado nem exibido.
        </p>
        <div style={{ display: "flex", gap: "0.5rem", alignItems: "center", flexWrap: "wrap" }}>
          <label style={{ display: "flex", gap: "0.4rem", alignItems: "center" }}>
            raio
            <select value={radius} onChange={(e) => setRadius(Number(e.target.value))}>
              {[100, 150, 200, 300, 500].map((r) => (
                <option key={r} value={r}>{r} m</option>
              ))}
            </select>
          </label>
          <button className="btn" onClick={setHomeZone}>Usar minha posição atual</button>
        </div>
      </div>

      <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap", borderTop: "1px solid var(--line)", paddingTop: "0.7rem" }}>
        <button className="btn" onClick={exportData}>Exportar meus dados (LGPD)</button>
        <button className="btn" style={{ color: "var(--coral)", borderColor: "var(--coral)" }} onClick={del}>
          Excluir minha conta
        </button>
      </div>

      {msg && <p className="small" style={{ color: "var(--ink)" }}>{msg}</p>}
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
        <li key={r.distance_key}>
          <span>{recordLabel(r.distance_key)}</span>
          <span className="mono">{recordValue(r.distance_key, r.value_s)}</span>
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
