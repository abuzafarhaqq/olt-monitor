/**
 * Report registry. The page renders whatever is declared here, so adding a
 * report is one entry plus one series — not a new screen.
 */
export type ReportId =
  | "onu-availability"
  | "onu-flapping"
  | "optical-rx"
  | "pon-availability"
  | "olt-reachability"
  | "collection-reliability"
  | "incidents";

export interface ReportDefinition {
  id: ReportId;
  label: string;
  description: string;
  unit: string;
  /** Percentages average across days; counts sum. */
  aggregation: "avg" | "sum";
  /** Which direction is good — used only for colouring the delta. */
  better: "higher" | "lower";
  /** Decimal places for display. */
  precision: number;
}

export const REPORTS: ReportDefinition[] = [
  {
    id: "onu-availability",
    label: "ONU availability",
    description: "Share of ONUs reporting online, per day.",
    unit: "%",
    aggregation: "avg",
    better: "higher",
    precision: 2,
  },
  {
    id: "onu-flapping",
    label: "ONU flapping",
    description: "ONUs with three or more deregistrations in a day.",
    unit: "ONUs",
    aggregation: "sum",
    better: "lower",
    precision: 0,
  },
  {
    id: "optical-rx",
    label: "Optical RX",
    description: "Mean RX power across ONUs with a reading.",
    unit: "dBm",
    aggregation: "avg",
    better: "higher",
    precision: 2,
  },
  {
    id: "pon-availability",
    label: "PON availability",
    description: "Share of PON-minutes in which the port was up.",
    unit: "%",
    aggregation: "avg",
    better: "higher",
    precision: 2,
  },
  {
    id: "olt-reachability",
    label: "OLT reachability",
    description: "OLTs reachable at the daily collection window.",
    unit: "OLTs",
    aggregation: "avg",
    better: "higher",
    precision: 1,
  },
  {
    id: "collection-reliability",
    label: "Collection reliability",
    description: "Share of collection jobs that completed successfully.",
    unit: "%",
    aggregation: "avg",
    better: "higher",
    precision: 2,
  },
  {
    id: "incidents",
    label: "Incidents opened",
    description: "Incidents opened per day.",
    unit: "incidents",
    aggregation: "sum",
    better: "lower",
    precision: 0,
  },
];

export function getReport(id: string): ReportDefinition {
  return REPORTS.find((report) => report.id === id) ?? REPORTS[0];
}

/** Detailed measurements are kept 90 days; daily aggregates go back further. */
export const DETAILED_RETENTION_DAYS = 90;
