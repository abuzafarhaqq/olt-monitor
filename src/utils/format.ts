/**
 * Formatting helpers. Kept in one place so a timestamp is never rendered two
 * different ways in two different components.
 */

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** "01:12" — clock only, for dense table cells. */
export function formatClock(iso: string): string {
  return new Date(iso).toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** "06 Nov 2026 01:12" — full local time, seconds omitted. */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** "42s ago", "7m ago", "3h ago", "2d ago". */
export function formatRelative(iso: string, now: number = Date.now()): string {
  const diff = now - new Date(iso).getTime();
  if (diff < MINUTE) return "just now";
  if (diff < HOUR) return `${Math.floor(diff / MINUTE)}m ago`;
  if (diff < DAY) return `${Math.floor(diff / HOUR)}h ago`;
  return `${Math.floor(diff / DAY)}d ago`;
}

/**
 * A device clock that reports year 2000 is not a date, it is a symptom.
 * Formatting it as-is keeps the operator honest about the evidence.
 */
export function formatDeviceTime(iso: string | null): string {
  if (iso === null) return "not reported";
  const year = new Date(iso).getUTCFullYear();
  const text = formatDateTime(iso);
  return year < 2010 ? `${text} (clock suspect)` : text;
}

/** "−27.4 dBm" or "—" when there is no reading. Never "0 dBm". */
export function formatDbm(value: number | null): string {
  if (value === null) return "—";
  return `${value.toFixed(1)} dBm`;
}

export function formatDistance(meters: number | null): string {
  if (meters === null) return "—";
  if (meters < 1000) return `${meters} m`;
  return `${(meters / 1000).toFixed(2)} km`;
}

export function formatCount(value: number): string {
  return value.toLocaleString("en-GB");
}

/** "820 ms", "1.4 s", "12 s". null = the run produced no duration. */
export function formatDuration(ms: number | null): string {
  if (ms === null) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(1)} s`;
}
