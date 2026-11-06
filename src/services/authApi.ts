import { request } from "./api";
import type { Role } from "./auth";
import { USE_MOCK_API } from "./client";

export interface SessionUser {
  id: string;
  username: string;
  fullName: string;
  role: Role;
}

/**
 * Mock authentication, so the app is usable without a backend. It is labelled
 * as mock wherever it is reachable, and it stores no secret — only the demo
 * username, name and role.
 */
const MOCK_STORAGE_KEY = "oltmon.mock.session";
export const MOCK_PASSWORD = "demo";

const MOCK_USERS: Record<string, SessionUser> = {
  admin: { id: "mock-1", username: "admin", fullName: "Demo Administrator", role: "administrator" },
  engineer: {
    id: "mock-2",
    username: "engineer",
    fullName: "Demo Engineer",
    role: "network-engineer",
  },
  operator: { id: "mock-3", username: "operator", fullName: "Demo Operator", role: "noc-operator" },
  viewer: { id: "mock-4", username: "viewer", fullName: "Demo Viewer", role: "viewer" },
};

export const DEMO_ACCOUNTS = Object.keys(MOCK_USERS);

function readMockSession(): SessionUser | null {
  try {
    const raw = sessionStorage.getItem(MOCK_STORAGE_KEY);
    return raw === null ? null : (JSON.parse(raw) as SessionUser);
  } catch {
    // Corrupt or unavailable storage simply means "not signed in".
    return null;
  }
}

export async function login(username: string, password: string): Promise<SessionUser> {
  if (USE_MOCK_API) {
    await new Promise((resolve) => setTimeout(resolve, 250));
    const account = MOCK_USERS[username.trim().toLowerCase()];
    if (account === undefined || password !== MOCK_PASSWORD) {
      throw new Error("Invalid credentials");
    }
    sessionStorage.setItem(MOCK_STORAGE_KEY, JSON.stringify(account));
    return account;
  }

  return request<SessionUser>("/auth/login", {
    method: "POST",
    body: { username, password },
    timeoutMs: 15_000,
  });
}

export async function logout(): Promise<void> {
  if (USE_MOCK_API) {
    sessionStorage.removeItem(MOCK_STORAGE_KEY);
    return;
  }
  // A failed logout still clears the client; revoking the cookie is the
  // server's job and it must not block the UI from signing out.
  await request<void>("/auth/logout", { method: "POST" }).catch(() => undefined);
}

export async function currentUser(): Promise<SessionUser | null> {
  if (USE_MOCK_API) {
    return readMockSession();
  }
  try {
    return await request<SessionUser>("/auth/me");
  } catch {
    // 401 means "not signed in", which is not an error worth surfacing.
    return null;
  }
}

/**
 * The server answers 204 whether or not the address exists. The UI must say the
 * same thing either way, or the form becomes an account-enumeration tool.
 */
export async function requestPasswordReset(email: string): Promise<void> {
  if (USE_MOCK_API) {
    await new Promise((resolve) => setTimeout(resolve, 250));
    return;
  }
  await request<void>("/auth/password-reset", { method: "POST", body: { email } });
}
