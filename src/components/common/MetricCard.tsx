import type { CSSProperties, ReactNode } from "react";

export type MetricTone =
  | "neutral"
  | "online"
  | "offline"
  | "warning"
  | "info"
  | "unknown"
  | "investigating";

interface MetricCardProps {
  label: string;
  /** Text, a number, or a badge — the card only controls the typography. */
  value: ReactNode;
  hint?: string;
  tone?: MetricTone;
}

/** Tone → CSS variable. One mapping, so a status colour is never guessed. */
const TONE_COLOR: Record<MetricTone, string> = {
  neutral: "var(--color-border-strong)",
  online: "var(--color-online)",
  offline: "var(--color-offline)",
  warning: "var(--color-warning)",
  info: "var(--color-info)",
  unknown: "var(--color-unknown)",
  investigating: "var(--color-investigating)",
};

export function MetricCard({ label, value, hint, tone = "neutral" }: MetricCardProps) {
  // Custom properties are not part of CSSProperties, hence the cast.
  const style = { "--tone-color": TONE_COLOR[tone] } as CSSProperties;

  return (
    <article className="metric-card" style={style}>
      <span className="metric-card__label">{label}</span>
      <span className="metric-card__value">{value}</span>
      {hint ? <span className="metric-card__hint">{hint}</span> : null}
    </article>
  );
}
