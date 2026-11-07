import type { HistoryPoint } from "../../mock/history";

const WIDTH = 760;
const HEIGHT = 220;
const PADDING = { top: 12, right: 12, bottom: 26, left: 46 };
const INNER_WIDTH = WIDTH - PADDING.left - PADDING.right;
const INNER_HEIGHT = HEIGHT - PADDING.top - PADDING.bottom;

interface TimeSeriesChartProps {
  current: HistoryPoint[];
  /** Same-length previous period, aligned by index — dates differ. */
  previous?: HistoryPoint[];
  unit: string;
  precision: number;
}

/**
 * Two series, one scale. The previous period is dashed on purpose: it shares
 * the axis but not the dates, so it must not look like the same line.
 */
export function TimeSeriesChart({ current, previous, unit, precision }: TimeSeriesChartProps) {
  if (current.length === 0) {
    return <p className="cell-sub">No data in this range.</p>;
  }

  const allValues = [...current, ...(previous ?? [])].map((point) => point.value);
  const min = Math.min(...allValues);
  const max = Math.max(...allValues);
  const span = max - min || 1;
  const padded = { min: min - span * 0.1, max: max + span * 0.1 };

  const xFor = (index: number, length: number): number =>
    PADDING.left + (index / Math.max(length - 1, 1)) * INNER_WIDTH;
  const yFor = (value: number): number =>
    PADDING.top + INNER_HEIGHT - ((value - padded.min) / (padded.max - padded.min || 1)) * INNER_HEIGHT;

  const toPolyline = (points: HistoryPoint[]): string =>
    points.map((point, index) => `${xFor(index, points.length)},${yFor(point.value)}`).join(" ");

  const ticks = [padded.min, (padded.min + padded.max) / 2, padded.max];

  return (
    <div className="trend">
      <svg
        className="trend__svg"
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        role="img"
        aria-label={`Time series in ${unit}${previous ? ", compared with the previous period" : ""}`}
      >
        {ticks.map((tick) => (
          <g key={tick}>
            <line
              className="trend__grid-line"
              x1={PADDING.left}
              x2={WIDTH - PADDING.right}
              y1={yFor(tick)}
              y2={yFor(tick)}
            />
            <text
              className="trend__axis-label"
              x={PADDING.left - 6}
              y={yFor(tick) + 3}
              textAnchor="end"
            >
              {tick.toFixed(precision)}
            </text>
          </g>
        ))}

        {previous ? (
          <polyline
            points={toPolyline(previous)}
            fill="none"
            stroke="var(--color-text-muted)"
            strokeWidth="1.5"
            strokeDasharray="5 4"
            opacity="0.8"
          />
        ) : null}

        <polyline
          points={toPolyline(current)}
          fill="none"
          stroke="var(--color-info)"
          strokeWidth="2"
        />

        <text className="trend__axis-label" x={PADDING.left} y={HEIGHT - 8} textAnchor="start">
          {current[0].date}
        </text>
        <text
          className="trend__axis-label"
          x={WIDTH - PADDING.right}
          y={HEIGHT - 8}
          textAnchor="end"
        >
          {current[current.length - 1].date}
        </text>
      </svg>
    </div>
  );
}
