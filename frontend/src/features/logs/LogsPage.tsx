import { useState } from "react";
import { DataTable, type Column } from "../../components/common/DataTable";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import { EmptyState, ErrorState, LoadingState } from "../../components/common/States";
import { PageHeader } from "../../components/common/PageHeader";
import { useResource } from "../../hooks/useResource";
import { listRuns, runEvidence, type Evidence, type RunSummary } from "../../services/logsApi";
import { USE_MOCK_API } from "../../services/client";
import { formatDateTime, formatDuration } from "../../utils/format";

const RESULT_TONE: Record<RunSummary["result"], string> = {
  successful: "badge--online",
  partial: "badge--warning",
  failed: "badge--offline",
  skipped: "badge--unknown",
};

/** Everything the collector has stored: one row per run, evidence per command. */
export function LogsPage() {
  const { data: runs, loading, error } = useResource<RunSummary[]>(() => listRuns(100), []);

  const [openRun, setOpenRun] = useState<RunSummary | null>(null);
  const [evidence, setEvidence] = useState<Evidence[] | null>(null);
  const [evidenceError, setEvidenceError] = useState<string | null>(null);

  const open = (run: RunSummary) => {
    setOpenRun(run);
    setEvidence(null);
    setEvidenceError(null);
    runEvidence(run.id)
      .then(setEvidence)
      .catch((caught: unknown) =>
        setEvidenceError(caught instanceof Error ? caught.message : "Could not load evidence"),
      );
  };

  const columns: Column<RunSummary>[] = [
    {
      key: "started",
      header: "When",
      render: (run) => <span className="tabular">{formatDateTime(run.startedAt)}</span>,
    },
    { key: "olt", header: "OLT", render: (run) => run.oltName },
    {
      key: "result",
      header: "Result",
      render: (run) => <span className={`badge ${RESULT_TONE[run.result]}`}>{run.result}</span>,
    },
    {
      key: "duration",
      header: "Duration",
      align: "right",
      render: (run) => formatDuration(run.durationMs),
    },
    {
      key: "evidence",
      header: "Commands",
      align: "right",
      render: (run) => run.evidenceCount,
    },
    {
      key: "note",
      header: "Note",
      render: (run) => <span className="cell-sub">{run.errorMessage ?? "—"}</span>,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (run) => (
        <button type="button" className="btn btn--small" onClick={() => open(run)}>
          Open
        </button>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Logs"
        subtitle="Every collection run with the raw output it captured. Nothing here is summarised away."
        isMock={USE_MOCK_API}
      />

      {loading ? <LoadingState label="Loading runs…" /> : null}
      {error === null ? null : <ErrorState description={error} />}

      {runs === null || loading ? null : runs.length === 0 ? (
        <EmptyState
          title="No collection has run yet"
          description="Press Refresh on an OLT in the inventory; the run and its evidence land here."
        />
      ) : (
        <section className="card" aria-label="Collection runs">
          <div className="card__body card__body--flush">
            <DataTable
              columns={columns}
              rows={runs}
              getRowKey={(run) => String(run.id)}
              caption="Collection runs"
              emptyMessage="No run stored yet."
            />
          </div>
        </section>
      )}

      {openRun === null ? null : (
        <DetailDrawer
          title={`Run #${openRun.id} — ${openRun.oltName}`}
          subtitle={formatDateTime(openRun.startedAt)}
          onClose={() => setOpenRun(null)}
          footer={
            <button type="button" className="btn" onClick={() => setOpenRun(null)}>
              Close
            </button>
          }
        >
          {evidenceError === null ? null : (
            <p className="form__error" role="alert">
              {evidenceError}
            </p>
          )}

          {evidence === null && evidenceError === null ? (
            <LoadingState label="Loading evidence…" />
          ) : null}

          {evidence === null
            ? null
            : evidence.map((row) => (
                <div className="command-result" key={row.id} style={{ marginBottom: "var(--space-3)" }}>
                  <div className="command-result__head">
                    <code className="mono">{row.command}</code>
                    <span className="cell-sub">
                      {formatDateTime(row.capturedAt)} · {row.parserVersion} ·{" "}
                      {Math.round(row.completeness * 100)}% parsed
                    </span>
                  </div>
                  <pre className="raw__pre" style={{ maxHeight: 300 }}>
                    {row.body}
                  </pre>
                </div>
              ))}
        </DetailDrawer>
      )}
    </>
  );
}
