import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type Run, type Split } from "../lib/api";
import { clock, date, km, pace } from "../lib/format";

export function History() {
  const q = useQuery({ queryKey: ["runs"], queryFn: () => api.runs(40) });

  return (
    <div className="page">
      <header className="page-head">
        <h1>Histórico</h1>
        <p className="muted">
          Cada corrida com splits por km, elevação, cadência e pace ajustado ao aclive (GAP).
        </p>
      </header>

      {q.isLoading && <p className="muted">Carregando…</p>}
      {q.isError && <p className="err">Não foi possível carregar as corridas.</p>}
      {q.data?.runs.length === 0 && (
        <p className="muted">Nenhuma corrida ainda — registre uma pelo app.</p>
      )}

      <ul className="run-list">
        {q.data?.runs.map((r) => (
          <li key={r.id}>
            <RunItem run={r} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function RunItem({ run }: { run: Run }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="run-item">
      <button className="run-head" onClick={() => setOpen((v) => !v)} aria-expanded={open}>
        <div className="run-when">
          <strong>{date(run.started_at)}</strong>
          <span className="muted small">{run.data_source}</span>
        </div>
        <div className="run-figs">
          <span>{km(run.distance_m)}</span>
          <span className="mono">{pace(run.avg_pace_s)}</span>
          <span className="muted">{clock(run.moving_s)}</span>
          <span className="muted">{run.elevation_gain_m} m D+</span>
        </div>
        {run.status !== "valid" && run.status !== "processing" && (
          <span className="chip">{run.status}</span>
        )}
      </button>
      {open && <RunMetrics id={run.id} />}
    </div>
  );
}

function RunMetrics({ id }: { id: string }) {
  const q = useQuery({ queryKey: ["run-metrics", id], queryFn: () => api.runMetrics(id) });

  if (q.isLoading) return <p className="muted small">Carregando métricas…</p>;
  if (q.isError || !q.data) return <p className="err small">Métricas indisponíveis.</p>;

  const m = q.data;
  const maxPace = Math.max(...m.splits.map((s) => s.pace_s_per_km), 1);

  return (
    <div className="run-metrics">
      <div className="metric-chips">
        {m.grade_adjusted_pace_s ? (
          <Chip label="Pace ajustado (GAP)" value={pace(m.grade_adjusted_pace_s)} />
        ) : null}
        {m.best_km_pace_s ? <Chip label="Melhor km" value={pace(m.best_km_pace_s)} /> : null}
        <Chip label="Elevação" value={`+${Math.round(m.elev_gain_m)} / −${Math.round(m.elev_loss_m)} m`} />
        {m.has_cadence && <Chip label="Cadência méd." value={`${m.avg_cadence_spm} spm`} />}
        {m.has_heart_rate && <Chip label="FC méd. / máx." value={`${m.avg_hr_bpm} / ${m.max_hr_bpm} bpm`} />}
      </div>

      <table className="splits">
        <thead>
          <tr>
            <th>km</th>
            <th>Pace</th>
            <th></th>
            <th>D+</th>
            {m.has_cadence && <th>spm</th>}
            {m.has_heart_rate && <th>bpm</th>}
          </tr>
        </thead>
        <tbody>
          {m.splits.map((s) => (
            <SplitRow key={s.index} s={s} maxPace={maxPace} showCad={m.has_cadence} showHr={m.has_heart_rate} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SplitRow({
  s,
  maxPace,
  showCad,
  showHr,
}: {
  s: Split;
  maxPace: number;
  showCad: boolean;
  showHr: boolean;
}) {
  const partial = s.distance_m < 995;
  return (
    <tr>
      <td className="mono">
        {s.index}
        {partial && <span className="muted"> ·{(s.distance_m / 1000).toFixed(2)}</span>}
      </td>
      <td className="mono">{pace(s.pace_s_per_km)}</td>
      <td className="bar-cell">
        <span className="split-bar" style={{ width: `${(s.pace_s_per_km / maxPace) * 100}%` }} />
      </td>
      <td className="mono muted">{Math.round(s.elev_gain_m)}</td>
      {showCad && <td className="mono muted">{s.avg_cadence_spm ? Math.round(s.avg_cadence_spm) : "—"}</td>}
      {showHr && <td className="mono muted">{s.avg_hr_bpm ? Math.round(s.avg_hr_bpm) : "—"}</td>}
    </tr>
  );
}

function Chip({ label, value }: { label: string; value: string }) {
  return (
    <div className="mchip">
      <span className="v mono">{value}</span>
      <span className="l">{label}</span>
    </div>
  );
}
