import { request } from "./api";
import { USE_MOCK_API } from "./client";
import type { SessionUser } from "./authApi";

export interface SetupInput {
  username: string;
  fullName: string;
  email: string;
  password: string;
}

/** True while the platform has no accounts at all. */
export async function getSetupStatus(): Promise<boolean> {
  if (USE_MOCK_API) return false;
  try {
    const payload = await request<{ required: boolean }>("/setup/status");
    return payload.required;
  } catch {
    // An unreachable API is not "setup required"; the login page will say so.
    return false;
  }
}

/** Creates the first administrator and signs them in. */
export async function createFirstAdmin(input: SetupInput): Promise<SessionUser> {
  if (USE_MOCK_API) {
    throw new Error("Setup needs the API (set USE_MOCK_API = false).");
  }
  return request<SessionUser>("/setup", { method: "POST", body: input, timeoutMs: 20_000 });
}
