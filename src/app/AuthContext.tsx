import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import { MOCK_USER, can, type CurrentUser, type Permission, type Role } from "../services/auth";

interface AuthState {
  user: CurrentUser;
  /** Stand-in for sign-in: switches the role the UI renders as. */
  setRole: (role: Role) => void;
  can: (permission: Permission) => boolean;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser>(MOCK_USER);

  // Recreated only when the user changes, so consumers do not re-render
  // on every parent render.
  const value = useMemo<AuthState>(
    () => ({
      user,
      setRole: (role: Role) => setUser((current) => ({ ...current, role })),
      can: (permission: Permission) => can(user.role, permission),
    }),
    [user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const context = useContext(AuthContext);
  if (context === null) {
    // A silent default role would be a security bug disguised as convenience.
    throw new Error("useAuth must be used inside <AuthProvider>");
  }
  return context;
}
