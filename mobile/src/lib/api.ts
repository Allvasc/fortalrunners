// Cliente da API para o app. Tokens no SecureStore (Keychain / Keystore).
import * as SecureStore from "expo-secure-store";
import Constants from "expo-constants";

const BASE: string = (Constants.expoConfig?.extra as { apiUrl?: string })?.apiUrl ?? "http://localhost:8080";
const KEY = "fr.tokens";

export type Tokens = { access_token: string; refresh_token: string; expires_at: string };
export type PublicUser = { id: string; athlete_id: string; username: string; email: string; role: string };

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
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
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
  async login(email: string, password: string) {
    const r = await request<{ user: PublicUser; tokens: Tokens }>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
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
};
