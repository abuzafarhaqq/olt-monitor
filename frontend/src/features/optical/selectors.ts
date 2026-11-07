import type { DistributionBucket } from "../../components/charts/DistributionChart";
import type { OpticalSeverity } from "../../constants/opticalThresholds";
import { classifyRx } from "../../constants/opticalThresholds";
import type { Onu, OverviewResponse, Pon } from "../../types/domain";
import { getOltMap, severityOf } from "../onus/selectors";

export interface OpticalFilters {
  oltId: string;
  ponId: string;
  vendor: string;
  severity: string;
  minDbm: string;
  maxDbm: string;
}

export const EMPTY_OPTICAL_FILTERS: OpticalFilters = {
  oltId: "",
  ponId: "",
  vendor: "",
  severity: "",
  minDbm: "",
  maxDbm: "",
};

export interface OpticalKpis {
  withReading: number;
  low: number;
  critical: number;
  missing: number;
  unsupported: number;
  unconfigured: number;
}

export function getOpticalOnus(data: OverviewResponse, filters: OpticalFilters): Onu[] {
  const olts = getOltMap(data);
  const min = filters.minDbm === "" ? null : Number(filters.minDbm);
  const max = filters.maxDbm === "" ? null : Number(filters.maxDbm);

  return data.onus.filter((onu) => {
    if (filters.oltId !== "" && onu.oltId !== filters.oltId) return false;
    if (filters.ponId !== "" && onu.ponId !== filters.ponId) return false;
    if (filters.vendor !== "" && onu.vendor !== filters.vendor) return false;

    if (filters.severity !== "" && severityOf(onu, olts) !== filters.severity) return false;

    // RX range only applies to rows that actually have a reading.
    if (min !== null || max !== null) {
      if (onu.rxPowerDbm === null) return false;
      if (min !== null && onu.rxPowerDbm < min) return false;
      if (max !== null && onu.rxPowerDbm > max) return false;
    }
    return true;
  });
}

export function getOpticalKpis(data: OverviewResponse): OpticalKpis {
  const olts = getOltMap(data);
  const tally: Record<OpticalSeverity, number> = {
    ok: 0,
    warning: 0,
    critical: 0,
    missing: 0,
    unsupported: 0,
    unconfigured: 0,
  };
  for (const onu of data.onus) {
    tally[classifyRx(onu.rxPowerDbm, onu.vendor, olts.get(onu.oltId)?.technology ?? "GPON", onu.opticalSupported)] += 1;
  }

  return {
    withReading: data.onus.filter((onu) => onu.rxPowerDbm !== null).length,
    low: tally.warning,
    critical: tally.critical,
    missing: tally.missing,
    unsupported: tally.unsupported,
    unconfigured: tally.unconfigured,
  };
}

/**
 * Buckets of 2 dBm. The bar colour is the *dominant* severity inside that
 * bucket, computed per ONU with its own model threshold — never a global rule.
 */
export function getRxDistribution(onus: Onu[], data: OverviewResponse): DistributionBucket[] {
  const olts = getOltMap(data);
  const edges = [-35, -33, -31, -29, -27, -25, -23, -21, -19, -17, -15];

  return edges.slice(0, -1).map((from, index) => {
    const to = edges[index + 1];
    const inBucket = onus.filter(
      (onu) => onu.rxPowerDbm !== null && onu.rxPowerDbm >= from && onu.rxPowerDbm < to,
    );

    const counts = { critical: 0, warning: 0, ok: 0 };
    for (const onu of inBucket) {
      const severity = severityOf(onu, olts);
      if (severity === "critical") counts.critical += 1;
      else if (severity === "warning") counts.warning += 1;
      else if (severity === "ok") counts.ok += 1;
    }

    const tone =
      counts.critical >= counts.warning && counts.critical >= counts.ok && counts.critical > 0
        ? "offline"
        : counts.warning > 0 && counts.warning >= counts.ok
          ? "warning"
          : "neutral";

    return { label: `${from}`, count: inBucket.length, tone };
  });
}

export interface PonOpticalHealth {
  pon: Pon;
  oltName: string;
  total: number;
  withReading: number;
  low: number;
  critical: number;
  missing: number;
}

export function getHealthByPon(data: OverviewResponse, onus: Onu[]): PonOpticalHealth[] {
  const olts = getOltMap(data);

  return data.pons
    .map((pon) => {
      const ponOnus = onus.filter((onu) => onu.ponId === pon.id);
      return {
        pon,
        oltName: olts.get(pon.oltId)?.name ?? pon.oltId,
        total: ponOnus.length,
        withReading: ponOnus.filter((onu) => onu.rxPowerDbm !== null).length,
        low: ponOnus.filter((onu) => severityOf(onu, olts) === "warning").length,
        critical: ponOnus.filter((onu) => severityOf(onu, olts) === "critical").length,
        missing: ponOnus.filter((onu) => severityOf(onu, olts) === "missing").length,
      };
    })
    .filter((row) => row.total > 0);
}
