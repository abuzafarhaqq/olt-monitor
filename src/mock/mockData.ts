/**
 * MOCK DATA — synthetic, deterministic, and never a live measurement.
 *
 * Every screen that renders this must show the "MOCK DATA" flag from
 * PageHeader. The shapes here match `OverviewResponse`, i.e. exactly what
 * GET /api/v1/dashboard/overview is meant to return in Stage 2.
 *
 * Values are generated from a fixed seed, so the dashboard looks the same on
 * every reload and screenshots stay comparable. Real values would drift.
 */

import { classifyRx } from "../constants/opticalThresholds";
import type {
  AvailabilityPoint,
  DeviceTimestamps,
  Incident,
  NetworkEvent,
  Onu,
  OnuStatus,
  Olt,
  OverviewResponse,
  Pon,
  Technology,
  Vendor,
} from "../types/domain";

const NOW = Date.now();
const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

const minutesAgo = (minutes: number): string =>
  new Date(NOW - minutes * MINUTE).toISOString();

/** Deterministic PRNG (Lehmer / Park–Miller). Same seed → same dashboard. */
function makeRng(seed: number): () => number {
  let state = seed % 2147483647;
  if (state <= 0) state += 2147483646;
  return () => {
    state = (state * 16807) % 2147483647;
    return (state - 1) / 2147483646;
  };
}

const rng = makeRng(20261106);
const round1 = (n: number): number => Math.round(n * 10) / 10;
const pad2 = (n: number): string => String(n).padStart(2, "0");

/* -------------------------------------------------------------------------
 * Device definitions
 * ---------------------------------------------------------------------- */

interface PonSeed {
  name: string;
  online: number;
  offline: number;
  unknown: number;
}

interface OltSeed {
  id: string;
  name: string;
  vendor: Vendor;
  model: string;
  technology: Technology;
  managementAddress: string;
  connection: "ssh" | "telnet";
  reachable: boolean;
  firmware: string;
  hardwareVersion: string;
  uptimeSeconds: number | null;
  lastCollectionMinutesAgo: number;
  pons: PonSeed[];
}

/**
 * Four devices, matching the real estate in the spec: 1 VSOL EPON,
 * 2 VSOL GPON, 1 C-Data GPON. Addresses are from 192.0.2.0/24 (TEST-NET-1) documentation space;
 * MACs use the locally-administered 02: prefix, so none of it can be real.
 */
const OLT_SEEDS: OltSeed[] = [
  {
    id: "olt-1",
    name: "EPON-OLT-01",
    vendor: "VSOL",
    model: "EPON-OLT",
    technology: "EPON",
    managementAddress: "192.0.2.11",
    connection: "ssh",
    reachable: true,
    firmware: "demo-2.03.75",
    hardwareVersion: "V1.1.4",
    uptimeSeconds: 3_542_400,
    lastCollectionMinutesAgo: 3,
    pons: [
      { name: "epon 0/1", online: 12, offline: 2, unknown: 0 },
      { name: "epon 0/2", online: 7, offline: 4, unknown: 1 },
      { name: "epon 0/3", online: 9, offline: 1, unknown: 0 },
    ],
  },
  {
    id: "olt-2",
    name: "GPON-OLT-01",
    vendor: "VSOL",
    model: "GPON-OLT",
    technology: "GPON",
    managementAddress: "192.0.2.12",
    connection: "ssh",
    reachable: true,
    firmware: "demo-2.03.71",
    hardwareVersion: "V1.1.4",
    uptimeSeconds: 1_036_800,
    // Data is stale: collection succeeds but has not run recently.
    lastCollectionMinutesAgo: 47,
    pons: [
      { name: "gpon 0/1", online: 8, offline: 1, unknown: 0 },
      { name: "gpon 0/2", online: 5, offline: 0, unknown: 0 },
    ],
  },
  {
    id: "olt-3",
    name: "GPON-OLT-02",
    vendor: "VSOL",
    model: "GPON-OLT",
    technology: "GPON",
    managementAddress: "192.0.2.13",
    connection: "ssh",
    reachable: true,
    firmware: "demo-2.03.75",
    hardwareVersion: "V1.1.4",
    uptimeSeconds: 432_000,
    lastCollectionMinutesAgo: 6,
    pons: [
      { name: "gpon 0/1", online: 7, offline: 2, unknown: 1 },
      { name: "gpon 0/2", online: 4, offline: 0, unknown: 0 },
    ],
  },
  {
    id: "olt-4",
    name: "GPON-OLT-03",
    vendor: "C-Data",
    model: "FD-OLT",
    technology: "GPON",
    managementAddress: "192.0.2.14",
    connection: "telnet",
    reachable: false,
    firmware: "unknown",
    hardwareVersion: "unknown",
    uptimeSeconds: null,
    lastCollectionMinutesAgo: 21,
    pons: [{ name: "gpon 0/1", online: 0, offline: 0, unknown: 6 }],
  },
];

const ONU_MODEL: Record<string, string> = {
  "VSOL/EPON": "HG323",
  "VSOL/GPON": "HG323R",
  "C-Data/GPON": "FD611GW",
  "C-Data/EPON": "FD511G",
};

function onuVendor(seed: OltSeed): Vendor {
  // ONUs are not always the same brand as the OLT that terminates them.
  return seed.vendor === "VSOL" && rng() > 0.88 ? "C-Data" : seed.vendor;
}

/* -------------------------------------------------------------------------
 * Build ONUs, then derive everything else from them
 * ---------------------------------------------------------------------- */

const onus: Onu[] = [];

const EVENT_LABELS = [
  "onu-register",
  "onu-deregister",
  "onu-dying-gasp",
  "onu-auth-success",
  "link-lost",
];

for (const seed of OLT_SEEDS) {
  seed.pons.forEach((ponSeed, ponIndex) => {
    const ponNumber = ponIndex + 1;
    const ponId = `${seed.id}-pon${ponNumber}`;
    const statuses: OnuStatus[] = [
      ...Array<OnuStatus>(ponSeed.online).fill("online"),
      ...Array<OnuStatus>(ponSeed.offline).fill("offline"),
      ...Array<OnuStatus>(ponSeed.unknown).fill("unknown"),
    ];

    statuses.forEach((status, index) => {
      const onuNumber = index + 1;
      const vendor = onuVendor(seed);
      const hasOptical = status !== "unknown";
      const rx = hasOptical && status === "online" ? round1(-17 - rng() * 8) : null;

      onus.push({
        id: `${ponId}-onu${onuNumber}`,
        oltId: seed.id,
        ponId,
        onuId: onuNumber,
        mac: `02:1A:2B:${pad2(ponNumber).slice(-2)}:${pad2(onuNumber)}:${pad2((onuNumber * 7) % 256)}`,
        vendor,
        model: ONU_MODEL[`${vendor}/${seed.technology}`] ?? "unknown",
        status,
        rxPowerDbm: rx,
        distanceMeters: hasOptical ? 120 + Math.round(rng() * 1800) : null,
        opticalSupported: hasOptical,
        lastEvent:
          status === "online"
            ? "onu-register"
            : EVENT_LABELS[Math.floor(rng() * EVENT_LABELS.length)],
        deregistrations24h:
          status === "offline" ? 1 + Math.floor(rng() * 3) : 0,
        collectionStatus: status === "unknown" ? "failed" : "successful",
      });
    });
  });
}

/** Find an ONU by OLT id, 1-based PON number and 1-based ONU number. */
function ref(oltId: string, ponNumber: number, onuNumber: number): Onu {
  const id = `${oltId}-pon${ponNumber}-onu${onuNumber}`;
  const onu = onus.find((candidate) => candidate.id === id);
  if (!onu) throw new Error(`mock: no ONU ${id}`);
  return onu;
}

/* Edge cases the UI must survive ---------------------------------------- */

// Adapter has no optical command for this ONU model. Unsupported is not zero.
Object.assign(ref("olt-1", 2, 3), {
  rxPowerDbm: null,
  opticalSupported: false,
  collectionStatus: "unsupported" as const,
});
Object.assign(ref("olt-1", 2, 5), {
  rxPowerDbm: null,
  opticalSupported: false,
  collectionStatus: "unsupported" as const,
});

// Optical command ran but returned nothing for these rows. Missing ≠ offline.
Object.assign(ref("olt-1", 1, 4), { rxPowerDbm: null, collectionStatus: "partial" as const });
Object.assign(ref("olt-3", 1, 2), { rxPowerDbm: null, collectionStatus: "partial" as const });

// Degraded optics while the ONU still reports online.
Object.assign(ref("olt-1", 1, 2), { vendor: "VSOL" as const, model: "HG323", rxPowerDbm: -31.4 });
Object.assign(ref("olt-1", 2, 1), { vendor: "VSOL" as const, model: "HG323", rxPowerDbm: -27.9 });
Object.assign(ref("olt-3", 1, 3), { vendor: "VSOL" as const, model: "HG323R", rxPowerDbm: -25.6 });

// Dying gasp: power-loss related, not proof of root cause.
Object.assign(ref("olt-1", 2, 6), { lastEvent: "onu-dying-gasp" });
Object.assign(ref("olt-2", 1, 3), { lastEvent: "onu-dying-gasp" });

// OLT-2 data is old: the last run succeeded, but nothing is fresh.
for (const onu of onus.filter((candidate) => candidate.oltId === "olt-2")) {
  onu.collectionStatus = "stale";
}

/* -------------------------------------------------------------------------
 * OLTs and PONs, derived from the ONU list
 * ---------------------------------------------------------------------- */

function collectionStatusForOlt(seed: OltSeed): Olt["collectionStatus"] {
  if (!seed.reachable) return "failed";
  if (seed.id === "olt-3") return "partial";
  if (seed.lastCollectionMinutesAgo > 30) return "stale";
  return "successful";
}

function timestampsFor(seed: OltSeed): DeviceTimestamps {
  return {
    ingestedAt: minutesAgo(seed.lastCollectionMinutesAgo),
    // The first VSOL EPON reports May 2000 — its clock cannot be trusted.
    deviceTime: seed.id === "olt-1" ? "2000-05-20T04:12:33.000Z" : minutesAgo(seed.lastCollectionMinutesAgo - 1),
    quality: seed.id === "olt-1" ? "untrusted" : "trusted",
  };
}

const olts: Olt[] = OLT_SEEDS.map((seed) => ({
  id: seed.id,
  name: seed.name,
  vendor: seed.vendor,
  model: seed.model,
  technology: seed.technology,
  managementAddress: seed.managementAddress,
  connection: seed.connection,
  reachable: seed.reachable,
  firmware: seed.firmware,
  hardwareVersion: seed.hardwareVersion,
  uptimeSeconds: seed.uptimeSeconds,
  ponCount: seed.pons.length,
  lastCollection: timestampsFor(seed),
  collectionStatus: collectionStatusForOlt(seed),
}));

const pons: Pon[] = OLT_SEEDS.flatMap((seed) =>
  seed.pons.map((ponSeed, ponIndex) => {
    const ponNumber = ponIndex + 1;
    const ponId = `${seed.id}-pon${ponNumber}`;
    const ponOnus = onus.filter((onu) => onu.ponId === ponId);
    const oltStatus = collectionStatusForOlt(seed);
    const status = oltStatus === "successful" && ponNumber === 2 ? "partial" : oltStatus;

    return {
      id: ponId,
      oltId: seed.id,
      name: ponSeed.name,
      counts: {
        total: ponOnus.length,
        online: ponOnus.filter((onu) => onu.status === "online").length,
        offline: ponOnus.filter((onu) => onu.status === "offline").length,
        unknown: ponOnus.filter((onu) => onu.status === "unknown").length,
      },
      // Thresholds come from the shared model-specific table, never a literal.
      lowOpticalCount: ponOnus.filter((onu) => {
        const severity = classifyRx(onu.rxPowerDbm, onu.vendor, seed.technology, onu.opticalSupported);
        return severity === "warning" || severity === "critical";
      }).length,
      dyingGaspCount24h: ponOnus.filter((onu) => onu.lastEvent === "onu-dying-gasp").length,
      flappingCount: ponOnus.filter((onu) => onu.status === "offline").length > 3 ? 2 : 0,
      lastCollection: timestampsFor(seed),
      collectionStatus: status,
    };
  }),
);

/* -------------------------------------------------------------------------
 * Events — duplicates are kept on purpose (§7: do not blindly deduplicate)
 * ---------------------------------------------------------------------- */

const events: NetworkEvent[] = [
  {
    id: "evt-1001",
    type: "onu-deregister",
    severity: "critical",
    oltId: "olt-1",
    ponId: "olt-1-pon2",
    onuId: 6,
    message: "ONU deregistered — dying gasp recorded",
    timestamps: {
      deviceTime: "2000-05-20T04:12:33.000Z",
      ingestedAt: minutesAgo(4),
      quality: "untrusted",
    },
    evidenceRef: "evidence/olt-1/syslog-deregister/2026-11-06T01-12Z.txt",
  },
  {
    // Same event, second raw record, different evidence reference.
    id: "evt-1002",
    type: "onu-deregister",
    severity: "critical",
    oltId: "olt-1",
    ponId: "olt-1-pon2",
    onuId: 6,
    message: "ONU deregistered — dying gasp recorded",
    timestamps: {
      deviceTime: "2000-05-20T04:12:33.000Z",
      ingestedAt: minutesAgo(4),
      quality: "untrusted",
    },
    evidenceRef: "evidence/olt-1/syslog-dying-gasp/2026-11-06T01-12Z.txt",
  },
  {
    id: "evt-1003",
    type: "onu-register",
    severity: "info",
    oltId: "olt-1",
    ponId: "olt-1-pon1",
    onuId: 2,
    message: "ONU registered — auth success",
    timestamps: { deviceTime: minutesAgo(9), ingestedAt: minutesAgo(8), quality: "trusted" },
    evidenceRef: "evidence/olt-1/syslog-register/2026-11-06T01-07Z.txt",
  },
  {
    id: "evt-1004",
    type: "olt-unreachable",
    severity: "critical",
    oltId: "olt-4",
    ponId: null,
    onuId: null,
    message: "OLT unreachable — SSH timeout after 3 attempts",
    timestamps: { deviceTime: null, ingestedAt: minutesAgo(21), quality: "missing" },
    evidenceRef: "evidence/olt-4/collector/2026-11-06T00-55Z.log",
  },
  {
    id: "evt-1005",
    type: "collector-failure",
    severity: "warning",
    oltId: "olt-3",
    ponId: "olt-3-pon1",
    onuId: null,
    message: "Optical query returned 9 of 10 rows — partial collection",
    timestamps: { deviceTime: minutesAgo(7), ingestedAt: minutesAgo(6), quality: "trusted" },
    evidenceRef: "evidence/olt-3/opm-diag/2026-11-06T01-10Z.txt",
  },
  {
    id: "evt-1006",
    type: "onu-auth-failure",
    severity: "warning",
    oltId: "olt-1",
    ponId: "olt-1-pon3",
    onuId: 4,
    message: "ONU authentication failed — serial mismatch",
    timestamps: { deviceTime: minutesAgo(33), ingestedAt: minutesAgo(32), quality: "trusted" },
    evidenceRef: "evidence/olt-1/syslog-auth/2026-11-06T00-43Z.txt",
  },
  {
    id: "evt-1007",
    type: "onu-dying-gasp",
    severity: "warning",
    oltId: "olt-2",
    ponId: "olt-2-pon1",
    onuId: 3,
    message: "Dying gasp — possible local power loss",
    timestamps: { deviceTime: minutesAgo(88), ingestedAt: minutesAgo(87), quality: "trusted" },
    evidenceRef: "evidence/olt-2/syslog-dying-gasp/2026-11-05T23-48Z.txt",
  },
  {
    id: "evt-1008",
    type: "link-lost",
    severity: "warning",
    oltId: "olt-1",
    ponId: "olt-1-pon2",
    onuId: 9,
    message: "Link lost on PON 0/2 — ONU 9",
    timestamps: { deviceTime: minutesAgo(140), ingestedAt: minutesAgo(139), quality: "trusted" },
    evidenceRef: "evidence/olt-1/syslog-link/2026-11-05T22-56Z.txt",
  },
];

/* -------------------------------------------------------------------------
 * Incidents — a suspected PON outage is a hypothesis, not a root cause
 * ---------------------------------------------------------------------- */

const incidents: Incident[] = [
  {
    id: "INC-2411-007",
    title: "PON 0/2 flapping with dying-gasp cluster",
    state: "investigating",
    severity: "critical",
    oltId: "olt-1",
    ponId: "olt-1-pon2",
    openedAt: minutesAgo(186),
    hypothesis:
      "Four ONUs deregistered within 20 minutes. Shared PON outage is suspected but not confirmed — the OLT itself is reachable and PON 0/1 is unaffected.",
  },
  {
    id: "INC-2411-008",
    title: "C-Data GPON-OLT-03 unreachable",
    state: "new",
    severity: "critical",
    oltId: "olt-4",
    ponId: null,
    openedAt: minutesAgo(21),
    hypothesis: "Management connectivity lost. ONU state on this OLT is last known, not current.",
  },
  {
    id: "INC-2411-004",
    title: "ONU 0/3:4 authentication failures",
    state: "acknowledged",
    severity: "warning",
    oltId: "olt-1",
    ponId: "olt-1-pon3",
    openedAt: minutesAgo(320),
    hypothesis: "Serial mismatch after ONU replacement. Awaiting field confirmation.",
  },
  {
    id: "INC-2410-019",
    title: "GPON-OLT-02 partial optical collection",
    state: "resolved",
    severity: "warning",
    oltId: "olt-3",
    ponId: "olt-3-pon1",
    openedAt: minutesAgo(60 * 26),
  },
];

/* -------------------------------------------------------------------------
 * 24-hour availability trend, hourly buckets
 * ---------------------------------------------------------------------- */

const availabilityTrend: AvailabilityPoint[] = (() => {
  const totalOnline = onus.filter((onu) => onu.status === "online").length;
  const points: AvailabilityPoint[] = [];

  for (let hourOffset = 23; hourOffset >= 0; hourOffset -= 1) {
    const bucket = new Date(NOW - hourOffset * HOUR);
    const label = `${pad2(bucket.getHours())}:00`;

    // A dip 8–11 hours ago: the suspected PON 0/2 outage window.
    const inOutageWindow = hourOffset <= 11 && hourOffset >= 8;
    const drift = Math.round((rng() - 0.5) * 3);
    const offline = inOutageWindow ? 14 + Math.round(rng() * 2) : 6 + Math.round(rng() * 3);
    const unknown = hourOffset === 0 ? 7 : 2 + Math.round(rng() * 3);

    points.push({
      hourLabel: label,
      online: totalOnline + drift - (inOutageWindow ? 6 : 0),
      offline,
      unknown,
    });
  }

  return points;
})();

export const MOCK_OVERVIEW: OverviewResponse = {
  generatedAt: new Date(NOW).toISOString(),
  olts,
  pons,
  onus,
  events,
  incidents,
  availabilityTrend,
};
