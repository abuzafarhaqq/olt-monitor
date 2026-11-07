import { classifyRx, type OpticalSeverity } from "../../constants/opticalThresholds";
import type { Onu, Olt, OverviewResponse, Pon } from "../../types/domain";

export interface PonBranch {
  olt: Olt;
  pons: Pon[];
}

/** OLT → PON tree. PON names come from the adapter, never generated here. */
export function getTree(data: OverviewResponse): PonBranch[] {
  return data.olts.map((olt) => ({
    olt,
    pons: data.pons.filter((pon) => pon.oltId === olt.id),
  }));
}

export interface OnuFilters {
  state: string;
  optical: string;
  event: string;
  /** "3" means three or more deregistrations in 24h. */
  disconnects: string;
  vendorModel: string;
  query: string;
}

export const EMPTY_ONU_FILTERS: OnuFilters = {
  state: "",
  optical: "",
  event: "",
  disconnects: "",
  vendorModel: "",
  query: "",
};

export function getVendorModelOptions(onus: Onu[]): string[] {
  return [...new Set(onus.map((onu) => `${onu.vendor} ${onu.model}`))].sort();
}

/**
 * Every filter is independent and combinable. "Missing optical reading" is a
 * filter of its own, so an operator cannot accidentally read absence as a fault.
 */
export function filterPonOnus(onus: Onu[], olt: Olt, filters: OnuFilters): Onu[] {
  const query = filters.query.trim().toLowerCase();

  return onus.filter((onu) => {
    if (filters.state !== "" && onu.status !== filters.state) return false;

    if (filters.optical !== "") {
      const severity: OpticalSeverity = classifyRx(
        onu.rxPowerDbm,
        onu.vendor,
        olt.technology,
        onu.opticalSupported,
      );
      if (severity !== filters.optical) return false;
    }

    if (filters.event !== "" && onu.lastEvent !== filters.event) return false;
    if (filters.disconnects === "3" && onu.deregistrations24h < 3) return false;
    if (filters.vendorModel !== "" && `${onu.vendor} ${onu.model}` !== filters.vendorModel) {
      return false;
    }
    if (query !== "" && !`${onu.onuId} ${onu.mac}`.toLowerCase().includes(query)) return false;

    return true;
  });
}

export interface PonMetrics {
  total: number;
  online: number;
  offline: number;
  unknown: number;
  lowOptical: number;
  dyingGasp: number;
  flapping: number;
}

export function getPonMetrics(onus: Onu[], olt: Olt): PonMetrics {
  const severityOf = (onu: Onu) =>
    classifyRx(onu.rxPowerDbm, onu.vendor, olt.technology, onu.opticalSupported);

  return {
    total: onus.length,
    online: onus.filter((onu) => onu.status === "online").length,
    offline: onus.filter((onu) => onu.status === "offline").length,
    unknown: onus.filter((onu) => onu.status === "unknown").length,
    lowOptical: onus.filter((onu) => {
      const severity = severityOf(onu);
      return severity === "warning" || severity === "critical";
    }).length,
    dyingGasp: onus.filter((onu) => onu.lastEvent === "onu-dying-gasp").length,
    flapping: onus.filter((onu) => onu.deregistrations24h >= 3).length,
  };
}
