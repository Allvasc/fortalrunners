import { useEffect, useState } from "react";
import { api, FeedEvent, Friend } from "../lib/api";

export function SocialPage() {
  const [feed, setFeed] = useState<FeedEvent[]>([]);
  const [friends, setFriends] = useState<Friend[]>([]);
  const [loading, setLoading] = useState(true);
  const [targetId, setTargetId] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const loadData = () => {
    setLoading(true);
    Promise.all([api.feed(), api.friends()])
      .then(([feedRes, friendsRes]) => {
        setFeed(feedRes.feed ?? []);
        setFriends(friendsRes.friends ?? []);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleAddFriend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!targetId.trim()) return;
    setSubmitting(true);
    try {
      await api.sendFriendRequest(targetId.trim());
      alert("Solicitação de amizade enviada!");
      setTargetId("");
      loadData();
    } catch (err: any) {
      alert(err.message || "Erro ao adicionar amigo.");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleKudos = async (runId: string) => {
    try {
      await api.toggleKudos(runId);
      loadData();
    } catch (err: any) {
      alert("Erro ao dar kudos: " + err.message);
    }
  };

  if (loading) {
    return <div style={{ padding: "2rem", color: "var(--ink-soft)" }}>Carregando feed e amigos...</div>;
  }

  return (
    <div className="page">
      <header style={{ marginBottom: "2rem", borderBottom: "1px solid var(--line)", paddingBottom: "1rem" }}>
        <p style={{ fontFamily: "var(--f-mono)", fontSize: "0.8rem", color: "var(--teal)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
          FortalRunners · Comunidade
        </p>
        <h1 style={{ fontFamily: "var(--f-display)", fontSize: "2.2rem", fontWeight: 700, margin: "0.3rem 0 0.5rem" }}>
          Feed Social & Amigos
        </h1>
        <p style={{ color: "var(--ink-soft)", maxWidth: "60ch" }}>
          Acompanhe as corridas, conquistas de território e selos dos seus parceiros de treino.
        </p>
      </header>

      <div className="two-col sidebar" style={{ gap: "2rem" }}>
        {/* Activity Feed */}
        <div>
          <h3 style={{ fontFamily: "var(--f-display)", fontSize: "1.1rem", marginBottom: "1rem" }}>
            Feed de Atividades ({feed.length})
          </h3>

          <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
            {feed.length === 0 ? (
              <div style={{ padding: "2rem", background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "10px", textAlign: "center", color: "var(--ink-soft)" }}>
                Nenhuma atividade no feed ainda. Adicione amigos ou faça sua primeira corrida!
              </div>
            ) : (
              feed.map((ev) => (
                <div
                  key={ev.id}
                  style={{
                    padding: "1.2rem",
                    borderRadius: "10px",
                    border: "1px solid var(--line)",
                    background: "var(--surface)",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "0.4rem" }}>
                    <span style={{ fontWeight: 700, color: "var(--ink)" }}>{ev.actor_name}</span>
                    <span style={{ fontSize: "0.75rem", fontFamily: "var(--f-mono)", color: "var(--ink-soft)" }}>
                      {new Date(ev.created_at).toLocaleDateString()}
                    </span>
                  </div>

                  <div style={{ fontSize: "0.9rem", color: "var(--ink-soft)", marginBottom: "0.8rem" }}>
                    {ev.type === "run" && "🏃 completou uma corrida na cidade!"}
                    {ev.type === "claim" && "🚩 conquistou um novo território em Fortaleza!"}
                    {ev.type === "badge" && "🏆 desbloqueou um novo selo histórico!"}
                    {ev.type === "checkin" && "📍 fez check-in em um marco turístico!"}
                  </div>

                  {ev.type === "run" && (
                    <div style={{ display: "flex", alignItems: "center", gap: "1rem", marginTop: "0.5rem" }}>
                      <button
                        onClick={() => handleToggleKudos(ev.subject_id)}
                        style={{
                          background: ev.has_kudos ? "var(--coral)" : "var(--line)",
                          color: ev.has_kudos ? "#fff" : "var(--ink)",
                          border: "none",
                          padding: "0.3rem 0.8rem",
                          borderRadius: "6px",
                          fontSize: "0.8rem",
                          fontWeight: 600,
                          cursor: "pointer",
                        }}
                      >
                        👏 Kudos ({ev.kudos_count})
                      </button>
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        </div>

        {/* Sidebar: Add Friend & Friends list */}
        <div>
          <div style={{ padding: "1.2rem", background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "10px", marginBottom: "1.5rem" }}>
            <h4 style={{ fontFamily: "var(--f-display)", fontSize: "1rem", margin: "0 0 0.8rem" }}>Adicionar Amigo</h4>
            <form onSubmit={handleAddFriend} style={{ display: "flex", flexDirection: "column", gap: "0.6rem" }}>
              <input
                type="text"
                value={targetId}
                onChange={(e) => setTargetId(e.target.value)}
                placeholder="ID do Atleta (ex: FR-000123)"
                style={{
                  padding: "0.5rem",
                  borderRadius: "6px",
                  border: "1px solid var(--line)",
                  background: "var(--bg)",
                  color: "var(--ink)",
                  fontSize: "0.85rem",
                }}
              />
              <button
                type="submit"
                disabled={submitting}
                style={{
                  background: "var(--teal)",
                  color: "#fff",
                  border: "none",
                  padding: "0.5rem",
                  borderRadius: "6px",
                  fontWeight: 600,
                  fontSize: "0.85rem",
                  cursor: "pointer",
                }}
              >
                {submitting ? "Enviando..." : "Enviar Convite"}
              </button>
            </form>
          </div>

          <div style={{ padding: "1.2rem", background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "10px" }}>
            <h4 style={{ fontFamily: "var(--f-display)", fontSize: "1rem", margin: "0 0 0.8rem" }}>Seus Amigos ({friends.length})</h4>
            <div style={{ display: "flex", flexDirection: "column", gap: "0.6rem" }}>
              {friends.length === 0 ? (
                <div style={{ fontSize: "0.8rem", color: "var(--ink-soft)", fontStyle: "italic" }}>
                  Nenhum amigo adicionado ainda.
                </div>
              ) : (
                friends.map((f) => (
                  <div key={f.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: "0.85rem", borderBottom: "1px solid var(--line)", paddingBottom: "0.4rem" }}>
                    <div>
                      <div style={{ fontWeight: 600 }}>{f.name}</div>
                      <div style={{ fontSize: "0.75rem", fontFamily: "var(--f-mono)", color: "var(--ink-soft)" }}>{f.athlete_id}</div>
                    </div>
                    <span style={{ fontSize: "0.75rem", color: f.status === "accepted" ? "var(--good)" : "var(--warn)" }}>
                      {f.status === "accepted" ? "Amigo" : "Pendente"}
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
