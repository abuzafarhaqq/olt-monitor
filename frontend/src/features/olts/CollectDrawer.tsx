import { useEffect, useState } from "react";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import {
  collectNow,
  getCollectionPlan,
  type CollectionPlan,
  type CollectionResult,
} from "../../services/oltApi";

interface CollectDrawerProps {
  oltId: string;
  oltName: string;
  onClose: () => void;
  /** Called after a successful run so the inventory reloads. */
  onDone: () => void;
}

/**
 * Refresh confirmation.
 *
 * The command list is fetched from the server and shown in full before anything
 * is dialled: this button opens an SSH session to a production OLT, and the
 * operator gets to read exactly what will run.
 */
export function CollectDrawer({ oltId, oltName, onClose, onDone }: CollectDrawerProps) {
  const [plan, setPlan] = useState<CollectionPlan | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<CollectionResult | null>(null);

  useEffect(() => {
    let alive = true;
    getCollectionPlan(oltId)
      .then((fetched) => {
        if (alive) setPlan(fetched);
      })
      .catch((caught: unknown) => {
        if (alive) setError(caught instanceof Error ? caught.message : "Could not load the plan");
      });
    return () => {
      alive = false;
    };
  }, [oltId]);

  const run = async () => {
    setRunning(true);
    setError(null);
    try {
      const collected = await collectNow(oltId);
      setResult(collected);
      onDone();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Collection failed");
    } finally {
      setRunning(false);
    }
  };

  return (
    <DetailDrawer
      title={`Refresh ${oltName}`}
      subtitle="Read-only commands. Nothing is configured or rebooted."
      onClose={onClose}
      footer={
        result === null ? (
          <>
            <button type="button" className="btn" onClick={onClose} disabled={running}>
              Cancel
            </button>
            <button
              type="button"
              className="btn btn--primary"
              onClick={run}
              disabled={running || plan === null}
            >
              {running ? "Collecting… (up to 90s)" : "Run collection"}
            </button>
          </>
        ) : (
          <button type="button" className="btn btn--primary" onClick={onClose}>
            Close
          </button>
        )
      }
    >
      {result === null ? (
        <>
          <p className="cell-sub">
            One SSH session, {plan?.commands.length ?? "…"} commands, then the result is written to
            the database in a single transaction.
          </p>

          {plan === null ? null : (
            <pre className="raw__pre" style={{ maxHeight: 320 }}>
              {plan.lines.map((line, index) => `${String(index + 1).padStart(2, " ")}  ${line}`).join("\n")}
            </pre>
          )}

          {running ? (
            <p className="cell-sub" role="status">
              Dialling {oltName}. The device answers slowly on optical queries; the request is held
              open until it finishes.
            </p>
          ) : null}
        </>
      ) : (
        <>
          <p className="auth-banner" role="status">
            <strong>Stored.</strong> Run #{result.runId} finished in {Math.round(result.tookMs / 1000)}s.
            The inventory and ONU pages reload from the database now.
          </p>
          <p className="cell-sub">
            {result.plan.commands.length} commands executed on {oltName}. Raw output is kept as
            evidence for every one of them.
          </p>

          <ul className="command-results">
            {result.results.map((command) => (
              <li key={command.id} className="command-result">
                <div className="command-result__head">
                  <span className={`badge ${command.ok ? "badge--online" : "badge--offline"}`}>
                    {command.ok ? "ok" : "failed"}
                  </span>
                  <code className="mono">{command.line}</code>
                  <span className="cell-sub">{command.summary ?? command.error ?? ""}</span>
                </div>
                {command.raw === undefined || command.raw === "" ? null : (
                  <details className="command-result__raw">
                    <summary>output</summary>
                    <pre className="raw__pre">{command.raw}</pre>
                  </details>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      {error === null ? null : (
        <p className="form__error" role="alert">
          {error}
        </p>
      )}
    </DetailDrawer>
  );
}
