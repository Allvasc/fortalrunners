import { useEffect } from "react";
import { useMutation } from "@tanstack/react-query";
import { api, ApiError } from "../lib/api";

// Página pública aberta pelo link do e-mail: /confirmar-email?token=...
export function ConfirmEmail() {
  const token = new URLSearchParams(window.location.search).get("token") ?? "";
  const m = useMutation({ mutationFn: () => api.verifyEmail(token) });

  useEffect(() => {
    if (token) m.mutate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  return (
    <div className="auth">
      <div className="auth-brand">
        <span className="mark" aria-hidden />
        FortalRunners
        <span className="muted small">confirmar e-mail</span>
      </div>

      <div className="card">
        {!token && (
          <>
            <h2>Link inválido</h2>
            <p className="muted small">Abra o link direto do e-mail que você recebeu.</p>
            <a className="btn quiet" href="/">Voltar</a>
          </>
        )}
        {token && m.isPending && <p className="muted small">Confirmando…</p>}
        {token && m.isSuccess && (
          <>
            <h2>E-mail confirmado ✓</h2>
            <p className="muted small">Tudo certo. Sua conta está verificada.</p>
            <a className="btn primary" href="/">Ir para o app</a>
          </>
        )}
        {token && m.isError && (
          <>
            <h2>Não deu para confirmar</h2>
            <p className="err">
              {m.error instanceof ApiError ? m.error.message : "Link inválido ou expirado."}
            </p>
            <p className="muted small">
              Peça um novo e-mail de confirmação nas configurações da conta.
            </p>
            <a className="btn quiet" href="/">Voltar</a>
          </>
        )}
      </div>
    </div>
  );
}
