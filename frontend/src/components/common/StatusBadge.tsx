import type { OnuStatus } from "../../types/domain";

/**
 * Operational status only. Collection health has its own badge on purpose —
 * a UI that shows one merged "status" cannot tell an outage from a blindspot.
 */
const LABELS: Record<OnuStatus, string> = {
  online: "Online",
  offline: "Offline",
  unknown: "Unknown",
};

const TONES: Record<OnuStatus, string> = {
  online: "badge--online",
  offline: "badge--offline",
  unknown: "badge--unknown",
};

interface StatusBadgeProps {
  status: OnuStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <span className={`badge ${TONES[status]}`}>
      <span className="badge__dot" aria-hidden="true" />
      {LABELS[status]}
    </span>
  );
}
