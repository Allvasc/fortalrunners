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

  leaderboardGlobal: () => request<Leaderboard>("/v1/leaderboards/global"),

  lifetime: () => request<Lifetime>("/v1/me/lifetime"),
  records: () => request<{ records: PersonalRecord[] }>("/v1/me/records"),

  challenges: () => request<{ challenges: ChallengeView[] }>("/v1/challenges"),
  challengeLeaderboard: (slug: string) =>
    request<ChallengeLeaderboard>(`/v1/challenges/${encodeURIComponent(slug)}/leaderboard`),

  mfaStatus: () => request<MFAStatus>("/v1/auth/mfa"),
  mfaSetup: () => request<{ secret: string; otpauth_url: string }>("/v1/auth/mfa/setup", { method: "POST" }),
  mfaActivate: (code: string) =>
    request<{ recovery_codes: string[] }>("/v1/auth/mfa/activate", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  mfaDisable: (code: string) =>
    request<void>("/v1/auth/mfa/disable", { method: "POST", body: JSON.stringify({ code }) }),
};
