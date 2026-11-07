import { useState } from "react";
import { FilterBar, Select } from "../../components/common/FilterBar";
import { PageHeader } from "../../components/common/PageHeader";
import { PermissionGate } from "../../components/common/PermissionGate";
import { MOCK_OVERVIEW } from "../../mock/mockData";
import { executeDiagnostic, USE_MOCK_API } from "../../services/client";
import { DIAGNOSTIC_COMMANDS } from "../../services/mockBackend";
import type { DiagnosticResult } from "../../types/domain";
import { formatDateTime, formatDuration } from "../../utils/format";

type RunState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "done"; result: DiagnosticResult }
  | { status: "error"; message: string };

/**
 * Read-only console (§5.9). The operator picks an OLT and an approved command
 * id; the backend maps that id to CLI. No free-text command reaches a device
 * from a browser, and every run keeps its evidence reference.
 */
export function DiagnosticsConsolePage() {
  const data = MOCK_OVERVIEW;
  const [oltId, setOltId] = useState(data.olts[0]?.id ?? "");
  const [commandId, setCommandId] = useState(DIAGNOSTIC_COMMANDS[0].id);
  const [run, setRun] = useState<RunState>({ status: "idle" });

  const olt = data.olts.find((candidate) => candidate.id === oltId);
  const commands = DIAGNOSTIC_COMMANDS.filter(
    (command) => command.technology === "any" || command.technology === olt?.technology,
  );

  const selectedCommand =
    commands.find((command) => command.id === commandId) ?? commands[0];

  const execute = async () => {
    setRun({ status: "running" });
    try {
      const result = await executeDiagnostic(oltId, selectedCommand.id);
      setRun({ status: "done", result });
    } catch (error) {
      setRun({
        status: "error",
        message: error instanceof Error ? error.message : "Unknown error",
      });
    }
  };

  return (
    <>
      <PageHeader
        title="Diagnostics Console"
        subtitle={`Read-only · adapter-approved commands only${USE_MOCK_API ? " · mock backend" : ""}`}
        isMock
      />

      <section className="card" aria-label="Console controls">
        <div className="card__body">
          <FilterBar>
            <Select
              id="console-olt"
              label="OLT"
              value={oltId}
              onChange={setOltId}
              options={data.olts.map((candidate) => ({
                value: candidate.id,
                label: `${candidate.name} (${candidate.technology})`,
              }))}
            />
            <Select
              id="console-command"
              label="Command"
              value={selectedCommand.id}
              onChange={setCommandId}
              options={commands.map((command) => ({
                value: command.id,
                label: `${command.label} — ${command.command}`,
              }))}
            />

            <PermissionGate
              permission="diagnostics.run"
              fallback={
                <span className="badge badge--unknown" title="Your role cannot run diagnostics">
                  read-only for your role
                </span>
              }
            >
              <button
                type="button"
                className="btn btn--primary"
                onClick={execute}
                disabled={run.status === "running" || olt?.reachable === false}
              >
                {run.status === "running" ? "Running…" : "Run"}
              </button>
            </PermissionGate>

            {olt?.reachable === false ? (
              <span className="badge badge--offline">OLT unreachable</span>
            ) : null}
          </FilterBar>
        </div>
      </section>

      <section className="card" aria-label="Output">
        <div className="card__header">
          <h2 className="card__title">Output</h2>
          <span className="card__aside">
            {run.status === "done"
              ? `${formatDuration(run.result.durationMs)} · ${run.result.result} · ${formatDateTime(run.result.startedAt)}`
              : "nothing executed yet"}
          </span>
        </div>
        <div className="card__body">
          {run.status === "idle" ? (
            <p className="cell-sub">
              Pick a device and an approved command, then run it. Output is captured with its
              evidence reference.
            </p>
          ) : null}

          {run.status === "running" ? <p className="cell-sub">Waiting for the device prompt…</p> : null}

          {run.status === "error" ? (
            <p className="form__error" role="alert">
              {run.message}
            </p>
          ) : null}

          {run.status === "done" ? (
            <>
              <div className="console-meta">
                <code className="mono">{run.result.command}</code>
                <span className={`badge ${run.result.result === "ok" ? "badge--online" : "badge--warning"}`}>
                  {run.result.result}
                </span>
                <span className="cell-sub mono">{run.result.evidenceRef}</span>
              </div>
              <pre className="raw__pre console-output">{run.result.text}</pre>
            </>
          ) : null}
        </div>
      </section>

      <p className="page-footnote">
        Configuration mode and arbitrary CLI are deliberately absent. Only the ids above are
        accepted, and the server maps them through a vendor adapter.
      </p>
    </>
  );
}
