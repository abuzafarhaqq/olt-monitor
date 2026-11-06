import type { OpticalSeverity } from "../../constants/opticalThresholds";

/**
 * "unconfigured" and "missing" exist so that an absent judgement cannot be
 * mistaken for a healthy reading. They are grey, never green.
 */
const LABELS: Record<OpticalSeverity, string> = {
  ok: "OK",
  warning: "Low",
  critical: "Critical",
  missing: "No reading",
  unsupported: "Unsupported",
  unconfigured: "No threshold",
};

const TONES: Record<OpticalSeverity, string> = {
  ok: "badge--online",
  warning: "badge--warning",
  critical: "badge--offline",
  missing: "badge--unknown",
  unsupported: "badge--unknown",
  unconfigured: "badge--unknown",
};

interface OpticalBadgeProps {
  severity: OpticalSeverity;
}

export function OpticalBadge({ severity }: OpticalBadgeProps) {
  return <span className={`badge ${TONES[severity]}`}>{LABELS[severity]}</span>;
}
