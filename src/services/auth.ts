/**
 * Roles and permissions (§5.10).
 *
 * This is a *rendering* aid. Hiding a button is not authorisation — the Go API
 * must enforce every permission again. The comment is the contract.
 */

export type Role = "administrator" | "network-engineer" | "noc-operator" | "viewer";

export type Permission =
  | "device.read"
  | "device.manage"
  | "diagnostics.read"
  | "diagnostics.run"
  | "incident.read"
  | "incident.acknowledge"
  | "incident.note"
  | "incident.resolve"
  | "reports.read"
  | "reports.export"
  | "settings.read"
  | "settings.manage"
  | "users.manage";

export interface RoleDefinition {
  id: Role;
  label: string;
  description: string;
}

export const ROLES: RoleDefinition[] = [
  {
    id: "administrator",
    label: "Administrator",
    description: "Full system management, including users and thresholds.",
  },
  {
    id: "network-engineer",
    label: "Network Engineer",
    description: "Diagnostics, incident resolution and device management.",
  },
  {
    id: "noc-operator",
    label: "NOC Operator",
    description: "Monitoring, acknowledgement, incident notes and exports.",
  },
  {
    id: "viewer",
    label: "Viewer",
    description: "Read-only dashboards and reports.",
  },
];

const READ_ONLY: Permission[] = [
  "device.read",
  "diagnostics.read",
  "incident.read",
  "reports.read",
];

export const ROLE_PERMISSIONS: Record<Role, Permission[]> = {
  viewer: [...READ_ONLY],
  "noc-operator": [...READ_ONLY, "incident.acknowledge", "incident.note", "reports.export"],
  "network-engineer": [
    ...READ_ONLY,
    "incident.acknowledge",
    "incident.note",
    "incident.resolve",
    "reports.export",
    "diagnostics.run",
    "device.manage",
    "settings.read",
  ],
  administrator: [
    ...READ_ONLY,
    "incident.acknowledge",
    "incident.note",
    "incident.resolve",
    "reports.export",
    "diagnostics.run",
    "device.manage",
    "settings.read",
    "settings.manage",
    "users.manage",
  ],
};

export function can(role: Role, permission: Permission): boolean {
  return ROLE_PERMISSIONS[role].includes(permission);
}

export interface CurrentUser {
  id: string;
  name: string;
  role: Role;
}

/**
 * Stand-in until the login endpoint exists. The role switcher in the top bar
 * exists so permissions can be exercised; it is not a security boundary.
 */
export const MOCK_USER: CurrentUser = {
  id: "user-1",
  name: "NOC Operator",
  role: "noc-operator",
};
