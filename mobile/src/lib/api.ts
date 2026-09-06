// Cliente da API para o app. Tokens no SecureStore (Keychain / Keystore).
import * as SecureStore from "expo-secure-store";
import Constants from "expo-constants";

const BASE: string = (Constants.expoConfig?.extra as { apiUrl?: string })?.apiUrl ?? "http://localhost:8080";
const KEY = "fr.tokens";

export type Tokens = { access_token: string; refresh_token: string; expires_at: string };
export type PublicUser = { id: string; athlete_id: string; username: string; email: string; role: string };

export type RunPoint = { lat: number; lon: number; alt?: number; t: number; acc?: number; spd?: number };
export type RunView = {
  id: string;
  distance_m: number;
  moving_s: number;
  avg_pace_s: number;
  elevation_gain_m: number;
  territory_area_m2: number;
  new_blocks: number;
  status: string;
  started_at: string;
};
export type Lifetime = {
  run_count: number;
  total_distance_m?: number;
  total_moving_s?: number;
  territory_area_m2?: number;
  current_streak_days?: number;
};

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function getTokens(): Promise<Tokens | null> {
  const raw = await SecureStore.getItemAsync(KEY);
  return raw ? (JSON.parse(raw) as Tokens) : null;
}
async function setTokens(t: Tokens | null) {
  if (t) await SecureStore.setItemAsync(KEY, JSON.stringify(t));
  else await SecureStore.deleteItemAsync(KEY);
}
export async function isAuthed() {
  return (await getTokens()) !== null;
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const tokens = await getTokens();
  const isForm = typeof FormData !== "undefined" && init.body instanceof FormData;
  const headers: Record<string, string> = {
    ...(isForm ? {} : { "Content-Type": "application/json" }),
    ...(init.headers as Record<string, string>),
  };
  if (tokens) headers.Authorization = `Bearer ${tokens.access_token}`;

  const res = await fetch(`${BASE}${path}`, { ...init, headers });

  if (res.status === 401 && retry && tokens?.refresh_token) {
    const r = await fetch(`${BASE}/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: tokens.refresh_token }),
    });
    if (r.ok) {
      const { tokens: fresh } = (await r.json()) as { tokens: Tokens };
      await setTokens(fresh);
      return request<T>(path, init, false);
    }
    await setTokens(null);
  }

  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

export const api = {
  async register(input: { email: string; username: string; password: string }) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    });
    await setTokens(r.tokens);
    return r.user;
  },
  async login(email: string, password: string): Promise<{ mfa: false } | { mfa: true; mfaToken: string }> {
    const r = await request<
      { user: PublicUser; tokens: Tokens } | { mfa_required: true; mfa_token: string }
    >("/v1/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
    if ("mfa_required" in r) return { mfa: true, mfaToken: r.mfa_token };
    await setTokens(r.tokens);
    return { mfa: false };
  },

  async verifyMfa(mfaToken: string, code: string) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ mfa_token: mfaToken, code }),
    });
    await setTokens(r.tokens);
    return r.user;
  },
  async logout() {
    const t = await getTokens();
    await request("/v1/auth/logout", {
      method: "POST",
      body: JSON.stringify({ refresh_token: t?.refresh_token ?? "" }),
    }).catch(() => {});
    await setTokens(null);
  },
  me: () => request<PublicUser>("/v1/me"),

  uploadRun: (input: {
    started_at: string;
    ended_at: string;
    points: RunPoint[];
    cadence?: { t: number; v: number }[];
    mock_location?: boolean;
  }) =>
    request<RunView>("/v1/runs", {
      method: "POST",
      body: JSON.stringify({ data_source: "phone", gnss_mode: "phone", ...input }),
    }),

  run: (id: string) => request<RunView>(`/v1/runs/${id}`),
  runs: (limit = 15) => request<{ runs: RunView[] }>(`/v1/runs?limit=${limit}`),
  lifetime: () => request<Lifetime>("/v1/me/lifetime"),
  territories: () =>
    request<{ type: "FeatureCollection"; features: unknown[] }>("/v1/territories?scope=me"),
  coverage: () =>
    request<{ city: { pct: number; covered_cells: number; total_cells: number } }>("/v1/coverage"),

  landmarksProgress: () =>
    request<{
      total_landmarks: number;
      unlocked_count: number;
      landmarks: {
        id: string;
        name: string;
        radius_m: number;
        blurb?: string;
        lat: number;
        lng: number;
        checked_in: boolean;
      }[];
    }>("/v1/landmarks"),

  // check-in por foto: entra em moderação (plano §3). photoUri = arquivo da câmera in-app.
  landmarkCheckin: (id: string, lat: number, lng: number, photoUri: string, runId?: string) => {
    const form = new FormData();
    form.append("lat", String(lat));
    form.append("lng", String(lng));
    if (runId) form.append("run_id", runId);
    // @ts-expect-error RN aceita { uri, name, type } como parte do FormData
    form.append("photo", { uri: photoUri, name: "checkin.jpg", type: "image/jpeg" });
    return request<{ status: string }>(`/v1/landmarks/${encodeURIComponent(id)}/checkin`, {
      method: "POST",
      body: form,
    });
  },

  routes: () =>
    request<{
      routes: {
        id: string;
        name: string;
        description?: string;
        distance_m: number;
        surface: string;
        is_official: boolean;
        avg_rating: number;
      }[];
    }>("/v1/routes"),

  feed: () =>
    request<{
      feed: {
        id: string;
        actor_id: string;
        actor_name: string;
        type: string;
        subject_id: string;
        kudos_count: number;
        has_kudos: boolean;
        created_at: string;
      }[];
    }>("/v1/feed"),

  toggleKudos: (runId: string) =>
    request<{ kudosed: boolean }>(`/v1/runs/${encodeURIComponent(runId)}/kudos`, { method: "POST" }),

  clubs: () =>
    request<{
      clubs: {
        id: string;
        name: string;
        description?: string;
        color_hex: string;
        member_count: number;
        is_member: boolean;
      }[];
    }>("/v1/clubs"),

  joinClub: (id: string) =>
    request<{ joined: boolean }>(`/v1/clubs/${encodeURIComponent(id)}/join`, { method: "POST" }),

  // --- segurança ---
  safetyContacts: () =>
    request<{ contacts: { id: string; name: string; phone: string; relation?: string }[] }>(
      "/v1/safety/contacts",
    ),
  addSafetyContact: (name: string, phone: string, relation: string) =>
    request<{ id: string }>("/v1/safety/contacts", {
      method: "POST",
      body: JSON.stringify({ name, phone, relation }),
    }),
  deleteSafetyContact: (id: string) =>
    request<void>(`/v1/safety/contacts/${encodeURIComponent(id)}`, { method: "DELETE" }),
  triggerSOS: (lat: number, lng: number, note?: string, pin?: string, run_id?: string) =>
    request<{ id: string; share_token: string; share_url?: string; notified_contacts: number }>(
      "/v1/safety/sos",
      { method: "POST", body: JSON.stringify({ lat, lng, note, pin, run_id }) },
    ),
  cancelSOS: (id: string, pin?: string) =>
    request<{ status: string }>(`/v1/safety/sos/${encodeURIComponent(id)}/cancel`, {
      method: "POST",
      body: JSON.stringify({ pin }),
    }),
  updateBeacon: (id: string, lat: number, lng: number) =>
    request<void>(`/v1/safety/sos/${encodeURIComponent(id)}/beacon`, {
      method: "POST",
      body: JSON.stringify({ lat, lng }),
    }),

  // --- perigos na via ---
  hazards: (bbox?: string) =>
    request<{
      hazards: { id: string; type: string; lat: number; lng: number; severity: number; note?: string; confirms: number; disputes: number }[];
    }>(`/v1/hazards${bbox ? `?bbox=${encodeURIComponent(bbox)}` : ""}`),
  reportHazard: (type: string, lat: number, lng: number, severity: number, note?: string) =>
    request<{ id: string }>("/v1/hazards", {
      method: "POST",
      body: JSON.stringify({ type, lat, lng, severity, note }),
    }),

  // --- dispositivos & assinatura & perfil ---
  devices: () =>
    request<{ devices: { id: string; kind: string; brand?: string; model?: string }[] }>("/v1/devices"),
  addDevice: (kind: string, brand: string, model: string) =>
    request<{ id: string }>("/v1/devices", { method: "POST", body: JSON.stringify({ kind, brand, model }) }),
  deleteDevice: (id: string) =>
    request<void>(`/v1/devices/${encodeURIComponent(id)}`, { method: "DELETE" }),
  badges: () =>
    request<{ badges: { code: string; name: string; description?: string; earned_at: string }[] }>(
      "/v1/me/badges",
    ),
  subscription: () =>
    request<{ active: boolean; plan?: string; status?: string; current_period_end?: string }>(
      "/v1/me/subscription",
    ),
  subscribe: (plan: string) =>
    request<{ status: string }>("/v1/me/subscription", { method: "POST", body: JSON.stringify({ plan }) }),
  cancelSubscription: () => request<{ status: string }>("/v1/me/subscription", { method: "DELETE" }),
  coachSummary: () =>
    request<{ runs: number; distance_km: number; moving_hours: number; elevation_m: number; note: string }>(
      "/v1/coach/summary",
    ),

  events: () =>
    request<{
      events: {
        id: string;
        title: string;
        description: string;
        type: string;
        starts_at: string;
        ends_at: string;
        location_name: string;
        status: string;
        is_registered: boolean;
      }[];
    }>("/v1/events"),

  registerEvent: (id: string, category = "5k", shirt_size = "M") =>
    request<{ participant: unknown; message: string }>(`/v1/events/${encodeURIComponent(id)}/register`, {
      method: "POST",
      body: JSON.stringify({ category, shirt_size }),
    }),

  qrToken: (eventId?: string) =>
    request<{ qr: { id: string; payload_sig: string; expires_at: string } }>(
      `/v1/qr/token${eventId ? `?event_id=${encodeURIComponent(eventId)}` : ""}`
    ),

  askAICoach: (prompt: string) =>
    request<{
      coach: {
        id: string;
        message: string;
        tips: string[];
        source: "ia" | "fallback";
        disclaimer?: string;
        created_at: string;
      };
    }>("/v1/coach/ask", {
      method: "POST",
      body: JSON.stringify({ prompt }),
    }),
};


