// Formatação pt-BR: vírgula decimal, unidades com espaço fino.

const nf = (min = 0, max = 1) =>
  new Intl.NumberFormat("pt-BR", { minimumFractionDigits: min, maximumFractionDigits: max });

export function km(meters?: number): string {
  if (!meters) return "0 km";
  return `${nf(1, 1).format(meters / 1000)} km`;
}

export function area(m2?: number): string {
  if (!m2) return "0 m²";
  if (m2 >= 1_000_000) return `${nf(2, 2).format(m2 / 1_000_000)} km²`;
  return `${nf(0, 0).format(m2)} m²`;
}

export function hours(seconds?: number): string {
  if (!seconds) return "0 h";
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  return h > 0 ? `${h} h ${m} min` : `${m} min`;
}

export function int(n?: number): string {
  return nf(0, 0).format(n ?? 0);
}

export function date(iso?: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "short", year: "numeric" });
}

export function relativeDeadline(iso: string): string {
  const ms = new Date(iso).getTime() - Date.now();
  if (ms <= 0) return "encerrado";
  const days = Math.floor(ms / 86_400_000);
  if (days >= 1) return `faltam ${days} d`;
  const h = Math.floor(ms / 3_600_000);
  if (h >= 1) return `faltam ${h} h`;
  return "falta < 1 h";
}

const RECORD_LABELS: Record<string, string> = {
  longest_distance_m: "Maior distância",
  longest_run_s: "Corrida mais longa",
  most_elevation_m: "Mais elevação",
  biggest_territory_m2: "Maior território",
  fastest_5k_s: "5 km mais rápido",
  fastest_10k_s: "10 km mais rápido",
};
export function recordLabel(key: string): string {
  return RECORD_LABELS[key] ?? key;
}

export function recordValue(key: string, value: number): string {
  if (key.endsWith("_m2")) return area(value);
  if (key.endsWith("_m")) return km(value);
  if (key.endsWith("_s")) return hours(value);
  return int(value);
}

const CADENCE_LABELS: Record<string, string> = {
  weekly: "Semanal",
  biweekly: "Quinzenal",
  monthly: "Mensal",
  oneoff: "Especial",
};
export function cadenceLabel(c: string): string {
  return CADENCE_LABELS[c] ?? c;
}

const METRIC_LABELS: Record<string, string> = {
  new_area_m2: "área nova",
  new_blocks: "quarteirões novos",
  new_neighborhoods: "bairros novos",
  distance_m: "distância",
  elevation_gain_m: "elevação",
};
export function metricValue(metric: string, value: number): string {
  switch (metric) {
    case "new_area_m2":
      return area(value);
    case "distance_m":
      return km(value);
    case "elevation_gain_m":
      return `${int(value)} m`;
    default:
      return int(value);
  }
}
export function metricLabel(metric: string): string {
  return METRIC_LABELS[metric] ?? metric;
}
