// Motor de gravação de corrida. GPS em background via expo-location +
// expo-task-manager; os pontos vão para o AsyncStorage (canal entre a task de
// background e a tela). A tela lê num intervalo e recalcula distância/pace.
import AsyncStorage from "@react-native-async-storage/async-storage";
import * as Location from "expo-location";
import { Pedometer } from "expo-sensors";
import * as TaskManager from "expo-task-manager";

export const LOCATION_TASK = "fr-location-task";
const BUF_KEY = "fr.rec.points";
const META_KEY = "fr.rec.meta";
const CAD_KEY = "fr.rec.cadence";

export type GPSPoint = {
  lat: number;
  lon: number;
  alt?: number;
  t: number; // unix ms
  acc?: number; // precisão horizontal (m)
  spd?: number; // m/s
};

export type Sample = { t: number; v: number }; // cadência (passos/min) no tempo

type Meta = { startedAt: number; paused: boolean };

// --- task de background: só acumula os pontos ---
TaskManager.defineTask(LOCATION_TASK, async ({ data, error }) => {
  if (error || !data) return;
  const { locations } = data as { locations: Location.LocationObject[] };
  const meta = await readMeta();
  if (!meta || meta.paused) return;

  const pts: GPSPoint[] = locations.map((l) => ({
    lat: l.coords.latitude,
    lon: l.coords.longitude,
    alt: l.coords.altitude ?? undefined,
    t: Math.round(l.timestamp),
    acc: l.coords.accuracy ?? undefined,
    spd: l.coords.speed ?? undefined,
  }));
  await appendPoints(pts);
});

async function readMeta(): Promise<Meta | null> {
  const raw = await AsyncStorage.getItem(META_KEY);
  return raw ? (JSON.parse(raw) as Meta) : null;
}
async function appendPoints(pts: GPSPoint[]) {
  const raw = await AsyncStorage.getItem(BUF_KEY);
  const buf: GPSPoint[] = raw ? JSON.parse(raw) : [];
  buf.push(...pts);
  await AsyncStorage.setItem(BUF_KEY, JSON.stringify(buf));
}

// --- API de controle ---

export async function ensurePermissions(): Promise<boolean> {
  const fg = await Location.requestForegroundPermissionsAsync();
  if (fg.status !== "granted") return false;
  const bg = await Location.requestBackgroundPermissionsAsync();
  return bg.status === "granted" || fg.status === "granted"; // background é desejável, não obrigatório
}

// --- cadência (pedômetro no foreground; para junto com a tela ativa) ---
let cadSub: { remove: () => void } | null = null;
let lastSteps = 0;
let lastStepsAt = 0;

async function startCadence() {
  const ok = await Pedometer.isAvailableAsync().catch(() => false);
  if (!ok) return;
  lastSteps = 0;
  lastStepsAt = Date.now();
  cadSub = Pedometer.watchStepCount(async ({ steps }) => {
    const now = Date.now();
    const dSteps = steps - lastSteps;
    const dt = (now - lastStepsAt) / 1000;
    lastSteps = steps;
    lastStepsAt = now;
    if (dt < 5 || dSteps <= 0) return;
    const spm = Math.round((dSteps / dt) * 60);
    const raw = await AsyncStorage.getItem(CAD_KEY);
    const buf: Sample[] = raw ? JSON.parse(raw) : [];
    buf.push({ t: now, v: spm });
    await AsyncStorage.setItem(CAD_KEY, JSON.stringify(buf));
  });
}

function stopCadence() {
  cadSub?.remove();
  cadSub = null;
}

export async function startRecording(): Promise<boolean> {
  if (!(await ensurePermissions())) return false;
  await AsyncStorage.multiRemove([BUF_KEY, META_KEY, CAD_KEY]);
  await AsyncStorage.setItem(META_KEY, JSON.stringify({ startedAt: Date.now(), paused: false } satisfies Meta));
  void startCadence();

  const running = await Location.hasStartedLocationUpdatesAsync(LOCATION_TASK).catch(() => false);
  if (running) await Location.stopLocationUpdatesAsync(LOCATION_TASK);

  await Location.startLocationUpdatesAsync(LOCATION_TASK, {
    accuracy: Location.Accuracy.BestForNavigation,
    timeInterval: 1000,
    distanceInterval: 3,
    deferredUpdatesInterval: 1000,
    pausesUpdatesAutomatically: false,
    activityType: Location.ActivityType.Fitness,
    foregroundService: {
      notificationTitle: "FortalRunners",
      notificationBody: "Gravando sua corrida…",
      notificationColor: "#E0562F",
    },
  });
  return true;
}

export async function setPaused(paused: boolean) {
  const meta = await readMeta();
  if (meta) await AsyncStorage.setItem(META_KEY, JSON.stringify({ ...meta, paused }));
}

/**
 * Encerra uma gravação órfã: a task de GPS em background continua rodando (com a
 * notificação "Gravando sua corrida…") mas não há corrida ativa — acontece quando
 * o app é fechado à força durante uma corrida. Chamar na abertura do app.
 */
export async function cleanupStaleRecording() {
  try {
    const running = await Location.hasStartedLocationUpdatesAsync(LOCATION_TASK).catch(() => false);
    if (!running) return;
    const meta = await readMeta();
    // sem meta = órfã; meta com mais de 8h = corrida esquecida
    const stale = !meta || Date.now() - meta.startedAt > 8 * 3600 * 1000;
    if (stale) {
      await Location.stopLocationUpdatesAsync(LOCATION_TASK).catch(() => {});
      stopCadence();
      await AsyncStorage.multiRemove([BUF_KEY, META_KEY, CAD_KEY]);
    }
  } catch {
    /* best-effort */
  }
}

export async function stopRecording(): Promise<{ startedAt: number; points: GPSPoint[]; cadence: Sample[] }> {
  const running = await Location.hasStartedLocationUpdatesAsync(LOCATION_TASK).catch(() => false);
  if (running) await Location.stopLocationUpdatesAsync(LOCATION_TASK);
  stopCadence();
  const meta = await readMeta();
  const [ptsRaw, cadRaw] = await AsyncStorage.multiGet([BUF_KEY, CAD_KEY]);
  const points: GPSPoint[] = ptsRaw[1] ? JSON.parse(ptsRaw[1]) : [];
  const cadence: Sample[] = cadRaw[1] ? JSON.parse(cadRaw[1]) : [];
  await AsyncStorage.multiRemove([BUF_KEY, META_KEY, CAD_KEY]);
  return { startedAt: meta?.startedAt ?? (points[0]?.t ?? Date.now()), points, cadence };
}

export async function readLive(): Promise<{ meta: Meta | null; points: GPSPoint[] }> {
  const [metaRaw, bufRaw] = await AsyncStorage.multiGet([META_KEY, BUF_KEY]);
  return {
    meta: metaRaw[1] ? JSON.parse(metaRaw[1]) : null,
    points: bufRaw[1] ? JSON.parse(bufRaw[1]) : [],
  };
}

// --- métricas derivadas (espelham o clean.go do backend) ---

export function haversine(a: GPSPoint, b: GPSPoint): number {
  const R = 6371000;
  const φ1 = (a.lat * Math.PI) / 180;
  const φ2 = (b.lat * Math.PI) / 180;
  const dφ = ((b.lat - a.lat) * Math.PI) / 180;
  const dλ = ((b.lon - a.lon) * Math.PI) / 180;
  const h =
    Math.sin(dφ / 2) ** 2 + Math.cos(φ1) * Math.cos(φ2) * Math.sin(dλ / 2) ** 2;
  return R * 2 * Math.atan2(Math.sqrt(h), Math.sqrt(1 - h));
}

export type LiveStats = {
  distanceM: number;
  movingS: number;
  elapsedS: number;
  paceS: number; // s/km em movimento
  points: GPSPoint[];
  autoPaused: boolean; // parado há alguns segundos — o cronômetro de movimento congela
};

export function computeStats(points: GPSPoint[], startedAt: number): LiveStats {
  let dist = 0;
  let moving = 0;
  const kept: GPSPoint[] = [];
  let prev: GPSPoint | null = null;
  for (const p of points) {
    if (p.acc != null && p.acc > 30) continue;
    if (prev) {
      const d = haversine(prev, p);
      const dt = (p.t - prev.t) / 1000;
      if (dt <= 0) continue;
      if (d < 1 && dt < 1) continue;
      if (d / dt > 12) continue; // teleporte
      dist += d;
      if (d / dt > 0.5) moving += dt; // auto-pause: só conta tempo quando há deslocamento real
    }
    kept.push(p);
    prev = p;
  }
  const elapsedS = (Date.now() - startedAt) / 1000;
  const paceS = dist > 100 ? moving / (dist / 1000) : 0;

  // auto-pause: os últimos ~8 s de pontos ficaram num raio de ~6 m
  let autoPaused = false;
  if (kept.length >= 3) {
    const now = kept[kept.length - 1].t;
    const recent = kept.filter((p) => now - p.t <= 8000);
    if (recent.length >= 3) {
      const a = recent[0];
      autoPaused = recent.every((p) => haversine(a, p) < 6);
    }
  }
  return { distanceM: dist, movingS: moving, elapsedS, paceS, points: kept, autoPaused };
}
