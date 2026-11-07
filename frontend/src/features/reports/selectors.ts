import type { ReportDefinition } from "../../constants/reports";
import type { HistoryPoint, HistorySeries } from "../../mock/history";
import type { ExportRow } from "../../utils/export";

const DAY_MS = 86_400_000;

/** ISO dates compare as strings — that is the whole point of the format. */
export function pointsInRange(points: HistoryPoint[], from: string, to: string): HistoryPoint[] {
  return points.filter((point) => point.date >= from && point.date <= to);
}

export function shiftDays(date: string, days: number): string {
  return new Date(Date.parse(`${date}T00:00:00Z`) + days * DAY_MS).toISOString().slice(0, 10);
}

export function daysBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / DAY_MS) + 1;
}

/**
 * Previous period of the same length, ending the day before `from`.
 * Equal length matters: comparing 7 days with 30 days produces a number
 * that looks precise and means nothing.
 */
export function previousRange(from: string, to: string): { from: string; to: string } {
  const length = daysBetween(from, to);
  return { from: shiftDays(from, -length), to: shiftDays(from, -1) };
}

export function summarize(points: HistoryPoint[], report: ReportDefinition): number | null {
  if (points.length === 0) return null;
  const total = points.reduce((sum, point) => sum + point.value, 0);
  return report.aggregation === "sum" ? total : total / points.length;
}

export interface Comparison {
  current: number | null;
  previous: number | null;
  delta: number | null;
  /** Percent change, null when the previous value is 0 (undefined ratio). */
  deltaPct: number | null;
  /** Is the change in the good direction? null when flat or unknown. */
  improved: boolean | null;
}

export function compare(
  current: HistoryPoint[],
  previous: HistoryPoint[],
  report: ReportDefinition,
): Comparison {
  const currentValue = summarize(current, report);
  const previousValue = summarize(previous, report);

  if (currentValue === null || previousValue === null) {
    return { current: currentValue, previous: previousValue, delta: null, deltaPct: null, improved: null };
  }

  const delta = currentValue - previousValue;
  const deltaPct = previousValue === 0 ? null : (delta / Math.abs(previousValue)) * 100;
  const improved =
    delta === 0 ? null : report.better === "higher" ? delta > 0 : delta < 0;

  return { current: currentValue, previous: previousValue, delta, deltaPct, improved };
}

export function getSeries(series: HistorySeries[], id: string): HistorySeries | undefined {
  return series.find((candidate) => candidate.id === id);
}

/** Rows for both the table and the CSV — one shape, two consumers. */
export function buildRows(
  current: HistoryPoint[],
  previous: HistoryPoint[] | null,
): ExportRow[] {
  return current.map((point, index) => ({
    date: point.date,
    value: point.value,
    previous: previous === null ? null : (previous[index]?.value ?? null),
  }));
}

export function formatValue(value: number | null, report: ReportDefinition): string {
  if (value === null) return "—";
  return `${value.toFixed(report.precision)}${report.unit === "%" ? "%" : ""}`;
}
