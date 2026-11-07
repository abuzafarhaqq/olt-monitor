import type { DeviceTimestamps } from "../../types/domain";
import { formatDeviceTime, formatRelative } from "../../utils/format";

interface FreshnessIndicatorProps {
  timestamps: DeviceTimestamps | null;
}

/**
 * Two clocks, always visible when they disagree.
 *
 * ingestedAt is ours and can be trusted. deviceTime is the OLT's and often
 * cannot — the first VSOL EPON reports May 2000. Showing only one of them
 * would either hide stale data or present a wrong timestamp as fact.
 */
export function FreshnessIndicator({ timestamps }: FreshnessIndicatorProps) {
  if (timestamps === null) {
    return <span className="freshness__ingested">never collected</span>;
  }

  return (
    <span className="freshness">
      <span className="freshness__ingested">{formatRelative(timestamps.ingestedAt)}</span>
      {timestamps.quality === "untrusted" ? (
        <span className="freshness__device">
          device clock: {formatDeviceTime(timestamps.deviceTime)}
        </span>
      ) : null}
      {timestamps.quality === "missing" ? (
        <span className="freshness__device">device time not reported</span>
      ) : null}
    </span>
  );
}
