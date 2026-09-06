import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getTokens, ApiError, type PublicUser } from "./lib/api";

// Portal do corredor — Fase 0: só login/cadastro + "quem sou eu".
// Mapa de territórios, histórico e admin entram nas próximas fases.

export function App() {
  const qc = useQueryClient();
  const authed = !!getTokens();

  const me = useQuery<PublicUser>({
    queryKey: ["me"],
    queryFn: api.me,
    enabled: authed,
    retry: false,
  });

  if (authed && me.data) {
    return (
      <Shell>
        <h1>Olá, {me.data.username}</h1>
        <p className="muted">
          ID {me.data.athlete_id} · {me.data.email} · perfil {me.data.role}
        </p>
        <button
          className="btn ghost"
          onClick={async () => {
            await api.logout();
            qc.clear();
          }}
        >
          Sair
        </button>
        <p className="muted small">
          Fase 0 — próxima entrega: mapa de territórios, gravação de corrida (mobile), ranking.
        </p>
      </Shell>
    );
  }

  return (
    <Shell>
      <AuthForm onDone={() => qc.invalidateQueries({ queryKey: ["me"] })} />
    </Shell>
  );
}

function AuthForm({ onDone }: { onDone: () => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const m = useMutation({
    mutationFn: () =>
      mode === "login" ? api.login(email, password) : api.register({ email, username, password }),
    onSuccess: onDone,
  });

  return (
    <form
      className="card"
      onSubmit={(e) => {
        e.preventDefault();
        m.mutate();
      }}
    >
      <div className="seg">
        <button type="button" className={mode === "login" ? "on" : ""} onClick={() => setMode("login")}>
          Entrar
        </button>
        <button type="button" className={mode === "register" ? "on" : ""} onClick={() => setMode("register")}>
          Criar conta
        </button>
      </div>

      <label>
        E-mail
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      </label>
      {mode === "register" && (
        <label>
          Nome de usuário
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </label>
      )}
      <label>
        Senha
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
      </label>

      {m.isError && (
        <p className="err">{m.error instanceof ApiError ? m.error.message : "Falha na autenticação"}</p>
      )}

      <button className="btn primary" disabled={m.isPending}>
        {m.isPending ? "…" : mode === "login" ? "Entrar" : "Criar conta"}
      </button>
    </form>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <header>
        <strong>FortalRunners</strong>
        <span className="muted small">portal do corredor</span>
      </header>
      <main>{children}</main>
    </div>
  );
}
