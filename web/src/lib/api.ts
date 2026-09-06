// Cliente HTTP do portal. Guarda os tokens no localStorage (Fase 1);
// migrar para cookie httpOnly + refresh silencioso quando o portal sair do MVP.

const BASE = import.meta.env.VITE_API_URL ?? "";

export type Tokens = { access_token: string; refresh_token: string; expires_at: string };

const KEY = "fr.tokens";
const listeners = new Set<() => void>();

export function onAuthChange(fn: () => void) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
function emit() {
  for (const fn of listeners) fn();
}

export function getTokens(): Tokens | null {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Tokens) : null;
  } catch {
    return null;
  }
}

function setTokens(t: Tokens | null) {
  try {
    if (t) localStorage.setItem(KEY, JSON.stringify(t));
    else localStorage.removeItem(KEY);
  } catch {
    /* modo privado — segue sem persistir */
  }
  emit();
}

export function isAuthed() {
  return !!getTokens();
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const tokens = getTokens();
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (tokens) headers.set("Authorization", `Bearer ${tokens.access_token}`);

  const res = await fetch(`${BASE}${path}`, { ...init, headers });

  if (res.status === 401 && retry && tokens?.refresh_token) {
    const refreshed = await tryRefresh(tokens.refresh_token);
    if (refreshed) return request<T>(path, init, false);
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}) as { error?: string });
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T);
}

async function tryRefresh(refresh_token: string): Promise<boolean> {
  const res = await fetch(`${BASE}/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token }),
  });
  if (!res.ok) {
    setTokens(null);
    return false;
  }
  const { tokens } = (await res.json()) as { tokens: Tokens };
  setTokens(tokens);
  return true;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

// --- tipos ---

export type PublicUser = {
  id: string;
  athlete_id: string;
  username: string;
  email: string;
  role: string;
};

export type LoginResult =
  | { kind: "ok"; user: PublicUser }
  | { kind: "mfa"; mfaToken: string };

export type TerritoryProps = {
  id: string;
  user_id: string;
  area_m2: number;
  neighborhood_id: string | null;
  claimed_at: string;
  status: string;
};
export type FeatureCollection = GeoJSON.FeatureCollection<GeoJSON.Geometry, TerritoryProps>;

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
export type ChallengeLeaderboard = {
  slug: string;
  period_no: number;
  closed: boolean;
  entries: ChallengeStanding[];
};

export type Lifetime = {
  total_moving_s?: number;
  total_distance_m?: number;
  total_steps?: number;
  run_count: number;
  elevation_gain_m?: number;
  territory_area_m2?: number;
  active_days?: number;
  current_streak_days?: number;
  longest_streak_days?: number;
  last_run_date?: string | null;
};

export type PersonalRecord = {
  key: string;
  value: number;
  run_id: string | null;
  achieved_at: string | null;
};

export type Run = {
  id: string;
  started_at: string;
  ended_at: string;
  distance_m: number;
  moving_s: number;
  duration_s: number;
  avg_pace_s: number;
  elevation_gain_m: number;
  data_source: string;
  territory_area_m2: number;
  new_blocks: number;
  status: string;
};

export type Split = {
  index: number;
  distance_m: number;
  elapsed_s: number;
  moving_s: number;
  pace_s_per_km: number;
  elev_gain_m: number;
  elev_loss_m: number;
  avg_cadence_spm?: number;
  avg_hr_bpm?: number;
};

export type RunMetrics = {
  run_id: string;
  splits: Split[];
  elev_gain_m: number;
  elev_loss_m: number;
  alt_min_m?: number;
  alt_max_m?: number;
  avg_cadence_spm?: number;
  max_cadence_spm?: number;
  avg_hr_bpm?: number;
  max_hr_bpm?: number;
  best_km_pace_s?: number;
  grade_adjusted_pace_s?: number;
  has_altitude: boolean;
  has_cadence: boolean;
  has_heart_rate: boolean;
};

export type AdminUser = {
  id: string;
  athlete_id: string;
  username: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
};

export type FlaggedRun = {
  id: string;
  user_id: string;
  username: string;
  started_at: string;
  distance_m: number;
  moving_s: number;
  avg_pace_s: number;
  fraud_score: number;
  fraud_flags: string[];
  status: string;
};

export type AuditEntry = {
  id: string;
  actor_role: string;
  action: string;
  target_type: string;
  target_id: string | null;
  created_at: string;
};

export type MFAStatus = {
  enabled: boolean;
  pending: boolean;
  activated_at?: string;
  recovery_codes_left: number;
};

// --- API ---

export const api = {
  async register(input: { email: string; username: string; password: string; display_name?: string }) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setTokens(r.tokens);
    return r.user;
  },

  async login(email: string, password: string): Promise<LoginResult> {
    const r = await request<
      { user: PublicUser; tokens: Tokens } | { mfa_required: true; mfa_token: string }
    >("/v1/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
    if ("mfa_required" in r) return { kind: "mfa", mfaToken: r.mfa_token };
    setTokens(r.tokens);
    return { kind: "ok", user: r.user };
  },

  async verifyMfa(mfaToken: string, code: string) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ mfa_token: mfaToken, code }),
    });
    setTokens(r.tokens);
    return r.user;
  },

  async logout() {
    const t = getTokens();
    await request("/v1/auth/logout", {
      method: "POST",
      body: JSON.stringify({ refresh_token: t?.refresh_token ?? "" }),
    }).catch(() => {});
    setTokens(null);
  },

  me: () => request<PublicUser>("/v1/me"),

  territories: (bbox?: [number, number, number, number]) =>
    request<FeatureCollection>(`/v1/territories${bbox ? `?bbox=${bbox.join(",")}` : ""}`),

  heatmap: (bbox?: [number, number, number, number]) =>
    request<GeoJSON.FeatureCollection<GeoJSON.Point, { w: number; hits: number }>>(
      `/v1/heatmap?scope=me${bbox ? `&bbox=${bbox.join(",")}` : ""}`,
    ),

  leaderboardGlobal: () => request<Leaderboard>("/v1/leaderboards/global"),

  coverage: () =>
    request<{
      city: { total_cells: number; covered_cells: number; pct: number };
      neighborhoods: { neighborhood_id: string; name: string; total_cells: number; covered_cells: number; pct: number }[];
    }>("/v1/coverage"),

  lifetime: () => request<Lifetime>("/v1/me/lifetime"),
  records: () => request<{ records: PersonalRecord[] }>("/v1/me/records"),

  runs: (limit = 30) => request<{ runs: Run[] }>(`/v1/runs?limit=${limit}`),
  runMetrics: (id: string) => request<RunMetrics>(`/v1/runs/${encodeURIComponent(id)}/metrics`),

  challenges: () => request<{ challenges: ChallengeView[] }>("/v1/challenges"),
  challengeLeaderboard: (slug: string) =>
    request<ChallengeLeaderboard>(`/v1/challenges/${encodeURIComponent(slug)}/leaderboard`),

  // --- admin (role admin/moderator + 2FA) ---
  adminUsers: (q: string) =>
    request<{ users: AdminUser[] }>(`/v1/admin/users?q=${encodeURIComponent(q)}`),
  adminSetUserStatus: (id: string, status: string, reason: string) =>
    request<void>(`/v1/admin/users/${id}/status`, {
      method: "POST",
      body: JSON.stringify({ status, reason }),
    }),
  adminFlaggedRuns: () => request<{ runs: FlaggedRun[] }>("/v1/admin/runs/flagged"),
  adminReviewRun: (id: string, decision: "valid" | "rejected", voidTerritory: boolean) =>
    request<void>(`/v1/admin/runs/${id}/review`, {
      method: "POST",
      body: JSON.stringify({ decision, void_territory: voidTerritory }),
    }),
  adminConfig: () => request<Record<string, unknown>>("/v1/admin/config"),
  adminSetConfig: (key: string, value: unknown) =>
    request<void>(`/v1/admin/config/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify(value),
    }),
  adminAudit: () => request<{ entries: AuditEntry[] }>("/v1/admin/audit?limit=50"),

  mfaStatus: () => request<MFAStatus>("/v1/auth/mfa"),
  mfaSetup: () => request<{ secret: string; otpauth_url: string }>("/v1/auth/mfa/setup", { method: "POST" }),
  mfaActivate: (code: string) =>
    request<{ recovery_codes: string[] }>("/v1/auth/mfa/activate", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  mfaDisable: (code: string) =>
    request<void>("/v1/auth/mfa/disable", { method: "POST", body: JSON.stringify({ code }) }),

  // --- landmarks & selos ---
  landmarksProgress: () => request<LandmarkProgress>("/v1/landmarks"),
  landmarkCheckin: (id: string, lat: number, lng: number) =>
    request<unknown>(`/v1/landmarks/${encodeURIComponent(id)}/checkin`, {
      method: "POST",
      body: JSON.stringify({ lat, lng }),
    }),

  // --- rotas ---
  routes: () => request<{ routes: RouteItem[] }>("/v1/routes"),
  routeDetail: (id: string) =>
    request<{ route: RouteItem; reviews: RouteReview[] }>(`/v1/routes/${encodeURIComponent(id)}`),
  addRouteReview: (id: string, rating: number, tags: string[], body: string) =>
    request<RouteReview>(`/v1/routes/${encodeURIComponent(id)}/reviews`, {
      method: "POST",
      body: JSON.stringify({ rating, tags, body }),
    }),

  // --- pontos de apoio ---
  amenities: (category?: string) =>
    request<{ amenities: POI[] }>(`/v1/amenities${category ? `?category=${category}` : ""}`),

  // --- social ---
  friends: () => request<{ friends: Friend[] }>("/v1/friends"),
  sendFriendRequest: (target_id: string) =>
    request<unknown>("/v1/friends/request", { method: "POST", body: JSON.stringify({ target_id }) }),
  acceptFriendRequest: (target_id: string) =>
    request<unknown>("/v1/friends/accept", { method: "POST", body: JSON.stringify({ target_id }) }),
  feed: () => request<{ feed: FeedEvent[] }>("/v1/feed"),
  toggleKudos: (runId: string) =>
    request<{ kudosed: boolean }>(`/v1/runs/${encodeURIComponent(runId)}/kudos`, { method: "POST" }),

  // --- clubes ---
  clubs: () => request<{ clubs: ClubItem[] }>("/v1/clubs"),
  createClub: (name: string, description: string, color_hex?: string) =>
    request<ClubItem>("/v1/clubs", { method: "POST", body: JSON.stringify({ name, description, color_hex }) }),
  joinClub: (id: string) =>
    request<{ joined: boolean }>(`/v1/clubs/${encodeURIComponent(id)}/join`, { method: "POST" }),

  // --- segurança ---
  safetyContacts: () => request<{ contacts: SafetyContact[] }>("/v1/safety/contacts"),
  addSafetyContact: (name: string, phone: string, relation?: string) =>
    request<SafetyContact>("/v1/safety/contacts", { method: "POST", body: JSON.stringify({ name, phone, relation }) }),
  triggerSOS: (lat: number, lng: number, note?: string) =>
    request<unknown>("/v1/safety/sos", { method: "POST", body: JSON.stringify({ lat, lng, note }) }),

  // --- clima ---
  weather: () => request<WeatherReport>("/v1/weather/current"),
};

export type Friend = {
  id: string;
  athlete_id: string;
  username: string;
  name: string;
  status: "pending" | "accepted" | "blocked";
  since: string;
};

export type FeedEvent = {
  id: string;
  actor_id: string;
  actor_name: string;
  type: "run" | "claim" | "badge" | "checkin";
  subject_id: string;
  payload: Record<string, unknown>;
  kudos_count: number;
  has_kudos: boolean;
  created_at: string;
};

export type ClubItem = {
  id: string;
  owner_id: string;
  name: string;
  description?: string;
  color_hex: string;
  member_count: number;
  is_member: boolean;
  created_at: string;
};

export type SafetyContact = {
  id: string;
  name: string;
  phone: string;
  relation?: string;
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


export type Landmark = {
  id: string;
  collection_id?: string;
  name: string;
  badge_code?: string;
  radius_m: number;
  blurb?: string;
  hero_photo_key?: string;
  difficulty: number;
  lat: number;
  lng: number;
  checked_in: boolean;
  checked_in_at?: string;
};

export type LandmarkCollection = {
  id: string;
  name: string;
  description?: string;
  badge_code?: string;
  reward_xp: number;
  total: number;
  unlocked: number;
};

export type LandmarkProgress = {
  total_landmarks: number;
  unlocked_count: number;
  landmarks: Landmark[];
  collections: LandmarkCollection[];
};

export type RouteItem = {
  id: string;
  created_by?: string;
  name: string;
  description?: string;
  distance_m: number;
  surface: string;
  is_official: boolean;
  geojson: string;
  avg_rating: number;
  review_count: number;
  created_at: string;
};

export type RouteReview = {
  id: string;
  route_id: string;
  user_id: string;
  user_name?: string;
  rating: number;
  tags: string[];
  body?: string;
  created_at: string;
};

export type POI = {
  id: string;
  city_id: string;
  name: string;
  category: "bebedouro" | "banheiro" | "hidratacao" | "emergencia" | "sombra";
  lat: number;
  lng: number;
  note?: string;
};

