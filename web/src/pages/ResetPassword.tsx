import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { api, ApiError } from "../lib/api";

// Página pública aberta pelo link do e-mail: /redefinir-senha?token=...
export function ResetPassword() {
  const token = new URLSearchParams(window.location.search).get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");

  const m = useMutation({ mutationFn: () => api.resetPassword(token, password) });

  return (
    <div className="auth">
      <div className="auth-brand">
        <span className="mark" aria-hidden />
        FortalRunners
        <span className="muted small">redefinir senha</span>
      </div>

      {!token ? (
        <div className="card">
          <h2>Link inválido</h2>
          <p className="muted small">Abra o link direto do e-mail que você recebeu.</p>
          <a className="btn quiet" href="/">Voltar</a>
        </div>
      ) : m.isSuccess ? (
        <div className="card">
          <h2>Senha redefinida ✓</h2>
          <p className="muted small">Você já pode entrar com a nova senha.</p>
          <a className="btn primary" href="/">Ir para o login</a>
        </div>
      ) : (
        <form
          className="card"
          onSubmit={(e) => {
            e.preventDefault();
            if (password.length >= 8 && password === confirm) m.mutate();
          }}
        >
          <h2>Nova senha</h2>
          <label>
            Nova senha
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="mínimo 8 caracteres"
              autoFocus
              required
            />
          </label>
          <label>
            Confirmar
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
            />
          </label>
          {confirm && password !== confirm && <p className="err">As senhas não conferem.</p>}
          {m.isError && (
            <p className="err">{m.error instanceof ApiError ? m.error.message : "Não foi possível redefinir."}</p>
          )}
          <button className="btn primary" disabled={m.isPending || password.length < 8 || password !== confirm}>
            {m.isPending ? "Salvando…" : "Redefinir senha"}
          </button>
        </form>
      )}
    </div>
  );
}
