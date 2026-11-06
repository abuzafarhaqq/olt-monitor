import type { CollectionJobResult } from "../../types/domain";

/**
 * A job result is not an ONU status. "failed" here means we learned nothing —
 * amber, never red. Red stays reserved for a confirmed outage (§3).
 */
const LABELS: Record<CollectionJobResult, string> = {
  successful: "Success",
  partial: "Partial",
  failed: "Failed",
  skipped: "Skipped",
};

const TONES: Record<CollectionJobResult, string> = {
  successful: "badge--online",
  partial: "badge--warning",
  failed: "badge--warning",
  skipped: "badge--unknown",
};

interface JobResultBadgeProps {
  result: CollectionJobResult;
}

export function JobResultBadge({ result }: JobResultBadgeProps) {
  return <span className={`badge ${TONES[result]}`}>{LABELS[result]}</span>;
}
