import { useEffect, useRef, useState } from "react";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import { LiveTerminal, type LiveTerminalHandle } from "../../components/common/LiveTerminal";
import { getCollectionPlan, type CollectionPlan } from "../../services/oltApi";
import { startCollection, streamRun } from "../../services/liveApi";

interface CollectDrawerProps {
  oltId: string;
  oltName: string;
  onClose: () => void;
  /** Called when the run finishes, so the inventory picks up the new timestamp. */
  onDone: () => void;
}

/**
 * Refresh confirmation with a live view of the run.
 *
 * Collection is asynchronous: the request returns 202 with a run id and the work
 * happens on the server. This drawer subscribes to that run's SSE stream, so
 * what you read here is the same bytes the device sent — the full transcript
 * stays in Logs afterwards.
 */
export function CollectDrawer({ oltId, oltName, onClose, onDone }: CollectDrawerProps) {
  const [plan, setPlan] = useState<CollectionPlan | null>(null);
  const [runId, setRunId] = useState<number | null>(null);
  const termRef = useRef<LiveTerminalHandle | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [state, setState] = useState<"plan" | "running" | "done">("plan");

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

  const append = (text: string) => termRef.current?.write(text);

  const run = async () => {
    setState("running");
    setError(null);
    termRef.current?.reset();

    try {
      const started = await startCollection(oltId);
      setRunId(started.runId);
      append(`\x1b[90mrun #${started.runId} on ${oltName}\x1b[0m\r\n`);

      streamRun(
        started.runId,
        (event) => {
          switch (event.kind) {
            case "cmd":
              append(`\r\n\x1b[36m# ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "out":
              append(event.data ?? "");
              break;
            case "info":
            case "start":
              append(`\r\n\x1b[32m· ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "error":
              append(`\r\n\x1b[31m! ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "truncated":
              termRef.current?.reset();
              append(`\x1b[33m--- missed ${event.data ?? "some"} events; view reset ---\x1b[0m\r\n`);
              break;
            case "end":
              append(`\r\n\x1b[32m■ finished: ${event.data ?? "done"}\x1b[0m\r\n`);
              setState("done");
              onDone();
              break;
          }
        },
        (message) => setError(message),
      );
    } catch (caught) {
      setState("plan");
      setError(caught instanceof Error ? caught.message : "Could not start the run");
    }
  };

  return (
    <DetailDrawer
      title={`Refresh ${oltName}`}
      subtitle={
        state === "running"
          ? `live · run #${runId ?? "…"}`
          : "Read-only commands. Nothing is configured or rebooted."
      }
      onClose={onClose}
      footer={
        state === "plan" ? (
          <>
            <button type="button" className="btn" onClick={onClose}>
              Cancel
            </button>
            <button
              type="button"
              className="btn btn--primary"
              onClick={run}
              disabled={plan === null}
            >
              Run collection
            </button>
          </>
        ) : state === "running" ? (
          <button type="button" className="btn" onClick={onClose}>
            Close (the run continues)
          </button>
        ) : (
          <button type="button" className="btn btn--primary" onClick={onClose}>
            Done
          </button>
        )
      }
    >
      {state === "plan" ? (
        <>
          <p className="cell-sub">
            One SSH session, {plan?.commands.length ?? "…"} commands, then the result is written to
            the database. The run keeps going if you close this panel.
          </p>
          {plan === null ? null : (
            <pre className="raw__pre" style={{ maxHeight: 300 }}>
              {plan.lines.map((line, index) => `${String(index + 1).padStart(2, " ")}  ${line}`).join("\n")}
            </pre>
          )}
        </>
      ) : (
        <LiveTerminal ref={termRef} rows={18} />
      )}

      {error === null ? null : (
        <p className="form__error" role="alert">
          {error}
        </p>
      )}
    </DetailDrawer>
  );
}
