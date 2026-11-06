/**
 * MOCK DATA — 120 days of daily aggregates per report.
 *
 * Deterministic: the same build produces the same curve. Dates are ISO
 * (YYYY-MM-DD) so ranges compare and sort as plain strings.
 */

import type { ReportId } from "../constants/reports";

export interface HistoryPoint {
  /** YYYY-MM-DD */
  date: string;
  value: number;
}

export interface HistorySeries {
  id: ReportId;
  points: HistoryPoint[];
}

const DAYS = 120;
const DAY_MS = 86_400_000;
const END = new Date(Date.UTC(2026, 10, 6)); // 2026-11-06, fixed for determinism

function dateAt(offsetFromEnd: number): string {
  return new Date(END.getTime() - offsetFromEnd * DAY_MS).toISOString().slice(0, 10);
}

function makeRng(seed: number): () => number {
  let state = seed % 2147483647;
  if (state <= 0) state += 2147483646;
  return () => {
    state = (state * 16807) % 2147483647;
    return (state - 1) / 2147483646;
  };
}

const round = (value: number, precision: number): number => {
  const factor = 10 ** precision;
  return Math.round(value * factor) / factor;
};

interface Shape {
  id: ReportId;
  base: number;
  spread: number;
  precision: number;
  /** Downward excursion: days-ago start, length, magnitude. */
  dip?: { from: number; length: number; magnitude: number };
}

const SHAPES: Shape[] = [
  { id: "onu-availability", base: 96.4, spread: 1.2, precision: 2, dip: { from: 46, length: 3, magnitude: 7.5 } },
  { id: "onu-flapping", base: 2.1, spread: 2.4, precision: 0, dip: { from: 46, length: 3, magnitude: 5 } },
  { id: "optical-rx", base: -21.3, spread: 0.35, precision: 2, dip: { from: 46, length: 4, magnitude: -1.1 } },
  { id: "pon-availability", base: 98.9, spread: 0.5, precision: 2, dip: { from: 46, length: 2, magnitude: 6.2 } },
  { id: "olt-reachability", base: 3.9, spread: 0.25, precision: 1, dip: { from: 21, length: 2, magnitude: 0.9 } },
  { id: "collection-reliability", base: 95.8, spread: 1.6, precision: 2, dip: { from: 21, length: 2, magnitude: 9.4 } },
  { id: "incidents", base: 0.9, spread: 1.4, precision: 0, dip: { from: 46, length: 2, magnitude: 2.4 } },
];

export const HISTORY: HistorySeries[] = SHAPES.map((shape, index) => {
  const rng = makeRng(4_211 + index * 97);
  const points: HistoryPoint[] = [];

  for (let offset = DAYS - 1; offset >= 0; offset -= 1) {
    const wave = Math.sin(offset / 5.5 + index) * shape.spread * 0.5;
    const noise = (rng() - 0.5) * shape.spread;
    let value = shape.base + wave + noise;

    if (
      shape.dip !== undefined &&
      offset <= shape.dip.from &&
      offset > shape.dip.from - shape.dip.length
    ) {
      value += shape.dip.magnitude;
    }

    points.push({ date: dateAt(offset), value: round(value, shape.precision) });
  }

  return { id: shape.id, points };
});

/** Oldest and newest dates the platform can answer for. */
export const HISTORY_RANGE = {
  from: HISTORY[0].points[0].date,
  to: HISTORY[0].points[HISTORY[0].points.length - 1].date,
  days: DAYS,
};
