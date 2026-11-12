import { useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { useAuth } from "../../app/AuthContext";
import { createFirstAdmin } from "../../services/setupApi";
import { USE_MOCK_API } from "../../services/client";

/**
 * First-run setup.
 *
 * This page exists for exactly one situation: no account exists. The API refuses
 * the endpoint the moment one does, so it cannot be used to add a second
 * administrator — the Users page owns that, with a session.
 */
export function SetupPage() {
  const { login, status, setupRequired } = useAuth();
  const navigate = useNavigate();

  const [username, setUsername] = useState("");
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  // Already initialised (or already signed in): this page has nothing to offer.
  if (status === "authenticated") {
    return <Navigate to="/" replace />;
  }
  if (status === "anonymous" && !setupRequired) {
    return <Navigate to="/login" replace />;
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await createFirstAdmin({ username, fullName, email, password });
      // The API signed us in with a cookie; adopt it in the app state.
      await login(username, password);
      setPassword("");
      navigate("/", { replace: true });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Setup failed");
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
            <h1 className="auth-title">First-time setup</h1>
            <p className="auth-subtitle">Create the administrator account</p>
          </div>
        </div>

        {USE_MOCK_API ? (
          <div className="auth-banner" role="note">
            <strong>Mock build.</strong> Setup needs the API; the demo accounts are
            admin / engineer / operator / viewer with password <code>demo</code>.
          </div>
        ) : null}

        <form className="auth-form" onSubmit={submit}>
          <label className="form__field">
            <span className="form__label">Full name</span>
            <input
              className="form__control"
              value={fullName}
              autoFocus
              onChange={(event) => setFullName(event.target.value)}
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Username</span>
            <input
              className="form__control mono"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Email</span>
            <input
              className="form__control"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </label>
          <label className="form__field">
            <span className="form__label">Password</span>
            <input
              className="form__control"
              type="password"
              autoComplete="new-password"
              value={password}
              minLength={8}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
            <span className="form__hint">
              At least 8 characters. This account is the administrator; add other users afterwards
              from Users &amp; Access.
            </span>
          </label>

          {error === null ? null : (
            <p className="form__error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn--primary auth-submit" disabled={pending}>
            {pending ? "Creating…" : "Create administrator"}
          </button>
        </form>
      </div>
    </main>
  );
}
