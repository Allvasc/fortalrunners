import { useEffect, useState } from "react";
import { api, ClubItem } from "../lib/api";

export function ClubsPage() {
  const [clubs, setClubs] = useState<ClubItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);

  // Create form
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [colorHex, setColorHex] = useState("#0E7C86");
  const [submitting, setSubmitting] = useState(false);

  const loadClubs = () => {
    setLoading(true);
    api.clubs()
      .then((res) => setClubs(res.clubs))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadClubs();
  }, []);

  const handleCreateClub = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setSubmitting(true);
    try {
      await api.createClub(name.trim(), description.trim(), colorHex);
      alert("Clube criado com sucesso! 🎉");
      setName("");
      setDescription("");
      setShowCreate(false);
      loadClubs();
    } catch (err: any) {
      alert("Erro ao criar clube: " + err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleJoinClub = async (id: string) => {
    try {
      await api.joinClub(id);
      alert("Você entrou no clube!");
      loadClubs();
    } catch (err: any) {
      alert("Erro ao entrar no clube: " + err.message);
    }
  };

  if (loading) {
    return <div style={{ padding: "2rem", color: "var(--ink-soft)" }}>Carregando clubes de corrida...</div>;
  }

  return (
    <div className="page">
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "2rem", borderBottom: "1px solid var(--line)", paddingBottom: "1rem" }}>
        <div>
          <p style={{ fontFamily: "var(--f-mono)", fontSize: "0.8rem", color: "var(--teal)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
            FortalRunners · Equipes & Grupos
          </p>
          <h1 style={{ fontFamily: "var(--f-display)", fontSize: "2.2rem", fontWeight: 700, margin: "0.3rem 0 0.5rem" }}>
            Clubes de Corrida
          </h1>
          <p style={{ color: "var(--ink-soft)", maxWidth: "60ch" }}>
            Junte-se a um grupo de corrida da sua região ou crie seu próprio clube em Fortaleza.
          </p>
        </div>

        <button
          onClick={() => setShowCreate((v) => !v)}
          style={{
            background: "var(--teal)",
            color: "#fff",
            border: "none",
            padding: "0.6rem 1.2rem",
            borderRadius: "8px",
            fontWeight: 600,
            fontSize: "0.9rem",
            cursor: "pointer",
          }}
        >
          {showCreate ? "Cancelar" : "+ Criar Clube"}
        </button>
      </header>

      {/* Create Club Form */}
      {showCreate && (
        <form onSubmit={handleCreateClub} style={{ padding: "1.5rem", background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "10px", marginBottom: "2rem" }}>
          <h3 style={{ fontFamily: "var(--f-display)", margin: "0 0 1rem" }}>Novo Clube de Corrida</h3>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem", marginBottom: "1rem" }}>
            <div>
              <label style={{ display: "block", fontSize: "0.85rem", marginBottom: "0.4rem", fontWeight: 600 }}>Nome do Clube</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Ex: Aldeota Runners"
                required
                style={{ width: "100%", padding: "0.6rem", borderRadius: "6px", border: "1px solid var(--line)", background: "var(--bg)", color: "var(--ink)" }}
              />
            </div>
            <div>
              <label style={{ display: "block", fontSize: "0.85rem", marginBottom: "0.4rem", fontWeight: 600 }}>Cor do Clube</label>
              <input
                type="color"
                value={colorHex}
                onChange={(e) => setColorHex(e.target.value)}
                style={{ width: "100%", height: "38px", border: "none", borderRadius: "6px", cursor: "pointer" }}
              />
            </div>
          </div>
          <div style={{ marginBottom: "1rem" }}>
            <label style={{ display: "block", fontSize: "0.85rem", marginBottom: "0.4rem", fontWeight: 600 }}>Descrição</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Descreva o propósito do grupo, horários de treino e locais de encontro..."
              rows={3}
              style={{ width: "100%", padding: "0.6rem", borderRadius: "6px", border: "1px solid var(--line)", background: "var(--bg)", color: "var(--ink)" }}
            />
          </div>
          <button
            type="submit"
            disabled={submitting}
            style={{ background: "var(--teal)", color: "#fff", border: "none", padding: "0.6rem 1.2rem", borderRadius: "6px", fontWeight: 600, cursor: "pointer" }}
          >
            {submitting ? "Criando..." : "Salvar Clube"}
          </button>
        </form>
      )}

      {/* Clubs Grid */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))", gap: "1.2rem" }}>
        {clubs.map((c) => (
          <div
            key={c.id}
            style={{
              padding: "1.2rem",
              borderRadius: "10px",
              border: "1px solid var(--line)",
              borderLeft: `5px solid ${c.color_hex}`,
              background: "var(--surface)",
              display: "flex",
              flexDirection: "column",
              justifyContent: "space-between",
            }}
          >
            <div>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "0.4rem" }}>
                <h3 style={{ fontFamily: "var(--f-display)", fontSize: "1.2rem", margin: 0, fontWeight: 700 }}>{c.name}</h3>
                <span style={{ fontSize: "0.75rem", fontFamily: "var(--f-mono)", background: "var(--line)", padding: "0.2rem 0.5rem", borderRadius: "999px" }}>
                  {c.member_count} membros
                </span>
              </div>
              <p style={{ fontSize: "0.85rem", color: "var(--ink-soft)", margin: "0 0 1rem", lineHeight: 1.4 }}>
                {c.description}
              </p>
            </div>

            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", paddingTop: "0.8rem", borderTop: "1px solid var(--line)" }}>
              <span style={{ fontSize: "0.8rem", color: "var(--ink-soft)" }}>
                {c.is_member ? "✓ Membro do clube" : "Aberto a novos membros"}
              </span>

              {!c.is_member && (
                <button
                  onClick={() => handleJoinClub(c.id)}
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
                  Entrar no Clube
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
