import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, type ShoeItem } from "../lib/api";
import { clock, km } from "../lib/format";

export function ShoesPage() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["shoes"], queryFn: api.shoes });
  const [show, setShow] = useState(false);
  const [brand, setBrand] = useState("");
  const [model, setModel] = useState("");
  const [nickname, setNickname] = useState("");
  const [price, setPrice] = useState("");
  const [goalKm, setGoalKm] = useState("700");

  const create = useMutation({
    mutationFn: () =>
      api.createShoe({
        brand: brand.trim(),
        model: model.trim(),
        nickname: nickname.trim() || undefined,
        purchase_price_cents: price ? Math.round(parseFloat(price.replace(",", ".")) * 100) : undefined,
        lifespan_goal_m: (parseInt(goalKm, 10) || 700) * 1000,
      }),
    onSuccess: () => {
      setShow(false);
      setBrand("");
      setModel("");
      setNickname("");
      setPrice("");
      qc.invalidateQueries({ queryKey: ["shoes"] });
    },
  });
  const retire = useMutation({
    mutationFn: (id: string) => api.retireShoe(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["shoes"] }),
  });

  const shoes = q.data?.shoes ?? [];
  const active = shoes.filter((s) => s.status !== "retired");
  const retired = shoes.filter((s) => s.status === "retired");

  return (
    <div className="page">
      <header className="page-head" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: "1rem" }}>
        <div>
          <h1>Meus tênis</h1>
          <p className="muted">Quilometragem por par, custo por km e curva de vida útil.</p>
        </div>
        <button className="btn primary" onClick={() => setShow((v) => !v)}>
          {show ? "Cancelar" : "+ Novo par"}
        </button>
      </header>

      {show && (
        <form
          className="card"
          onSubmit={(e) => {
            e.preventDefault();
            if (brand.trim() && model.trim()) create.mutate();
          }}
        >
          <div className="two-col">
            <label>Marca<input value={brand} onChange={(e) => setBrand(e.target.value)} placeholder="Nike" required /></label>
            <label>Modelo<input value={model} onChange={(e) => setModel(e.target.value)} placeholder="Pegasus 41" required /></label>
          </div>
          <div className="two-col">
            <label>Apelido (opcional)<input value={nickname} onChange={(e) => setNickname(e.target.value)} placeholder="treino leve" /></label>
            <label>Preço (R$)<input value={price} onChange={(e) => setPrice(e.target.value)} inputMode="decimal" placeholder="799,90" /></label>
          </div>
          <label>Meta de vida útil (km)<input value={goalKm} onChange={(e) => setGoalKm(e.target.value)} inputMode="numeric" /></label>
          {create.isError && <p className="err">{create.error instanceof ApiError ? create.error.message : "Erro ao cadastrar."}</p>}
          <button className="btn primary" disabled={create.isPending}>{create.isPending ? "Salvando…" : "Cadastrar par"}</button>
        </form>
      )}

      {q.isLoading && <p className="muted">Carregando…</p>}
      {!q.isLoading && shoes.length === 0 && <p className="muted">Nenhum par cadastrado.</p>}

      <div className="shoe-grid">
        {active.map((s) => <ShoeCard key={s.id} shoe={s} onRetire={() => retire.mutate(s.id)} />)}
      </div>

      {retired.length > 0 && (
        <>
          <h2>Aposentados</h2>
          <div className="shoe-grid">
            {retired.map((s) => <ShoeCard key={s.id} shoe={s} />)}
          </div>
        </>
      )}
    </div>
  );
}

function ShoeCard({ shoe, onRetire }: { shoe: ShoeItem; onRetire?: () => void }) {
  const pct = Math.min(100, Math.round(shoe.life_pct));
  const cost = shoe.cost_per_km_cents != null ? `R$ ${(shoe.cost_per_km_cents / 100).toFixed(2)}/km` : "—";
  return (
    <div className="card shoe-card">
      <div className="shoe-top">
        <div>
          <strong>{shoe.nickname || `${shoe.brand} ${shoe.model}`}</strong>
          {shoe.nickname && <span className="muted small"> · {shoe.brand} {shoe.model}</span>}
        </div>
        {onRetire && shoe.status !== "retired" && (
          <button className="btn quiet sm" onClick={onRetire}>aposentar</button>
        )}
      </div>
      <div className="shoe-bar">
        <i style={{ width: `${pct}%`, background: pct >= 100 ? "var(--crit)" : pct >= 75 ? "var(--gold, #b7841f)" : "var(--teal)" }} />
      </div>
      <p className="muted small">{km(shoe.total_distance_m)} / {km(shoe.lifespan_goal_m)} km · {pct}% da vida útil</p>
      <div className="shoe-figs">
        <span><strong>{shoe.run_count}</strong> corridas</span>
        <span><strong>{clock(shoe.total_moving_s)}</strong> em movimento</span>
        <span><strong>{cost}</strong></span>
      </div>
      {shoe.alert === "trocar_em_breve" && <p className="err" style={{ color: "var(--gold, #b7841f)" }}>⚠ planeje a troca</p>}
      {shoe.alert === "vencido" && <p className="err">⚠ amortecimento gasto — risco de lesão</p>}
    </div>
  );
}
