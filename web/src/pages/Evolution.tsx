import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";

type Week = { week: string; distance_m: number; area_m2: number; new_blocks: number; runs: number };

export function Evolution() {
  const q = useQuery({ queryKey: ["evolution"], queryFn: api.evolution });
  const weeks = q.data?.weeks ?? [];

  const total = useMemo(
    () =>
      weeks.reduce(
        (a, w) => ({ km: a.km + w.distance_m / 1000, area: a.area + w.area_m2, runs: a.runs + w.runs, blk: a.blk + w.new_blocks }),
        { km: 0, area: 0, runs: 0, blk: 0 },
      ),
    [weeks],
  );

  return (
    <div className="page">
      <header className="page-head">
        <h1>Evolução</h1>
        <p className="muted">Últimas 26 semanas — distância, área nova conquistada e frequência.</p>
      </header>

      {q.isLoading && <p className="muted">Carregando…</p>}
      {!q.isLoading && weeks.length === 0 && (
        <p className="muted">Ainda sem corridas válidas suficientes para montar a série.</p>
      )}

      {weeks.length > 0 && (
        <>
          <div className="two-col">
            <div className="stat-tile"><strong>{total.km.toLocaleString("pt-BR", { maximumFractionDigits: 0 })}</strong><span>km no período</span></div>
            <div className="stat-tile"><strong>{formatArea(total.area)}</strong><span>área nova no período</span></div>
          </div>
          <div className="two-col">
            <div className="stat-tile"><strong>{total.runs}</strong><span>corridas</span></div>
            <div className="stat-tile"><strong>{total.blk}</strong><span>quarteirões novos</span></div>
          </div>

          <Chart title="Distância por semana (km)" weeks={weeks} value={(w) => w.distance_m / 1000} fmt={(v) => `${v.toFixed(1)} km`} />
          <Chart title="Área nova por semana" weeks={weeks} value={(w) => w.area_m2} fmt={formatArea} />
          <Chart title="Corridas por semana" weeks={weeks} value={(w) => w.runs} fmt={(v) => `${Math.round(v)}`} kind="bar" />
        </>
      )}
    </div>
  );
}

function formatArea(m2: number) {
  if (m2 >= 1_000_000) return `${(m2 / 1_000_000).toLocaleString("pt-BR", { maximumFractionDigits: 1 })} km²`;
  return `${Math.round(m2).toLocaleString("pt-BR")} m²`;
}

const W = 720;
const H = 180;
const PAD = { t: 12, r: 14, b: 26, l: 46 };

function Chart({
  title,
  weeks,
  value,
  fmt,
  kind = "area",
}: {
  title: string;
  weeks: Week[];
  value: (w: Week) => number;
  fmt: (v: number) => string;
  kind?: "area" | "bar";
}) {
  const [hover, setHover] = useState<number | null>(null);
  const vals = weeks.map(value);
  const max = Math.max(1, ...vals);
  const innerW = W - PAD.l - PAD.r;
  const innerH = H - PAD.t - PAD.b;
  const x = (i: number) => PAD.l + (weeks.length <= 1 ? innerW / 2 : (i / (weeks.length - 1)) * innerW);
  const y = (v: number) => PAD.t + innerH - (v / max) * innerH;

  const line = vals.map((v, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const areaPath = `${line} L${x(vals.length - 1).toFixed(1)},${(PAD.t + innerH).toFixed(1)} L${x(0).toFixed(1)},${(PAD.t + innerH).toFixed(1)} Z`;

  const ticks = [0, max / 2, max];
  const barW = Math.min(18, (innerW / weeks.length) * 0.6);

  return (
    <figure className="evo-fig">
      <figcaption>{title}</figcaption>
      <div className="evo-wrap">
        <svg
          viewBox={`0 0 ${W} ${H}`}
          role="img"
          aria-label={title}
          preserveAspectRatio="none"
          onMouseLeave={() => setHover(null)}
          onMouseMove={(e) => {
            const r = (e.target as SVGElement).ownerSVGElement!.getBoundingClientRect();
            const px = ((e.clientX - r.left) / r.width) * W;
            let best = 0;
            let bd = Infinity;
            weeks.forEach((_, i) => {
              const d = Math.abs(x(i) - px);
              if (d < bd) { bd = d; best = i; }
            });
            setHover(best);
          }}
        >
          {ticks.map((t, i) => (
            <g key={i}>
              <line x1={PAD.l} x2={W - PAD.r} y1={y(t)} y2={y(t)} className="evo-grid" />
              <text x={PAD.l - 8} y={y(t) + 4} className="evo-axis" textAnchor="end">
                {fmt(t)}
              </text>
            </g>
          ))}

          {kind === "area" ? (
            <>
              <path d={areaPath} className="evo-area" />
              <path d={line} className="evo-line" />
              {hover != null && (
                <circle cx={x(hover)} cy={y(vals[hover])} r={4} className="evo-dot" />
              )}
              <circle cx={x(vals.length - 1)} cy={y(vals[vals.length - 1])} r={3.5} className="evo-end" />
            </>
          ) : (
            vals.map((v, i) => (
              <rect
                key={i}
                x={x(i) - barW / 2}
                y={y(v)}
                width={barW}
                height={PAD.t + innerH - y(v)}
                rx={2}
                className={hover === i ? "evo-bar hi" : "evo-bar"}
              />
            ))
          )}

          {hover != null && (
            <line x1={x(hover)} x2={x(hover)} y1={PAD.t} y2={PAD.t + innerH} className="evo-cross" />
          )}
          {weeks.map((w, i) =>
            i % 4 === 0 || i === weeks.length - 1 ? (
              <text key={w.week} x={x(i)} y={H - 8} className="evo-axis" textAnchor="middle">
                {w.week.slice(5)}
              </text>
            ) : null,
          )}
        </svg>
        {hover != null && (
          <div className="evo-tip" style={{ left: `${(x(hover) / W) * 100}%` }}>
            <strong>{fmt(vals[hover])}</strong>
            <span>semana de {weeks[hover].week.split("-").reverse().join("/")}</span>
          </div>
        )}
      </div>
    </figure>
  );
}
