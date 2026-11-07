import type { IncidentState } from "../types/domain";

/**
 * The workflow is a graph, not a dropdown. Encoding the legal moves here means
 * a button cannot offer a transition the backend would reject.
 */
export const ALLOWED_TRANSITIONS: Record<IncidentState, IncidentState[]> = {
  new: ["acknowledged"],
  acknowledged: ["investigating", "resolved"],
  investigating: ["resolved"],
  resolved: ["closed"],
  closed: [],
};

export const STATE_LABELS: Record<IncidentState, string> = {
  new: "New",
  acknowledged: "Acknowledged",
  investigating: "Investigating",
  resolved: "Resolved",
  closed: "Closed",
};

/** A suggested action label per target state. */
export const TRANSITION_LABELS: Record<IncidentState, string> = {
  new: "Reopen",
  acknowledged: "Acknowledge",
  investigating: "Investigate",
  resolved: "Resolve",
  closed: "Close",
};
