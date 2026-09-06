// Cliente da API para o app. Tokens no SecureStore (Keychain / Keystore).
import * as SecureStore from "expo-secure-store";
import * as WebBrowser from "expo-web-browser";
import Constants from "expo-constants";

// Ordem: env do build (EAS define via eas.json) → extra do app.json → localhost (dev).
const BASE: string =
  process.env.EXPO_PUBLIC_API_URL ||
  (Constants.expoConfig?.extra as { apiUrl?: string })?.apiUrl ||
  "http://localhost:8090";
const KEY = "fr.tokens";

export type Tokens = { access_token: string; refresh_token: string; expires_at: string };
export type PublicUser = { id: string; athlete_id: string; username: string; email: string; role: string };

export type GeoFC = { type: "FeatureCollection"; features: GeoJSON.Feature[] };

export type POI = {
  id: string;
  name: string;
  category: "bebedouro" | "banheiro" | "hidratacao" | "emergencia" | "sombra";
  lat: number;
  lng: number;
  note?: string;
};

export type WeatherReport = {
  city: string;
  temp_c: number;
  feels_like_c: number;
  humidity_pct: number;
  uv_index: number;
  wind_kmh: number;
  best_windows: string[];
  advice: string;
};

export type Friend = {
  id: string;
  athlete_id: string;
  username: string;
  name: string;
  status: "pending" | "accepted" | "blocked";
  incoming: boolean;
  since: string;
};

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
  total_steps?: number;
  territory_area_m2?: number;
  elevation_gain_m?: number;
  active_days?: number;
  current_streak_days?: number;
  longest_streak_days?: number;
  last_run_date?: string | null;
};

export type LeaderEntry = {
  rank: number;
  user_id: string;
  username: string;
  athlete_id: string;
  area_m2: number;
  blocks: number;
};
export type Leaderboard = { entries: LeaderEntry[]; me: LeaderEntry | null };

export type ChallengeView = {
  slug: string;
  title: string;
  description: string;
  cadence: "weekly" | "biweekly" | "monthly" | "oneoff";
  metric: string;
  goal?: number;
  period_no: number;
  starts_at: string;
  ends_at: string;
  my_value: number;
  my_rank: number;
  my_completed: boolean;
};
export type ChallengeStanding = {
  rank: number;
  user_id: string;
  username: string;
  athlete_id: string;
  value: number;
  completed: boolean;
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

/**
 * Login social: abre o provedor num browser in-app e captura os tokens do
 * deep link de volta (fortalrunners://auth/callback#access_token=...).
 * Lança em caso de cancelamento/erro.
 */
export async function oauthLogin(provider: "google" | "apple"): Promise<void> {
  const res = await fetch(`${BASE}/v1/auth/oauth/${provider}?dest=mobile`);
  if (!res.ok) throw new ApiError(res.status, "Login social indisponível");
  const { authorize_url } = (await res.json()) as { authorize_url: string };

  const result = await WebBrowser.openAuthSessionAsync(authorize_url, "fortalrunners://auth/callback");
  if (result.type !== "success" || !result.url) throw new ApiError(0, "Login cancelado");

  const frag = result.url.split("#")[1] ?? "";
  const kv: Record<string, string> = {};
  for (const part of frag.split("&")) {
    const [k, v] = part.split("=");
    if (k) kv[k] = decodeURIComponent(v ?? "");
  }
  if (!kv.access_token || !kv.refresh_token || !kv.expires_at) {
    throw new ApiError(0, "Resposta inválida do provedor");
  }
  await setTokens({
    access_token: kv.access_token,
    refresh_token: kv.refresh_token,
    expires_at: kv.expires_at,
  });
}

/**
 * Abre o WebSocket de eventos ao vivo (/v1/ws). `onEvent` recebe {type, data}.
 * Devolve uma função de close. Best-effort — a tela deve funcionar sem ele.
 */
export async function connectEvents(onEvent: (e: { type: string; data: any }) => void): Promise<() => void> {
  const t = await getTokens();
  if (!t) return () => {};
  const url = BASE.replace(/^http/, "ws") + `/v1/ws?token=${encodeURIComponent(t.access_token)}`;
  let ws: WebSocket | null = null;
  try {
    ws = new WebSocket(url);
    ws.onmessage = (m) => {
      try {
        onEvent(JSON.parse(String(m.data)));
      } catch {
        /* ignora frames não-JSON */
      }
    };
  } catch {
    /* sem WS, segue no polling */
  }
  return () => ws?.close();
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
    request<GeoFC>("/v1/territories?scope=me"),
  coverage: () =>
    request<{
      city: { pct: number; covered_cells: number; total_cells: number };
      neighborhoods?: { neighborhood_id: string; name: string; total_cells: number; covered_cells: number; pct: number }[];
    }>("/v1/coverage"),

  // --- ranking / leaderboards ---
  leaderboardGlobal: () => request<Leaderboard>("/v1/leaderboards/global"),
  leaderboardNeighborhood: (id: string) =>
    request<Leaderboard>(`/v1/leaderboards/neighborhood/${encodeURIComponent(id)}`),

  // --- desafios ---
  challenges: () => request<{ challenges: ChallengeView[] }>("/v1/challenges"),
  challengeLeaderboard: (slug: string) =>
    request<{ slug: string; period_no: number; closed: boolean; entries: ChallengeStanding[] }>(
      `/v1/challenges/${encodeURIComponent(slug)}/leaderboard`,
    ),

  // --- perfil ---
  records: () =>
    request<{ records: { distance_key: string; value_s: number; run_id: string | null; achieved_at: string | null }[] }>(
      "/v1/me/records",
    ),

  landmarksProgress: () =>
    request<{
      total_landmarks: number;
      unlocked_count: number;
      landmarks: {
        id: string;
        collection_id?: string;
        name: string;
        radius_m: number;
        blurb?: string;
        lat: number;
        lng: number;
        checked_in: boolean;
        checked_in_at?: string;
      }[];
      collections: {
        id: string;
        name: string;
        description?: string;
        reward_xp: number;
        total: number;
        unlocked: number;
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
        review_count?: number;
        geojson?: string;
      }[];
    }>("/v1/routes"),
  route: (id: string) =>
    request<{
      route: {
        id: string;
        name: string;
        description?: string;
        distance_m: number;
        surface: string;
        is_official: boolean;
        avg_rating: number;
      };
      reviews: { id: string; user_name: string; rating: number; body?: string; tags?: string[]; created_at: string }[];
    }>(`/v1/routes/${encodeURIComponent(id)}`),
  addRouteReview: (id: string, rating: number, body: string, tags: string[] = []) =>
    request<unknown>(`/v1/routes/${encodeURIComponent(id)}/reviews`, {
      method: "POST",
      body: JSON.stringify({ rating, body, tags }),
    }),

  // --- camadas do mapa ---
  amenities: () => request<{ amenities: POI[] }>("/v1/amenities"),
  riskZones: () => request<GeoFC>("/v1/risk-zones"),
  heatmap: (scope: "me" | "friends" | "city" = "me") => request<GeoFC>(`/v1/heatmap?scope=${scope}`),
  weather: () => request<WeatherReport>("/v1/weather/current"),

  // --- social: amigos ---
  friends: () => request<{ friends: Friend[] }>("/v1/friends"),
  sendFriendRequest: (target_id: string) =>
    request<{ status: string }>("/v1/friends/request", { method: "POST", body: JSON.stringify({ target_id }) }),
  acceptFriendRequest: (target_id: string) =>
    request<{ status: string }>("/v1/friends/accept", { method: "POST", body: JSON.stringify({ target_id }) }),
  removeFriend: (id: string) => request<void>(`/v1/friends/${encodeURIComponent(id)}`, { method: "DELETE" }),

  // --- clubes: criar ---
  createClub: (name: string, description: string, color_hex: string) =>
    request<{ id: string }>("/v1/clubs", {
      method: "POST",
      body: JSON.stringify({ name, description, color_hex }),
    }),

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

  // --- privacidade / LGPD ---
  setHomeZone: (lat: number, lng: number, radius_m: number) =>
    request<{ updated: boolean }>("/v1/me", {
      method: "PATCH",
      body: JSON.stringify({ home: { lat, lng, radius_m } }),
    }),
  exportMyData: () => request<Record<string, unknown>>("/v1/me/export"),
  deleteAccount: () =>
    request<void>("/v1/me", { method: "DELETE", body: JSON.stringify({ confirm: "EXCLUIR" }) }),
  territoriesScope: (scope: "me" | "friends") => request<GeoFC>(`/v1/territories?scope=${scope}`),
  leaderboardFriends: () => request<Leaderboard>("/v1/leaderboards/friends"),

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


