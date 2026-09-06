import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, oauthLogin } from "../lib/api";

type Mode = "login" | "register";

export function Auth() {
  const qc = useQueryClient();
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [mfaToken, setMfaToken] = useState<string | null>(null);
  const [code, setCode] = useState("");

  const done = () => qc.invalidateQueries({ queryKey: ["me"] });

  const submit = useMutation({
    mutationFn: async () => {
      if (mode === "register") {
        await api.register({ email, username, password });
        return;
      }
      const r = await api.login(email, password);
      if (r.kind === "mfa") setMfaToken(r.mfaToken);
    },
    onSuccess: () => {
      if (!mfaToken) done();
    },
  });

  const verify = useMutation({
    mutationFn: () => api.verifyMfa(mfaToken!, code.trim()),
    onSuccess: done,
  });

  return (
    <div className="auth">
      <div className="auth-brand">
        <span className="mark" aria-hidden />
        FortalRunners
        <span className="muted small">portal do corredor</span>
      </div>

      {mfaToken ? (
        <form
          className="card"
          onSubmit={(e) => {
            e.preventDefault();
            verify.mutate();
          }}
        >
          <h2>Verificação em duas etapas</h2>
          <p className="muted small">
            Digite o código de 6 dígitos do seu app de autenticação — ou um código de recuperação.
          </p>
          <label>
            Código
            <input
              inputMode="text"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="123456"
              autoFocus
              required
            />
          </label>
          {verify.isError && (
            <p className="err">
              {verify.error instanceof ApiError ? verify.error.message : "Código inválido"}
            </p>
          )}
          <button className="btn primary" disabled={verify.isPending}>
            {verify.isPending ? "Verificando…" : "Entrar"}
          </button>
          <button
            type="button"
            className="btn quiet"
            onClick={() => {
              setMfaToken(null);
              setCode("");
            }}
          >
            Voltar
          </button>
        </form>
      ) : (
        <form
          className="card"
          onSubmit={(e) => {
            e.preventDefault();
            submit.mutate();
          }}
        >
          <div className="seg">
            <button type="button" className={mode === "login" ? "on" : ""} onClick={() => setMode("login")}>
              Entrar
            </button>
            <button
              type="button"
              className={mode === "register" ? "on" : ""}
              onClick={() => setMode("register")}
            >
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
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
            />
          </label>

          {submit.isError && (
            <p className="err">
              {submit.error instanceof ApiError ? submit.error.message : "Falha na autenticação"}
            </p>
          )}

          <button className="btn primary" disabled={submit.isPending}>
            {submit.isPending ? "…" : mode === "login" ? "Entrar" : "Criar conta"}
          </button>

          <div className="oauth-sep">
            <span>ou</span>
          </div>
          <div className="oauth-row">
            <button type="button" className="btn" onClick={() => oauthLogin("google")}>
              Continuar com Google
            </button>
            <button type="button" className="btn" onClick={() => oauthLogin("apple")}>
              Continuar com Apple
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
