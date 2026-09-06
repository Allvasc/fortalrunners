import { useEffect, useState } from "react";
import { api, RouteItem, RouteReview } from "../lib/api";

export function RoutesPage() {
  const [routes, setRoutes] = useState<RouteItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedRouteId, setSelectedRouteId] = useState<string | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<{ route: RouteItem; reviews: RouteReview[] } | null>(null);

  // Review form
  const [rating, setRating] = useState(5);
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const loadRoutes = () => {
    setLoading(true);
    api.routes()
      .then((res) => {
        setRoutes(res.routes);
        setError(null);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadRoutes();
  }, []);

  const openRoute = async (id: string) => {
    setSelectedRouteId(id);
    try {
      const res = await api.routeDetail(id);
      setSelectedDetail(res);
    } catch (err: any) {
      alert("Erro ao carregar detalhes da rota: " + err.message);
    }
  };

  const handleReviewSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedRouteId) return;
    setSubmitting(true);
    try {
      await api.addRouteReview(selectedRouteId, rating, ["recomendada"], body);
      alert("Avaliação enviada com sucesso! ⭐");
      setBody("");
      openRoute(selectedRouteId);
      loadRoutes();
    } catch (err: any) {
      alert("Erro ao enviar avaliação: " + err.message);
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div style={{ padding: "2rem", color: "var(--ink-soft)" }}>Carregando rotas de Fortaleza...</div>;
  }

  if (error) {
    return <div style={{ padding: "2rem", color: "var(--crit)" }}>Erro ao carregar rotas: {error}</div>;
  }

  return (
    <div className="page">
      <header style={{ marginBottom: "2rem", borderBottom: "1px solid var(--line)", paddingBottom: "1rem" }}>
        <p style={{ fontFamily: "var(--f-mono)", fontSize: "0.8rem", color: "var(--teal)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
          FortalRunners · Percursos Curados
        </p>
        <h1 style={{ fontFamily: "var(--f-display)", fontSize: "2.2rem", fontWeight: 700, margin: "0.3rem 0 0.5rem" }}>
          Rotas Recomendadas de Fortaleza
        </h1>
        <p style={{ color: "var(--ink-soft)", maxWidth: "60ch" }}>
          Descubra e avalie percursos seguros, sombreados e testados pela comunidade de corredores.
        </p>
      </header>

      <div className={`two-col${selectedDetail ? " has-detail" : ""}`}>
        {/* Routes List */}
        <div>
          <h3 style={{ fontFamily: "var(--f-display)", fontSize: "1.1rem", marginBottom: "1rem" }}>
            Percursos Disponíveis ({routes.length})
          </h3>
          <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
            {routes.map((r) => {
              const isSelected = selectedRouteId === r.id;
              return (
                <div
                  key={r.id}
                  onClick={() => openRoute(r.id)}
                  style={{
                    padding: "1.2rem",
                    borderRadius: "10px",
                    border: isSelected ? "2px solid var(--teal)" : "1px solid var(--line)",
                    background: "var(--surface)",
                    cursor: "pointer",
                    transition: "all 0.2s ease",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "0.4rem" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                      <span style={{ fontWeight: 700, fontSize: "1.05rem" }}>{r.name}</span>
                      {r.is_official && (
                        <span style={{ fontSize: "0.7rem", background: "var(--teal)", color: "#fff", padding: "0.15rem 0.5rem", borderRadius: "999px", fontWeight: 600 }}>
                          Oficial
                        </span>
                      )}
                    </div>
                    <span style={{ fontSize: "0.9rem", fontWeight: 700, color: "var(--teal)" }}>
                      {(r.distance_m / 1000).toFixed(1)} km
                    </span>
                  </div>

                  <p style={{ fontSize: "0.85rem", color: "var(--ink-soft)", margin: "0 0 0.8rem", lineHeight: 1.4 }}>
                    {r.description}
                  </p>

                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: "0.8rem", color: "var(--ink-soft)" }}>
                    <span>Piso: <strong>{r.surface}</strong></span>
                    <span>
                      ⭐ {r.avg_rating > 0 ? r.avg_rating.toFixed(1) : "Sem nota"} ({r.review_count} avaliações)
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Selected Route Detail Panel */}
        {selectedDetail && (
          <div style={{ border: "1px solid var(--line)", borderRadius: "10px", padding: "1.5rem", background: "var(--surface)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1rem" }}>
              <div>
                <h2 style={{ fontFamily: "var(--f-display)", fontSize: "1.4rem", margin: "0 0 0.3rem" }}>
                  {selectedDetail.route.name}
                </h2>
                <span style={{ fontSize: "0.85rem", color: "var(--teal)", fontWeight: 600 }}>
                  {(selectedDetail.route.distance_m / 1000).toFixed(2)} km · Piso {selectedDetail.route.surface}
                </span>
              </div>
              <button
                onClick={() => { setSelectedRouteId(null); setSelectedDetail(null); }}
                style={{ background: "none", border: "none", fontSize: "1.2rem", cursor: "pointer", color: "var(--ink-soft)" }}
              >
                ✕
              </button>
            </div>

            <p style={{ fontSize: "0.9rem", color: "var(--ink-soft)", marginBottom: "1.5rem", lineHeight: 1.5 }}>
              {selectedDetail.route.description}
            </p>

            {/* Reviews list */}
            <h4 style={{ fontFamily: "var(--f-display)", fontSize: "1rem", marginBottom: "0.8rem", borderBottom: "1px solid var(--line)", paddingBottom: "0.4rem" }}>
              Avaliações da Comunidade ({selectedDetail.reviews.length})
            </h4>

            <div style={{ display: "flex", flexDirection: "column", gap: "0.8rem", maxHeight: "250px", overflowY: "auto", marginBottom: "1.5rem" }}>
              {selectedDetail.reviews.length === 0 ? (
                <div style={{ fontSize: "0.85rem", color: "var(--ink-soft)", fontStyle: "italic" }}>
                  Nenhuma avaliação ainda. Seja o primeiro a avaliar!
                </div>
              ) : (
                selectedDetail.reviews.map((rev) => (
                  <div key={rev.id} style={{ padding: "0.8rem", background: "var(--bg)", borderRadius: "6px" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.8rem", fontWeight: 600, marginBottom: "0.2rem" }}>
                      <span>{rev.user_name || "Corredor"}</span>
                      <span style={{ color: "var(--gold)" }}>{"★".repeat(rev.rating)}</span>
                    </div>
                    {rev.body && <div style={{ fontSize: "0.85rem", color: "var(--ink)" }}>{rev.body}</div>}
                  </div>
                ))
              )}
            </div>

            {/* Add Review Form */}
            <form onSubmit={handleReviewSubmit} style={{ borderTop: "1px solid var(--line)", paddingTop: "1rem" }}>
              <h5 style={{ margin: "0 0 0.8rem", fontSize: "0.9rem" }}>Deixar sua avaliação</h5>

              <div style={{ display: "flex", gap: "0.5rem", marginBottom: "0.8rem", alignItems: "center" }}>
                <span style={{ fontSize: "0.85rem" }}>Nota:</span>
                {[1, 2, 3, 4, 5].map((num) => (
                  <button
                    key={num}
                    type="button"
                    onClick={() => setRating(num)}
                    style={{
                      background: rating >= num ? "var(--gold)" : "var(--line)",
                      color: "#fff",
                      border: "none",
                      width: "28px",
                      height: "28px",
                      borderRadius: "50%",
                      cursor: "pointer",
                      fontSize: "0.85rem",
                    }}
                  >
                    ★
                  </button>
                ))}
              </div>

              <textarea
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder="Comentário sobre a rota (segurança, sombra, pavimento...)"
                rows={3}
                style={{
                  width: "100%",
                  padding: "0.6rem",
                  borderRadius: "6px",
                  border: "1px solid var(--line)",
                  background: "var(--bg)",
                  color: "var(--ink)",
                  fontSize: "0.85rem",
                  marginBottom: "0.8rem",
                }}
              />

              <button
                type="submit"
                disabled={submitting}
                style={{
                  width: "100%",
                  background: "var(--teal)",
                  color: "#fff",
                  border: "none",
                  padding: "0.6rem",
                  borderRadius: "6px",
                  fontWeight: 600,
                  fontSize: "0.9rem",
                  cursor: "pointer",
                }}
              >
                {submitting ? "Enviando..." : "Enviar Avaliação"}
              </button>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
