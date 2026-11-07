import { useState, type FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../../app/AuthContext";
import { DEMO_ACCOUNTS, MOCK_PASSWORD } from "../../services/authApi";
import { USE_MOCK_API } from "../../services/client";

interface LocationState {
  from?: string;
}

export function LoginPage() {
  const { login, status } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as LocationState | null)?.from ?? "/";

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  // Already signed in: redirect declaratively rather than in an effect.
  if (status === "authenticated") {
    return <Navigate to={from} replace />;
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await login(username, password);
      // The password does not survive a successful submit.
      setPassword("");
      navigate(from, { replace: true });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Sign-in failed");
      setPassword("");
    } finally {
      setPending(false);
    }
  };

  return (
    <main className="auth-shell">
      <div className="auth-card">
        <div className="auth-brand">
          <span className="sidebar__brand-mark" aria-hidden="true">
            OLT
          </span>
          <div>
            <h1 className="auth-title">OLT Diagnostics</h1>
            <p className="auth-subtitle">ISP NOC · sign in to continue</p>
          </div>
        </div>

        {USE_MOCK_API ? (
          <div className="auth-banner" role="note">
            <strong>Mock sign-in — no server is running.</strong> Any of these accounts works with
            the password <code>{MOCK_PASSWORD}</code>: {DEMO_ACCOUNTS.join(", ")}.
          </div>
        ) : null}

        <form className="auth-form" onSubmit={submit}>
          <label className="form__field">
            <span className="form__label">Username</span>
            <input
              className="form__control"
              name="username"
              autoComplete="username"
              autoFocus
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>

          <label className="form__field">
            <span className="form__label">Password</span>
            <input
              className="form__control"
              type="password"
              name="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>

          {error === null ? null : (
            <p className="form__error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn--primary auth-submit" disabled={pending}>
            {pending ? "Signing in…" : "Sign in"}
          </button>
        </form>

        <div className="auth-footer">
          <Link className="link-strong" to="/forgot-password">
            Forgot password?
          </Link>
          <span className="cell-sub">Roles and permissions are enforced by the API.</span>
        </div>
      </div>
    </main>
  );
}
