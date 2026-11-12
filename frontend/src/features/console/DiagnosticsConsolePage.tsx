import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

import { FilterBar, Select } from "../../components/common/FilterBar";
import { PageHeader } from "../../components/common/PageHeader";
import { PermissionGate } from "../../components/common/PermissionGate";
import { useOverview } from "../../hooks/useResource";
import { USE_MOCK_API } from "../../services/client";
import { startCollection, streamRun } from "../../services/liveApi";

/**
 * Live terminal for a running collection.
 *
 * The browser never types into the device: this is a view of a stream the
 * server produces, and every command comes from the adapter's allow-list. Using
 * xterm rather than a <pre> matters because the device speaks vt100 — backspaces,
 * carriage returns and "--More--" would otherwise render as garbage.
 */
export function DiagnosticsConsolePage() {
  const { data } = useOverview();
  const [oltId, setOltId] = useState<string>("");
  const [runId, setRunId] = useState<number | null>(null);
  const [status, setStatus] = useState<"idle" | "running" | "done" | "error">("idle");
  const [error, setError] = useState<string | null>(null);

  const hostRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const stopRef = useRef<(() => void) | null>(null);

  // One terminal for the life of the page.
  useEffect(() => {
    if (hostRef.current === null) return;

    const term = new Terminal({
      convertEol: true,
      cursorBlink: false,
      disableStdin: true, // view-only: no keystroke can reach the device
      fontFamily: '"JetBrains Mono", "SFMono-Regular", Consolas, monospace',
      fontSize: 12,
      scrollback: 8000,
      theme: { background: "#0e1a2b", foreground: "#d6e2f0" },
      cols: 120,
      rows: 26,
    });
    term.open(hostRef.current);
    termRef.current = term;

    return () => {
      stopRef.current?.();
      term.dispose();
      termRef.current = null;
    };
  }, []);

  const write = (text: string) => termRef.current?.write(text);

  const run = async () => {
    const term = termRef.current;
    if (term === null || oltId === "") return;

    stopRef.current?.();
    term.reset();
    setError(null);
    setStatus("running");

    try {
      const started = await startCollection(oltId);
      setRunId(started.runId);
      write(`\x1b[90mrun #${started.runId} started\x1b[0m\r\n`);

      stopRef.current = streamRun(
        started.runId,
        (event) => {
          switch (event.kind) {
            case "cmd":
              // Sent lines are echoed by the device anyway; showing the prompt
              // first makes the stream readable as a session.
              write(`\r\n\x1b[36m# ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "out":
              write(event.data ?? "");
              break;
            case "info":
              write(`\r\n\x1b[32m· ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "start":
              write(`\x1b[90m${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "error":
              write(`\r\n\x1b[31m! ${event.data ?? ""}\x1b[0m\r\n`);
              break;
            case "end":
              write(`\r\n\x1b[32m■ run finished: ${event.data ?? "done"}\x1b[0m\r\n`);
              setStatus(event.data === "successful" ? "done" : "error");
              stopRef.current?.();
              stopRef.current = null;
              break;
          }
        },
        (message) => setError(message),
      );
    } catch (caught) {
      setStatus("error");
      const message = caught instanceof Error ? caught.message : "could not start the run";
      // 409 means a run for this OLT is already in flight.
      setError(message);
      write(`\x1b[31m${message}\x1b[0m\r\n`);
    }
  };

  return (
    <>
      <PageHeader
        title="Live terminal"
        subtitle="Read-only stream of a collection run. Commands come from the adapter's allow-list; nothing is typed from the browser."
        isMock={USE_MOCK_API}
      />

      <section className="card" aria-label="Terminal controls">
        <div className="card__body">
          <FilterBar summary={runId === null ? "" : `run #${runId} · ${status}`}>
            <Select
              id="console-olt"
              label="OLT"
              value={oltId}
              onChange={setOltId}
              options={[
                { value: "", label: "Select a device…" },
                ...(data?.olts ?? []).map((olt) => ({
                  value: olt.id,
                  label: `${olt.name} (${olt.technology})`,
                })),
              ]}
            />
            <PermissionGate
              permission="diagnostics.run"
              fallback={<span className="badge badge--unknown">read-only for your role</span>}
            >
              <button
                type="button"
                className="btn btn--primary"
                onClick={run}
                disabled={oltId === "" || status === "running"}
              >
                {status === "running" ? "Running…" : "Run collection"}
              </button>
            </PermissionGate>
            {status === "running" ? (
              <button
                type="button"
                className="btn btn--small"
                onClick={() => {
                  stopRef.current?.();
                  stopRef.current = null;
                  setStatus("idle");
                  write("\r\n\x1b[90mdetached from the stream (the run continues on the server)\x1b[0m\r\n");
                }}
              >
                Detach
              </button>
            ) : null}
          </FilterBar>
        </div>
      </section>

      <section className="card" aria-label="Terminal output">
        <div className="card__header">
          <h2 className="card__title">Output</h2>
          <span className="card__aside">
            {runId === null ? "no run yet" : `live · run #${runId}`}
          </span>
        </div>
        <div className="card__body">
          <div className="terminal-host" ref={hostRef} />
          {error === null ? null : (
            <p className="form__error" role="alert">
              {error}
            </p>
          )}
        </div>
      </section>

      <p className="page-footnote">
        Detaching closes the browser stream only — the collection keeps running server-side and its
        evidence lands in Logs. Starting a second run on the same device is refused while one is in
        flight.
      </p>
    </>
  );
}
