import type { ReactNode } from "react";
import { useAuth } from "../../app/AuthContext";
import type { Permission } from "../../services/auth";

interface PermissionGateProps {
  permission: Permission;
  children: ReactNode;
  /** Rendered instead of the children when the role lacks the permission. */
  fallback?: ReactNode;
}

/**
 * Hides UI the current role cannot use. This is presentation only — the Go API
 * re-checks every permission, because the browser is not a trust boundary.
 */
export function PermissionGate({ permission, children, fallback = null }: PermissionGateProps) {
  const { can } = useAuth();
  return <>{can(permission) ? children : fallback}</>;
}
