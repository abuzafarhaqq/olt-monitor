import type { AvailabilityPoint } from "../../types/domain";

const WIDTH = 760;
const HEIGHT = 220;
const PADDING = { top: 10, right: 8, bottom: 26, left: 38 };

const INNER_WIDTH = WIDTH - PADDING.left - PADDING.right;
const INNER_HEIGHT = HEIGHT - PADDING.top - PADDING.bottom;

interface AvailabilityTrendChartProps {
  points: AvailabilityPoint[];
  /** Y-axis label, e.g. "ONUs". */
  unitLabel?: string;
}

/**
 * Hand-drawn stacked bars in SVG — no chart library yet.
 *
 * Why bars: each bucket is an hourly count, so a stacked column states the
 * exact total per hour. Switching to a line/area later is a change of shape
 * inside this one component; nothing outside it knows how the drawing works.
 */
export function AvailabilityTrendChart({ points, unitLabel = "ONUs" }: AvailabilityTrendChartProps) {
  const totals = points.map((point) => point.online + point.offline + point.unknown);
  const maxValue = Math.max(...totals, 1);
  const slot = INNER_WIDTH / Math.max(points.length, 1);
  const barWidth = Math.max(slot * 0.62, 1);

  const heightFor = (value: number): number => (value / maxValue) * INNER_HEIGHT;
  const labelEvery = Math.ceil(points.length / 6);

  return (
    <div className="trend">
      <div className="trend__legend">
        <span className="trend__legend-item">
          <span className="trend__swatch" style={{ background: "var(--color-online)" }} />
          Online
        </span>
        <span className="trend__legend-item">
          <span className="trend__swatch" style={{ background: "var(--color-offline)" }} />
          Offline
        </span>
        <span className="trend__legend-item">
          <span className="trend__swatch" style={{ background: "var(--color-unknown)" }} />
          Unknown
        </span>
      </div>

      <svg
        className="trend__svg"
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        role="img"
        aria-label={`ONU availability over the last ${points.length} hours`}
      >
        {/* Horizontal grid + y-axis ticks */}
        {[0, 0.25, 0.5, 0.75, 1].map((fraction) => {
          const y = PADDING.top + INNER_HEIGHT - fraction * INNER_HEIGHT;
          return (
            <g key={fraction}>
              <line
                className="trend__grid-line"
                x1={PADDING.left}
                x2={WIDTH - PADDING.right}
                y1={y}
                y2={y}
              />
              <text className="trend__axis-label" x={PADDING.left - 6} y={y + 3} textAnchor="end">
                {Math.round(fraction * maxValue)}
              </text>
            </g>
          );
        })}
        <text
          className="trend__axis-label"
          x={0}
          y={PADDING.top - 2}
          textAnchor="start"
        >
          {unitLabel}
        </text>

        {points.map((point, index) => {
          const x = PADDING.left + index * slot + (slot - barWidth) / 2;
          const onlineHeight = heightFor(point.online);
          const offlineHeight = heightFor(point.offline);
          const unknownHeight = heightFor(point.unknown);

          const onlineY = PADDING.top + INNER_HEIGHT - onlineHeight;
          const offlineY = onlineY - offlineHeight;
          const unknownY = offlineY - unknownHeight;

          return (
            <g key={`${point.hourLabel}-${index}`} className="trend__bar">
              <title>
                {`${point.hourLabel} — online ${point.online}, offline ${point.offline}, unknown ${point.unknown}`}
              </title>
              <rect
                className="trend__bar-online"
                x={x}
                y={onlineY}
                width={barWidth}
                height={onlineHeight}
              />
              <rect
                className="trend__bar-offline"
                x={x}
                y={offlineY}
                width={barWidth}
                height={offlineHeight}
              />
              <rect
                className="trend__bar-unknown"
                x={x}
                y={unknownY}
                width={barWidth}
                height={unknownHeight}
              />
            </g>
          );
        })}

        {/* X-axis labels, thinned out so they never collide */}
        {points.map((point, index) =>
          index % labelEvery === 0 ? (
            <text
              key={`label-${point.hourLabel}-${index}`}
              className="trend__axis-label"
              x={PADDING.left + index * slot + slot / 2}
              y={HEIGHT - 8}
              textAnchor="middle"
            >
              {point.hourLabel}
            </text>
          ) : null,
        )}
      </svg>
    </div>
  );
}
