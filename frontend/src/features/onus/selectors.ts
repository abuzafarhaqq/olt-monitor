import { classifyRx, type OpticalSeverity } from "../../constants/opticalThresholds";
import type { Onu, Olt, OverviewResponse } from "../../types/domain";

export interface OnuListFilters {
  query: string;
  oltId: string;
  ponId: string;
  state: string;
  optical: string;
}

export const EMPTY_ONU_LIST_FILTERS: OnuListFilters = {
  query: "",
  oltId: "",
  ponId: "",
  state: "",
  optical: "",
};

/** One lookup, reused by every filter below — avoids O(n²) scans. */
export function getOltMap(data: OverviewResponse): Map<string, Olt> {
  return new Map(data.olts.map((olt) => [olt.id, olt]));
}

export function severityOf(onu: Onu, olts: Map<string, Olt>): OpticalSeverity {
  const olt = olts.get(onu.oltId);
  return classifyRx(onu.rxPowerDbm, onu.vendor, olt?.technology ?? "GPON", onu.opticalSupported);
}

/**
 * Estate-wide ONU filter. `ponId` is only meaningful together with `oltId`,
 * so selecting an OLT resets the PON in the page that owns the state.
 */
export function filterOnus(data: OverviewResponse, filters: OnuListFilters): Onu[] {
  const olts = getOltMap(data);
  const query = filters.query.trim().toLowerCase();

  return data.onus.filter((onu) => {
    if (filters.oltId !== "" && onu.oltId !== filters.oltId) return false;
    if (filters.ponId !== "" && onu.ponId !== filters.ponId) return false;
    if (filters.state !== "" && onu.status !== filters.state) return false;
    if (filters.optical !== "" && severityOf(onu, olts) !== filters.optical) return false;
    if (query !== "" && !`${onu.onuId} ${onu.mac}`.toLowerCase().includes(query)) return false;
    return true;
  });
}
