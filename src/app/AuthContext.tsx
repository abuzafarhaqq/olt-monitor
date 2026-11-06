import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { can, type Permission } from "../services/auth";
import {
  currentUser,
  login as loginRequest,
  logout as logoutRequest,
  type SessionUser,
} from "../services/authApi";

type AuthStatus = "loading" | "anonymous" | "authenticated";

interface AuthState {
  user: SessionUser | null;
  status: AuthStatus;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  can: (permission: Permission) => boolean;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [status, setStatus] = useState<AuthStatus>("loading");

  // Restore the session once on mount: a reload must not silently sign someone
  // out, and a stale cookie must not silently sign them in.
  useEffect(() => {
    let alive = true;

    currentUser()
      .then((session) => {
        if (!alive) return;
        setUser(session);
        setStatus(session === null ? "anonymous" : "authenticated");
      })
      .catch(() => {
        if (!alive) return;
        setUser(null);
        setStatus("anonymous");
      });

    return () => {
      alive = false;
    };
  }, []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      status,
      login: async (username: string, password: string) => {
        const session = await loginRequest(username, password);
        setUser(session);
        setStatus("authenticated");
      },
      logout: async () => {
        await logoutRequest();
        setUser(null);
        setStatus("anonymous");
      },
      // No user means no permission. Defaulting to "allowed" would render
      // controls an anonymous viewer must never see.
      can: (permission: Permission) => (user === null ? false : can(user.role, permission)),
    }),
    [user, status],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const context = useContext(AuthContext);
  if (context === null) {
    throw new Error("useAuth must be used inside <AuthProvider>");
  }
  return context;
}
