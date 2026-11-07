import type { RawEvidence } from "../../types/domain";
import { formatDateTime } from "../../utils/format";

interface RawOutputViewerProps {
  evidence: RawEvidence[];
}

/**
 * Evidence is shown exactly as captured: monospaced, scrollable, no wrapping,
 * with the command, parser version and completeness beside it. If the parser
 * only matched half the rows, that fact travels with the output.
 */
export function RawOutputViewer({ evidence }: RawOutputViewerProps) {
  if (evidence.length === 0) {
    return <p className="cell-sub">No raw evidence was stored for this ONU.</p>;
  }

  return (
    <div className="raw-list">
      {evidence.map((entry) => (
        <section className="raw" key={entry.id}>
          <header className="raw__header">
            <code className="raw__command">{entry.command}</code>
            <span className="raw__meta tabular">
              {formatDateTime(entry.capturedAt)} · parser {entry.parserVersion} ·{" "}
              {Math.round(entry.completeness * 100)}% parsed
            </span>
          </header>
          <pre className="raw__pre">{entry.text}</pre>
          {entry.completeness < 1 ? (
            <p className="raw__warning">
              Incomplete capture — treat the missing rows as unknown, not as absent.
            </p>
          ) : null}
        </section>
      ))}
    </div>
  );
}
