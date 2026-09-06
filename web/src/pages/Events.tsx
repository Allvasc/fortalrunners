import React, { useEffect, useState } from "react";
import { api, EventItem, QRToken, PaymentOrder } from "../lib/api";

export const EventsPage: React.FC = () => {
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [qr, setQr] = useState<QRToken | null>(null);
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [msg, setMsg] = useState("");

  useEffect(() => {
    loadEvents();
  }, []);

  async function loadEvents() {
    try {
      const data = await api.events();
      setEvents(data.events || []);
    } catch {
      /* ignora erro na carga inicial */
    } finally {
      setLoading(false);
    }
  }

  async function handleRegister(ev: EventItem) {
    try {
      const res = await api.registerEvent(ev.id, "5k", "M");
      setMsg(res.message);
      loadEvents();
      fetchQR(ev.id);
    } catch (e: any) {
      setMsg(e.message || "Erro na inscrição");
    }
  }

  async function fetchQR(eventId?: string) {
    try {
      const res = await api.qrToken(eventId);
      setQr(res.qr);
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
      const res = await api.checkout(priceId, "pix");
      setOrder(res.order);
    } catch (e: any) {
      setMsg(e.message || "Erro no pagamento");
    }
  }

  if (loading) {
    return <div className="p-8 text-center text-gray-500">Carregando eventos de Fortaleza...</div>;
  }

  return (
    <div className="max-w-5xl mx-auto p-6 space-y-8">
      <header className="border-b border-gray-200 pb-4">
        <h1 className="text-3xl font-bold text-teal-800">🏁 Eventos & Provas de Fortaleza</h1>
        <p className="text-gray-600">Inscreva-se em maratonas, corridas noturnas na Beira-Mar e passeios ecológicos com passaporte QR Code digital.</p>
      </header>

      {msg && <div className="p-3 bg-teal-100 text-teal-900 rounded-lg font-medium">{msg}</div>}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {events.map((ev) => (
          <div key={ev.id} className="bg-white border border-gray-200 rounded-xl p-5 shadow-sm space-y-4 hover:shadow-md transition">
            <div className="flex justify-between items-start">
              <div>
                <span className="text-xs font-semibold px-2.5 py-1 bg-amber-100 text-amber-800 rounded-full uppercase tracking-wider">
                  {ev.type}
                </span>
                <h3 className="text-xl font-bold text-gray-900 mt-2">{ev.title}</h3>
                <p className="text-sm text-gray-500">📍 {ev.location_name}</p>
              </div>
              <span className={`text-xs font-bold px-2 py-1 rounded ${ev.status === "live" ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-600"}`}>
                {ev.status.toUpperCase()}
              </span>
            </div>

            <p className="text-sm text-gray-600">{ev.description}</p>

            <div className="text-xs text-gray-500 space-y-1">
              <p>🗓️ Início: {new Date(ev.starts_at).toLocaleString("pt-BR")}</p>
              <p>🏁 Término: {new Date(ev.ends_at).toLocaleString("pt-BR")}</p>
            </div>

            <div className="pt-3 border-t border-gray-100 flex items-center justify-between">
              {ev.is_registered ? (
                <div className="flex items-center gap-3">
                  <span className="text-xs font-bold text-green-700 bg-green-50 px-3 py-1.5 rounded-lg border border-green-200">
                    ✓ Inscrito
                  </span>
                  <button
                    onClick={() => fetchQR(ev.id)}
                    className="text-xs bg-teal-600 hover:bg-teal-700 text-white px-3 py-1.5 rounded-lg font-medium"
                  >
                    📱 Ver QR Code
                  </button>
                </div>
              ) : (
                <div className="flex gap-2">
                  <button
                    onClick={() => handleRegister(ev)}
                    className="text-xs bg-orange-600 hover:bg-orange-700 text-white px-4 py-2 rounded-lg font-bold shadow-sm"
                  >
                    Garantir Inscrição
                  </button>
                  <button
                    onClick={() => {
                      handleCheckout(ev.prices?.[0]?.id ?? "");
                    }}
                    className="text-xs bg-teal-800 hover:bg-teal-900 text-white px-3 py-2 rounded-lg font-medium"
                  >
                    {ev.prices?.[0]
                      ? `Pagar via PIX (R$ ${(ev.prices[0].amount_cents / 100).toFixed(2)})`
                      : "Inscrição paga"}
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {qr && (
        <div className="bg-teal-950 text-white p-6 rounded-2xl border border-teal-800 space-y-4 max-w-md mx-auto text-center shadow-xl">
          <h3 className="text-lg font-bold text-teal-300">🎫 Passaporte Digital FortalRunners</h3>
          <p className="text-xs text-teal-200">Apresente este QR Code dinâmico na estação de kit/largada do evento.</p>
          <div className="bg-white p-4 rounded-xl inline-block border-2 border-teal-500">
            <img
              src={`https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(qr.payload_sig)}`}
              alt="QR Code Atleta"
              className="w-44 h-44 mx-auto"
            />
          </div>
          <p className="text-xs font-mono text-amber-300">Token Sig: {qr.payload_sig}</p>
          <p className="text-[10px] text-teal-400">Expira em: {new Date(qr.expires_at).toLocaleTimeString()}</p>
        </div>
      )}

      {order && (
        <div className="bg-white p-6 rounded-xl border border-gray-200 shadow-md max-w-md mx-auto space-y-3">
          <h3 className="text-base font-bold text-gray-900">💳 Pagamento PIX Gerado (Asaas)</h3>
          <p className="text-xs text-gray-600">Escaneie o QR Code abaixo no app do seu banco ou copie a chave de pagamento.</p>
          {order.qr_code_url && (
            <img src={order.qr_code_url} alt="PIX QR Code" className="w-48 h-48 mx-auto border p-2 rounded-lg" />
          )}
          <div className="bg-gray-50 p-2 rounded font-mono text-[11px] break-all border text-gray-700">
            {order.pix_code}
          </div>
          <span className="text-xs font-semibold text-amber-600 block text-center">Status: {order.status.toUpperCase()}</span>
        </div>
      )}
    </div>
  );
};
