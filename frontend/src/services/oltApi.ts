import type { Olt } from "../types/domain";
import { request } from "./api";
import { USE_MOCK_API } from "./client";

export interface OltInput {
  name: string;
  vendor: string;
  model: string;
  technology: "EPON" | "GPON";
  managementAddress: string;
  managementPort: number;
  connection: "ssh" | "telnet";
  enabled: boolean;
}

/** Never contains the secret — only who set it and when. */
export interface CredentialMeta {
  username: string;
  keyVersion: number;
  updatedAt: string;
  updatedBy: string | null;
}

/** Writes need the API. Mock mode says so instead of pretending to save. */
export const WRITES_AVAILABLE = !USE_MOCK_API;

const MOCK_WRITE_ERROR =
  "This build runs on mock data: creating or editing devices needs the API (set USE_MOCK_API = false).";

export async function createOlt(input: OltInput): Promise<Olt> {
  if (USE_MOCK_API) throw new Error(MOCK_WRITE_ERROR);
  return request<Olt>("/olts", { method: "POST", body: input });
}

export async function updateOlt(id: string, input: OltInput): Promise<Olt> {
  if (USE_MOCK_API) throw new Error(MOCK_WRITE_ERROR);
  return request<Olt>(`/olts/${id}`, { method: "PATCH", body: input });
}

/** 404 means "no credential stored yet", which is a normal state, not an error. */
export async function getCredentialMeta(id: string): Promise<CredentialMeta | null> {
  if (USE_MOCK_API) return null;
  try {
    return await request<CredentialMeta>(`/olts/${id}/credential`);
  } catch {
    return null;
  }
}

export async function setCredential(id: string, username: string, password: string): Promise<void> {
  if (USE_MOCK_API) throw new Error(MOCK_WRITE_ERROR);
  // The password travels once, over TLS, and is never sent back.
  await request<void>(`/olts/${id}/credential`, {
    method: "PUT",
    body: { username, password },
    timeoutMs: 15_000,
  });
}

export async function deleteCredential(id: string): Promise<void> {
  if (USE_MOCK_API) throw new Error(MOCK_WRITE_ERROR);
  await request<void>(`/olts/${id}/credential`, { method: "DELETE" });
}

export async function deleteOlt(id: string): Promise<void> {
  if (USE_MOCK_API) throw new Error(MOCK_WRITE_ERROR);
  await request<void>(`/olts/${id}`, { method: "DELETE" });
}

export interface CollectionPlan {
  adapter: string;
  commands: string[];
  lines: string[];
}

export interface CommandResult {
  id: string;
  line: string;
  ok: boolean;
  error?: string;
  summary?: string;
  raw?: string;
}


/** The exact commands a refresh would run — shown before anything is dialled. */
export async function getCollectionPlan(id: string): Promise<CollectionPlan> {
  if (USE_MOCK_API) {
    throw new Error("Refresh needs the API (set USE_MOCK_API = false).");
  }
  return request<CollectionPlan>(`/olts/${id}/collection-plan`);
}

export interface DiscoveredCommand {
  path: string;
  help: string;
  kind: "read" | "action" | "param" | "unknown";
  depth: number;
}

/** What the last crawl found on this device. */
export async function listCommands(id: string): Promise<DiscoveredCommand[]> {
  if (USE_MOCK_API) return [];
  return request<DiscoveredCommand[]>(`/olts/${id}/commands`);
}
