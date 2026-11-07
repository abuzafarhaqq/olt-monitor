import type { Role } from "./auth";
import { request } from "./api";
import { USE_MOCK_API } from "./client";

export interface UserRecord {
  id: string;
  username: string;
  fullName: string;
  email: string;
  phone: string | null;
  role: Role;
  disabled: boolean;
  lastLoginAt: string | null;
}

export interface UserInput {
  username: string;
  fullName: string;
  email: string;
  phone: string;
  role: Role;
  password: string;
}

export const USER_WRITES_AVAILABLE = !USE_MOCK_API;

const MOCK_ERROR =
  "This build runs on mock data: account changes need the API (set USE_MOCK_API = false).";

/** Mock mode shows the four demo accounts, read-only. */
const MOCK_USERS: UserRecord[] = [
  { id: "mock-1", username: "admin", fullName: "Demo Administrator", email: "admin@example.test", phone: null, role: "administrator", disabled: false, lastLoginAt: null },
  { id: "mock-2", username: "engineer", fullName: "Demo Engineer", email: "engineer@example.test", phone: null, role: "network-engineer", disabled: false, lastLoginAt: null },
  { id: "mock-3", username: "operator", fullName: "Demo Operator", email: "operator@example.test", phone: null, role: "noc-operator", disabled: false, lastLoginAt: null },
  { id: "mock-4", username: "viewer", fullName: "Demo Viewer", email: "viewer@example.test", phone: null, role: "viewer", disabled: false, lastLoginAt: null },
];

export async function listUsers(): Promise<UserRecord[]> {
  if (USE_MOCK_API) return MOCK_USERS;
  return request<UserRecord[]>("/users");
}

export async function createUser(input: UserInput): Promise<UserRecord> {
  if (USE_MOCK_API) throw new Error(MOCK_ERROR);
  return request<UserRecord>("/users", { method: "POST", body: input });
}

export async function updateUser(
  id: string,
  input: Omit<UserInput, "username" | "password"> & { disabled: boolean },
): Promise<UserRecord> {
  if (USE_MOCK_API) throw new Error(MOCK_ERROR);
  return request<UserRecord>(`/users/${id}`, { method: "PATCH", body: input });
}

export async function resetPassword(id: string, password: string): Promise<void> {
  if (USE_MOCK_API) throw new Error(MOCK_ERROR);
  await request<void>(`/users/${id}/password`, { method: "PUT", body: { password } });
}

export async function deleteUser(id: string): Promise<void> {
  if (USE_MOCK_API) throw new Error(MOCK_ERROR);
  await request<void>(`/users/${id}`, { method: "DELETE" });
}
