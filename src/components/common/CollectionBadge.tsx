import type { CollectionStatus } from "../../types/domain";

/**
 * Collection health, deliberately never red-for-failure: red means a confirmed
 * outage (§3). A failed collection is amber — degraded knowledge, not a dead ONU.
 */
const LABELS: Record<CollectionStatus, string> = {
  successful: "Collected",
  partial: "Partial",
  failed: "Failed",
  stale: "Stale",
  unsupported: "Unsupported",
};

const TONES: Record<CollectionStatus, string> = {
  successful: "badge--online",
  partial: "badge--warning",
  failed: "badge--warning",
  stale: "badge--unknown",
  unsupported: "badge--unknown",
};

interface CollectionBadgeProps {
  status: CollectionStatus;
}

export function CollectionBadge({ status }: CollectionBadgeProps) {
  return <span className={`badge ${TONES[status]}`}>{LABELS[status]}</span>;
}
