// Cliente HTTP mínimo do portal. Guarda os tokens no localStorage (Fase 0);
// migrar para cookie httpOnly + refresh silencioso na Fase 1.

const BASE = import.meta.env.VITE_API_URL ?? "";

type Tokens = { access_token: string; refresh_token: string; expires_at: string };

const KEY = "fr.tokens";

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
    /* modo privado */
  }
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const tokens = getTokens();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (tokens) headers.set("Authorization", `Bearer ${tokens.access_token}`);

  const res = await fetch(`${BASE}${path}`, { ...init, headers });

  if (res.status === 401 && retry && tokens?.refresh_token) {
    const refreshed = await tryRefresh(tokens.refresh_token);
    if (refreshed) return request<T>(path, init, false);
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
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
  constructor(public status: number, message: string) {
    super(message);
  }
}

export type PublicUser = {
  id: string;
  athlete_id: string;
  username: string;
  email: string;
  role: string;
};

export const api = {
  async register(input: { email: string; username: string; password: string; display_name?: string }) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setTokens(r.tokens);
    return r.user;
  },
  async login(email: string, password: string) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
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
  me() {
    return request<PublicUser>("/v1/me");
  },
};
