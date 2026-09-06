export function km(m?: number): string {
  if (!m) return "0,00";
  return (m / 1000).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function pace(sPerKm?: number): string {
  if (!sPerKm || sPerKm <= 0) return "--'--\"";
  const mm = Math.floor(sPerKm / 60);
  const ss = Math.round(sPerKm % 60);
  return `${mm}'${String(ss).padStart(2, "0")}"`;
}

export function clock(s?: number): string {
  if (!s || s < 0) s = 0;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  const p = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${p(m)}:${p(sec)}` : `${p(m)}:${p(sec)}`;
}

export function area(m2?: number): string {
  if (!m2) return "0 m²";
  if (m2 >= 1_000_000) return `${(m2 / 1_000_000).toLocaleString("pt-BR", { maximumFractionDigits: 2 })} km²`;
  return `${Math.round(m2).toLocaleString("pt-BR")} m²`;
}
