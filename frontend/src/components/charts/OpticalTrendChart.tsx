import type { OpticalHistoryPoint } from "../../types/domain";
import { formatClock } from "../../utils/format";

const WIDTH = 720;
const HEIGHT = 220;
const PADDING = { top: 12, right: 56, bottom: 26, left: 44 };
const INNER_WIDTH = WIDTH - PADDING.left - PADDING.right;
const INNER_HEIGHT = HEIGHT - PADDING.top - PADDING.bottom;

interface OpticalTrendChartProps {
  points: OpticalHistoryPoint[];
  /** Model-specific thresholds. Passed in — they are never global. */
  warningDbm: number;
  criticalDbm: number;
}

/**
 * RX power over time, drawn as segments so a missing sample shows as a gap
 * instead of a straight line the operator would read as "fine".
 */
export function OpticalTrendChart({ points, warningDbm, criticalDbm }: OpticalTrendChartProps) {
  const values = points
    .map((point) => point.rxPowerDbm)
    .filter((value): value is number => value !== null);

  if (values.length === 0) {
    return <p className="cell-sub">No optical sample in the last 24 hours.</p>;
  }

  // Y range covers the data and both thresholds, never clipping a threshold line.
  const min = Math.floor(Math.min(...values, criticalDbm) - 2);
  const max = Math.ceil(Math.max(...values, warningDbm) + 2);

  const xFor = (index: number): number => PADDING.left + (index / Math.max(points.length - 1, 1)) * INNER_WIDTH;
  const yFor = (value: number): number =>
    PADDING.top + INNER_HEIGHT - ((value - min) / (max - min)) * INNER_HEIGHT;

  // Break the line wherever a sample is missing.
  const segments: { x: number; y: number }[][] = [];
  let current: { x: number; y: number }[] = [];
  points.forEach((point, index) => {
    if (point.rxPowerDbm === null) {
      if (current.length > 0) segments.push(current);
      current = [];
      return;
    }
    current.push({ x: xFor(index), y: yFor(point.rxPowerDbm) });
  });
  if (current.length > 0) segments.push(current);

  const yTicks = [min, Math.round((min + max) / 2), max];

  return (
    <svg
      className="trend__svg"
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      role="img"
      aria-label={`RX power over 24 hours with warning at ${warningDbm} dBm and critical at ${criticalDbm} dBm`}
    >
      {yTicks.map((tick) => (
        <g key={tick}>
          <line
            className="trend__grid-line"
            x1={PADDING.left}
            x2={WIDTH - PADDING.right}
            y1={yFor(tick)}
            y2={yFor(tick)}
          />
          <text className="trend__axis-label" x={PADDING.left - 6} y={yFor(tick) + 3} textAnchor="end">
            {tick}
          </text>
        </g>
      ))}

      <line
        x1={PADDING.left}
        x2={WIDTH - PADDING.right}
        y1={yFor(warningDbm)}
        y2={yFor(warningDbm)}
        stroke="var(--color-warning)"
        strokeWidth="1"
        strokeDasharray="4 3"
      />
      <text className="trend__axis-label" x={WIDTH - PADDING.right + 4} y={yFor(warningDbm) + 3} fill="var(--color-warning)">
        warn {warningDbm}
      </text>

      <line
        x1={PADDING.left}
        x2={WIDTH - PADDING.right}
        y1={yFor(criticalDbm)}
        y2={yFor(criticalDbm)}
        stroke="var(--color-offline)"
        strokeWidth="1"
        strokeDasharray="4 3"
      />
      <text className="trend__axis-label" x={WIDTH - PADDING.right + 4} y={yFor(criticalDbm) + 3} fill="var(--color-offline)">
        crit {criticalDbm}
      </text>

      {segments.map((segment, segmentIndex) => (
        <polyline
          key={segmentIndex}
          points={segment.map((point) => `${point.x},${point.y}`).join(" ")}
          fill="none"
          stroke="var(--color-info)"
          strokeWidth="2"
          strokeLinejoin="round"
        />
      ))}

      {points.map((point, index) =>
        index % 6 === 0 ? (
          <text
            key={`x-${point.at}`}
            className="trend__axis-label"
            x={xFor(index)}
            y={HEIGHT - 8}
            textAnchor="middle"
          >
            {formatClock(point.at)}
          </text>
        ) : null,
      )}
    </svg>
  );
}
