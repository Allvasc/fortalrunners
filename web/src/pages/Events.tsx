import React, { useEffect, useState } from "react";
import { api, EventItem, QRToken, PaymentOrder } from "../lib/api";

function money(cents: number) {
  return `R$ ${(cents / 100).toFixed(2).replace(".", ",")}`;
}

export const EventsPage: React.FC = () => {
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [qr, setQr] = useState<QRToken | null>(null);
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [msg, setMsg] = useState("");

  useEffect(() => {
    void loadEvents();
  }, []);

  async function loadEvents() {
    try {
      const data = await api.events();
      setEvents(data.events || []);
    } catch {
      /* silencioso na carga inicial */
    } finally {
      setLoading(false);
    }
  }

  async function handleRegister(ev: EventItem) {
    try {
      const res = await api.registerEvent(ev.id, "5k", "M");
      setMsg(res.message);
      void loadEvents();
      void fetchQR(ev.id);
    } catch (e: any) {
      setMsg(e?.message ?? "Erro na inscrição");
    }
  }

  async function fetchQR(eventId?: string) {
    try {
      setQr((await api.qrToken(eventId)).qr);
    } catch {
      /* ignora */
    }
  }

  async function handleCheckout(priceId: string) {
    if (!priceId) {
      setMsg("Este evento ainda não tem lote de inscrição aberto.");
      return;
    }
    try {
      setOrder((await api.checkout(priceId, "pix")).order);
    } catch (e: any) {
      setMsg(e?.message ?? "Erro no pagamento");
    }
  }

  if (loading) {
    return <div className="page"><p className="muted">Carregando eventos de Fortaleza…</p></div>;
  }

  return (
    <div className="page">
      <header className="page-head">
        <h1>Eventos & Provas</h1>
        <p className="muted">
          Corridas noturnas na Beira-Mar, maratonas e passeios ecológicos. A inscrição
          gera um passaporte com QR para o credenciamento e a largada.
        </p>
      </header>

      {msg && <p className="err" style={{ borderColor: "var(--teal)", color: "var(--ink)" }}>{msg}</p>}

      <section style={{ display: "grid", gap: "1rem" }}>
        {events.length === 0 && <p className="muted">Nenhum evento aberto no momento.</p>}
        {events.map((ev) => {
          const lot = ev.prices?.[0];
          return (
            <article key={ev.id} className="card" style={{ display: "grid", gap: "0.6rem" }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "1rem", alignItems: "baseline" }}>
                <div>
                  <span className="tag">{ev.type}</span>
                  <h2 style={{ margin: "0.3rem 0 0", fontSize: "1.2rem" }}>{ev.title}</h2>
                  <p className="small muted" style={{ margin: 0 }}>{ev.location_name}</p>
                </div>
                <span className="chip">{ev.status}</span>
              </div>

              <p style={{ margin: 0, fontSize: "0.92rem" }}>{ev.description}</p>
              <p className="small muted" style={{ margin: 0 }}>
                {new Date(ev.starts_at).toLocaleString("pt-BR")} — {new Date(ev.ends_at).toLocaleString("pt-BR")}
              </p>

              <div className="row-btns" style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap", marginTop: "0.3rem" }}>
                {ev.is_registered ? (
                  <>
                    <span className="chip" style={{ color: "var(--good)" }}>✓ inscrito</span>
                    <button className="btn" onClick={() => fetchQR(ev.id)}>Ver QR</button>
                  </>
                ) : lot ? (
                  <button className="btn primary" onClick={() => handleCheckout(lot.id)}>
                    Inscrever — {money(lot.amount_cents)} via PIX
                  </button>
                ) : (
                  <button className="btn primary" onClick={() => handleRegister(ev)}>
                    Inscrição gratuita
                  </button>
                )}
              </div>
            </article>
          );
        })}
      </section>

      {qr && (
        <section className="card qr" style={{ textAlign: "center", display: "grid", gap: "0.6rem", justifyItems: "center" }}>
          <h2 style={{ margin: 0, fontSize: "1.1rem" }}>Passaporte digital</h2>
          <p className="small muted" style={{ margin: 0 }}>
            Apresente na estação de kit/largada. O código gira a cada ~45 s.
          </p>
          <img
            width={180}
            height={180}
            alt="QR do atleta"
            src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(qr.payload_sig)}`}
          />
          <p className="small mono muted" style={{ margin: 0 }}>
            expira {new Date(qr.expires_at).toLocaleTimeString("pt-BR")}
          </p>
        </section>
      )}

      {order && (
        <section className="card" style={{ display: "grid", gap: "0.5rem", maxWidth: "24rem" }}>
          <h2 style={{ margin: 0, fontSize: "1.05rem" }}>Cobrança PIX gerada</h2>
          {order.sandbox && (
            <p className="small" style={{ margin: 0, color: "var(--sun)" }}>
              modo sandbox — pagamento não é real
            </p>
          )}
          <p className="small muted" style={{ margin: 0 }}>Copie o código no app do seu banco:</p>
          <code className="mono" style={{ wordBreak: "break-all", fontSize: "0.72rem" }}>{order.pix_code}</code>
          <span className="small mono muted">status: {order.status}</span>
        </section>
      )}
    </div>
  );
};
