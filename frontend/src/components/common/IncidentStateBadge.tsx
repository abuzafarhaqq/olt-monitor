import { STATE_LABELS } from "../../constants/incidentWorkflow";
import type { IncidentState } from "../../types/domain";

/**
 * Purple means "someone is on it" — acknowledged or investigating. Green is
 * resolved, grey is closed. New is blue: a case nobody has picked up yet.
 */
const TONES: Record<IncidentState, string> = {
  new: "badge--info",
  acknowledged: "badge--investigating",
  investigating: "badge--investigating",
  resolved: "badge--online",
  closed: "badge--unknown",
};

interface IncidentStateBadgeProps {
  state: IncidentState;
}

export function IncidentStateBadge({ state }: IncidentStateBadgeProps) {
  return <span className={`badge ${TONES[state]}`}>{STATE_LABELS[state]}</span>;
}
