import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { requestPasswordReset } from "../../services/authApi";
import { USE_MOCK_API } from "../../services/client";

/**
 * Self-service reset without a mailer.
 *
 * The request is recorded for an administrator, who resets the password with
 * `userctl passwd`. The wording is identical whether or not the address exists —
 * otherwise this page enumerates accounts.
 */
export function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      await requestPasswordReset(email);
      setSent(true);
    } catch {
      setError("Could not send the request. Try again, or contact your administrator.");
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
            <h1 className="auth-title">Reset password</h1>
            <p className="auth-subtitle">OLT Diagnostics</p>
          </div>
        </div>

        {sent ? (
          <>
            <p className="auth-banner" role="status">
              <strong>Request recorded.</strong> If that address belongs to an account, an
              administrator will reset the password and contact you.
            </p>
            <p className="cell-sub">
              Administrators reset passwords on the server with{" "}
              <code className="mono">userctl passwd -username &lt;name&gt;</code>, which also signs the
              account out everywhere. Email delivery is not enabled yet.
            </p>
          </>
        ) : (
          <form className="auth-form" onSubmit={submit}>
            <label className="form__field">
              <span className="form__label">Email address</span>
              <input
                className="form__control"
                type="email"
                name="email"
                autoComplete="email"
                autoFocus
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </label>

            {error === null ? null : (
              <p className="form__error" role="alert">
                {error}
              </p>
            )}

            <button type="submit" className="btn btn--primary auth-submit" disabled={pending}>
              {pending ? "Sending…" : "Request reset"}
            </button>

            {USE_MOCK_API ? (
              <p className="cell-sub">Mock mode: nothing is sent and nothing is recorded.</p>
            ) : null}
          </form>
        )}

        <div className="auth-footer">
          <Link className="link-strong" to="/login">
            ← Back to sign in
          </Link>
        </div>
      </div>
    </main>
  );
}
