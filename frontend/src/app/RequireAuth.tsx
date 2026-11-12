import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { LoadingState } from "../components/common/States";
import { useAuth } from "./AuthContext";

/**
 * Gate for every authenticated route.
 *
 * While the session is being restored nothing renders as if signed in — a flash
 * of the dashboard before a redirect looks like a bug and mounts components
 * that fetch data.
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { status, setupRequired } = useAuth();
  const location = useLocation();

  if (status === "loading") {
    return <LoadingState label="Checking session…" />;
  }

  if (status === "anonymous") {
    // No accounts exist yet: there is nothing to sign in to.
    if (setupRequired) {
      return <Navigate to="/setup" replace />;
    }
    // Remember where the user was headed so login can return them there.
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }

  return <>{children}</>;
}
