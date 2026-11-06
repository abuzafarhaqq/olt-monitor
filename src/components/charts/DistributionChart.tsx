export interface DistributionBucket {
  label: string;
  count: number;
  /** Tone of the dominant severity inside the bucket. */
  tone?: "neutral" | "warning" | "offline";
}

const TONE_FILL: Record<NonNullable<DistributionBucket["tone"]>, string> = {
  neutral: "var(--color-info)",
  warning: "var(--color-warning)",
  offline: "var(--color-offline)",
};

interface DistributionChartProps {
  buckets: DistributionBucket[];
  unitLabel?: string;
}

/** RX distribution. Bars, because each bucket is a count of ONUs. */
export function DistributionChart({ buckets, unitLabel = "ONUs" }: DistributionChartProps) {
  const maxCount = Math.max(...buckets.map((bucket) => bucket.count), 1);
  const slot = 100 / Math.max(buckets.length, 1);

  return (
    <div className="distribution">
      <div className="distribution__plot" role="img" aria-label={`${unitLabel} per RX bucket`}>
        {buckets.map((bucket) => (
          <div className="distribution__column" key={bucket.label}>
            <span className="distribution__count">{bucket.count}</span>
            <div
              className="distribution__bar"
              style={{
                height: `${(bucket.count / maxCount) * 100}%`,
                background: TONE_FILL[bucket.tone ?? "neutral"],
                width: `${Math.max(slot * 0.7, 4)}%`,
              }}
            />
          </div>
        ))}
      </div>
      <div className="distribution__labels">
        {buckets.map((bucket) => (
          <span className="distribution__label" key={bucket.label} style={{ width: `${slot}%` }}>
            {bucket.label}
          </span>
        ))}
      </div>
    </div>
  );
}
