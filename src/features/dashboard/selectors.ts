/**
 * Selectors: pure functions that turn raw data into what a screen needs.
 *
 * Keeping them out of components means the dashboard can be tested without a
 * browser, and that "how many are unknown" is answered in exactly one place.
 */

import { classifyRx } from "../../constants/opticalThresholds";
import type {
  Incident,
  NetworkEvent,
  Onu,
  OnuCounts,
  OverviewResponse,
  Pon,
} from "../../types/domain";

export interface OverviewKpis {
  totalOlts: number;
  reachableOlts: number;
  totalOnus: number;
  online: number;
  offline: number;
  unknown: number;
  activeIncidents: number;
  /** OLTs whose *last collection* did not succeed — never mixed with ONU state. */
  unhealthyCollections: number;
}

export function getOnuCounts(onus: Onu[]): OnuCounts {
  return {
    total: onus.length,
    online: onus.filter((onu) => onu.status === "online").length,
    offline: onus.filter((onu) => onu.status === "offline").length,
    unknown: onus.filter((onu) => onu.status === "unknown").length,
  };
}

export function isActiveIncident(incident: Incident): boolean {
  return (
    incident.state === "new" ||
    incident.state === "acknowledged" ||
    incident.state === "investigating"
  );
}

export function getKpis(data: OverviewResponse): OverviewKpis {
  const counts = getOnuCounts(data.onus);

  return {
    totalOlts: data.olts.length,
    reachableOlts: data.olts.filter((olt) => olt.reachable).length,
    totalOnus: counts.total,
    online: counts.online,
    offline: counts.offline,
    unknown: counts.unknown,
    activeIncidents: data.incidents.filter(isActiveIncident).length,
    unhealthyCollections: data.olts.filter(
      (olt) => olt.collectionStatus === "failed" || olt.collectionStatus === "partial",
    ).length,
  };
}

/** Newest first; duplicates are preserved, never collapsed. */
export function getRecentEvents(events: NetworkEvent[], limit: number): NetworkEvent[] {
  return [...events]
    .sort(
      (a, b) =>
        new Date(b.timestamps.ingestedAt).getTime() -
        new Date(a.timestamps.ingestedAt).getTime(),
    )
    .slice(0, limit);
}

export interface ProblemPon {
  pon: Pon;
  oltName: string;
  offline: number;
  lowOptical: number;
  dyingGasp: number;
  flapping: number;
  /** Ranking weight — only meaningful for a PON with reasonably fresh data. */
  score: number;
}

/**
 * Rank PONs by how much they hurt. Only PONs with fresh data are ranked:
 * a stale PON would otherwise top the list just because its counters are old.
 */
export function getTopProblemPons(data: OverviewResponse, limit = 5): ProblemPon[] {
  const oltById = new Map(data.olts.map((olt) => [olt.id, olt]));

  const problems = data.pons.map((pon): ProblemPon => {
    const olt = oltById.get(pon.oltId);
    const ponOnus = data.onus.filter((onu) => onu.ponId === pon.id);
    const lowOptical = ponOnus.filter((onu) => {
      const severity = classifyRx(
        onu.rxPowerDbm,
        onu.vendor,
        olt?.technology ?? "GPON",
        onu.opticalSupported,
      );
      return severity === "warning" || severity === "critical";
    }).length;

    const offline = pon.counts.offline;
    const flapping = pon.flappingCount;
    const dyingGasp = pon.dyingGaspCount24h;

    return {
      pon,
      oltName: olt?.name ?? pon.oltId,
      offline,
      lowOptical,
      dyingGasp,
      flapping,
      score: offline * 3 + flapping * 2 + lowOptical + dyingGasp,
    };
  });

  return problems
    .filter((problem) => problem.score > 0)
    .filter(
      (problem) =>
        problem.pon.collectionStatus !== "stale" &&
        problem.pon.collectionStatus !== "failed",
    )
    .sort((a, b) => b.score - a.score)
    .slice(0, limit);
}
