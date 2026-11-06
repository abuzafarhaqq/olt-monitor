import type { Olt, OverviewResponse } from "../../types/domain";

export interface OltRow {
  olt: Olt;
  ponCount: number;
  onuCount: number;
}

export interface OltFilters {
  technology: string;
  reachability: string;
  query: string;
}

export const EMPTY_OLT_FILTERS: OltFilters = { technology: "", reachability: "", query: "" };

export function getOltRows(data: OverviewResponse): OltRow[] {
  return data.olts.map((olt) => ({
    olt,
    ponCount: data.pons.filter((pon) => pon.oltId === olt.id).length,
    onuCount: data.onus.filter((onu) => onu.oltId === olt.id).length,
  }));
}

/** Search matches name, vendor, model and management address — never firmware. */
export function filterOltRows(rows: OltRow[], filters: OltFilters): OltRow[] {
  const query = filters.query.trim().toLowerCase();

  return rows.filter(({ olt }) => {
    if (filters.technology !== "" && olt.technology !== filters.technology) return false;
    if (filters.reachability === "reachable" && !olt.reachable) return false;
    if (filters.reachability === "unreachable" && olt.reachable) return false;
    if (query !== "") {
      const haystack = `${olt.name} ${olt.vendor} ${olt.model} ${olt.managementAddress}`.toLowerCase();
      if (!haystack.includes(query)) return false;
    }
    return true;
  });
}
