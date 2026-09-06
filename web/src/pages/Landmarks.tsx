import { useEffect, useState } from "react";
import { api, LandmarkProgress, Landmark, LandmarkCollection } from "../lib/api";

export function LandmarksPage() {
  const [data, setData] = useState<LandmarkProgress | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [checkingId, setCheckingId] = useState<string | null>(null);
  const [selectedCol, setSelectedCol] = useState<string | null>(null);

  const loadData = () => {
    setLoading(true);
    api.landmarksProgress()
      .then((res) => {
        setData(res);
        setError(null);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleCheckin = (lm: Landmark) => {
    if (!navigator.geolocation) {
      alert("Geolocalização não suportada pelo seu navegador.");
      return;
    }
    setCheckingId(lm.id);
    navigator.geolocation.getCurrentPosition(
      async (pos) => {
        try {
          await api.landmarkCheckin(lm.id, pos.coords.latitude, pos.coords.longitude);
          alert(`Selo "${lm.name}" desbloqueado com sucesso! 🎉`);
          loadData();
        } catch (err: any) {
          alert(err.message || "Não foi possível realizar o check-in.");
        } finally {
          setCheckingId(null);
        }
      },
      (err) => {
        alert("Erro ao obter sua localização: " + err.message);
        setCheckingId(null);
      },
      { enableHighAccuracy: true }
    );
  };

  if (loading) {
    return (
      <div style={{ padding: "2rem", color: "var(--ink-soft)" }}>
        Carregando marcos históricos e selos de Fortaleza...
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: "2rem", color: "var(--crit)" }}>
        Erro ao carregar marcos: {error}
      </div>
    );
  }

  const filteredLandmarks = data?.landmarks.filter((l) =>
    selectedCol ? l.collection_id === selectedCol : true
  );

  const pctUnlocked = data && data.total_landmarks > 0
    ? Math.round((data.unlocked_count / data.total_landmarks) * 100)
    : 0;

  return (
    <div style={{ maxWidth: "1100px", margin: "0 auto", padding: "1.5rem" }}>
      {/* Header */}
      <header style={{ marginBottom: "2rem", borderBottom: "1px solid var(--line)", paddingBottom: "1rem" }}>
        <p style={{ fontFamily: "var(--f-mono)", fontSize: "0.8rem", color: "var(--teal)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
          FortalRunners · Conquistas da Cidade
        </p>
        <h1 style={{ fontFamily: "var(--f-display)", fontSize: "2.2rem", fontWeight: 700, margin: "0.3rem 0 0.5rem" }}>
          Marcos Históricos & Selos
        </h1>
        <p style={{ color: "var(--ink-soft)", maxWidth: "60ch" }}>
          Corra até os cartões-postais e patrimônios de Fortaleza para desbloquear selos permanentes e coleções exclusivas.
        </p>

        {/* Total Progress */}
        <div style={{
          display: "flex",
          alignItems: "center",
          gap: "1.5rem",
          marginTop: "1.2rem",
          padding: "1rem 1.2rem",
          background: "var(--surface)",
          border: "1px solid var(--line)",
          borderRadius: "8px",
        }}>
          <div style={{ flex: 1 }}>
            <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "0.4rem", fontSize: "0.9rem", fontWeight: 600 }}>
              <span>Progresso dos Marcos</span>
              <span style={{ color: "var(--teal)" }}>{data?.unlocked_count} de {data?.total_landmarks} ({pctUnlocked}%)</span>
            </div>
            <div style={{ width: "100%", height: "8px", background: "var(--line)", borderRadius: "999px", overflow: "hidden" }}>
              <div style={{ width: `${pctUnlocked}%`, height: "100%", background: "var(--teal)", transition: "width 0.4s ease" }} />
            </div>
          </div>
        </div>
      </header>

      {/* Collections Filter */}
      {data?.collections && data.collections.length > 0 && (
        <section style={{ marginBottom: "2rem" }}>
          <h3 style={{ fontFamily: "var(--f-display)", fontSize: "1.1rem", marginBottom: "0.8rem" }}>
            Coleções de Fortaleza
          </h3>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: "1rem" }}>
            <div
              onClick={() => setSelectedCol(null)}
              style={{
                padding: "1rem",
                borderRadius: "8px",
                border: selectedCol === null ? "2px solid var(--teal)" : "1px solid var(--line)",
                background: "var(--surface)",
                cursor: "pointer",
                transition: "all 0.2s ease",
              }}
            >
              <div style={{ fontWeight: 700, fontSize: "0.95rem" }}>Todos os Marcos</div>
              <div style={{ fontSize: "0.8rem", color: "var(--ink-soft)", marginTop: "0.2rem" }}>
                Exibir todos os {data.total_landmarks} marcos de Fortaleza
              </div>
            </div>

            {data.collections.map((col: LandmarkCollection) => {
              const active = selectedCol === col.id;
              const colPct = col.total > 0 ? Math.round((col.unlocked / col.total) * 100) : 0;
              return (
                <div
                  key={col.id}
                  onClick={() => setSelectedCol(col.id)}
                  style={{
                    padding: "1rem",
                    borderRadius: "8px",
                    border: active ? "2px solid var(--teal)" : "1px solid var(--line)",
                    background: "var(--surface)",
                    cursor: "pointer",
                    transition: "all 0.2s ease",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span style={{ fontWeight: 700, fontSize: "0.95rem" }}>{col.name}</span>
                    <span style={{ fontSize: "0.75rem", fontFamily: "var(--f-mono)", color: "var(--teal)" }}>+{col.reward_xp} XP</span>
                  </div>
                  <div style={{ fontSize: "0.8rem", color: "var(--ink-soft)", marginTop: "0.2rem" }}>
                    {col.description}
                  </div>
                  <div style={{ marginTop: "0.6rem", fontSize: "0.8rem", color: "var(--teal-deep)" }}>
                    <strong>{col.unlocked}</strong> / {col.total} selos ({colPct}%)
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {/* Landmark Cards Grid */}
      <section>
        <h3 style={{ fontFamily: "var(--f-display)", fontSize: "1.1rem", marginBottom: "1rem" }}>
          {selectedCol ? "Marcos da Coleção" : "Todos os Marcos"} ({filteredLandmarks?.length})
        </h3>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))", gap: "1.2rem" }}>
          {filteredLandmarks?.map((lm) => (
            <div
              key={lm.id}
              style={{
                border: "1px solid var(--line)",
                borderRadius: "10px",
                background: "var(--surface)",
                padding: "1.2rem",
                display: "flex",
                flexDirection: "column",
                justifyContent: "space-between",
                boxShadow: "0 2px 8px rgba(0,0,0,0.04)",
              }}
            >
              <div>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "0.6rem" }}>
                  <div style={{
                    fontSize: "0.75rem",
                    fontFamily: "var(--f-mono)",
                    textTransform: "uppercase",
                    padding: "0.2rem 0.6rem",
                    borderRadius: "999px",
                    background: lm.checked_in ? "color-mix(in srgb, var(--good) 15%, transparent)" : "var(--line)",
                    color: lm.checked_in ? "var(--good)" : "var(--ink-soft)",
                    fontWeight: 600,
                  }}>
                    {lm.checked_in ? "✓ Conquistado" : "🔒 Bloqueado"}
                  </div>
                  <span style={{ fontSize: "0.75rem", color: "var(--ink-soft)", fontFamily: "var(--f-mono)" }}>
                    Raio {lm.radius_m}m
                  </span>
                </div>

                <h4 style={{ fontFamily: "var(--f-display)", fontSize: "1.1rem", margin: "0 0 0.4rem", fontWeight: 700 }}>
                  {lm.name}
                </h4>
                <p style={{ fontSize: "0.85rem", color: "var(--ink-soft)", margin: "0 0 0.8rem", lineHeight: 1.4 }}>
                  {lm.blurb}
                </p>
              </div>

              <div style={{ marginTop: "1rem", paddingTop: "0.8rem", borderTop: "1px solid var(--line)", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <span style={{ fontSize: "0.75rem", fontFamily: "var(--f-mono)", color: "var(--ink-soft)" }}>
                  GPS: {lm.lat.toFixed(4)}, {lm.lng.toFixed(4)}
                </span>

                {!lm.checked_in && (
                  <button
                    onClick={() => handleCheckin(lm)}
                    disabled={checkingId === lm.id}
                    style={{
                      background: "var(--teal)",
                      color: "#fff",
                      border: "none",
                      padding: "0.4rem 0.8rem",
                      borderRadius: "6px",
                      fontSize: "0.8rem",
                      fontWeight: 600,
                      cursor: "pointer",
                    }}
                  >
                    {checkingId === lm.id ? "Verificando..." : "Check-in GPS"}
                  </button>
                )}
                {lm.checked_in && lm.checked_in_at && (
                  <span style={{ fontSize: "0.75rem", color: "var(--good)", fontWeight: 600 }}>
                    {new Date(lm.checked_in_at).toLocaleDateString()}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
