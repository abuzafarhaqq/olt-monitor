/**
 * MOCK DATA — collection health. Deterministic, synthetic, not live.
 *
 * Deliberately consistent with MOCK_OVERVIEW: OLT-02 is stale, OLT-03 has the
 * partial optical query seen in the event log, OLT-04 is unreachable over
 * Telnet. A job result never contradicts the device's own state.
 */

import type {
  CollectionHealthResponse,
  CollectionJob,
  Collector,
  StaleSource,
} from "../types/domain";
import { MOCK_OVERVIEW } from "./mockData";

const NOW = Date.now();
const MINUTE = 60_000;

const minutesAgo = (minutes: number): string =>
  new Date(NOW - minutes * MINUTE).toISOString();

const jobs: CollectionJob[] = [
  // OLT-01 — healthy, 3 minutes old.
  {
    id: "job-1001",
    oltId: "olt-1",
    type: "onu-status",
    result: "successful",
    lastRunAt: minutesAgo(3),
    durationMs: 820,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1002",
    oltId: "olt-1",
    type: "onu-optical",
    result: "successful",
    lastRunAt: minutesAgo(3),
    durationMs: 1240,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1003",
    oltId: "olt-1",
    type: "syslog-onu-events",
    result: "successful",
    lastRunAt: minutesAgo(3),
    durationMs: 2100,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1004",
    oltId: "olt-1",
    type: "alarm-catalog",
    result: "successful",
    lastRunAt: minutesAgo(4),
    durationMs: 640,
    error: null,
    stateLeftUnchanged: null,
  },

  // OLT-02 — every run succeeded, but nothing is fresh.
  {
    id: "job-1005",
    oltId: "olt-2",
    type: "onu-status",
    result: "successful",
    lastRunAt: minutesAgo(47),
    durationMs: 910,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1006",
    oltId: "olt-2",
    type: "onu-optical",
    result: "successful",
    lastRunAt: minutesAgo(47),
    durationMs: 1380,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1007",
    oltId: "olt-2",
    type: "syslog-onu-events",
    result: "successful",
    lastRunAt: minutesAgo(46),
    durationMs: 3050,
    error: null,
    stateLeftUnchanged: null,
  },

  // OLT-03 — parser could not account for every row.
  {
    id: "job-1008",
    oltId: "olt-3",
    type: "onu-status",
    result: "successful",
    lastRunAt: minutesAgo(6),
    durationMs: 1020,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1009",
    oltId: "olt-3",
    type: "onu-optical",
    result: "partial",
    lastRunAt: minutesAgo(6),
    durationMs: 1610,
    error: {
      kind: "parser",
      message: "show onu opm-diag all returned 10 rows, 9 parsed — column layout differs for ONU model HG323R",
    },
    stateLeftUnchanged: 1,
  },
  {
    id: "job-1010",
    oltId: "olt-3",
    type: "device-info",
    result: "successful",
    lastRunAt: minutesAgo(7),
    durationMs: 480,
    error: null,
    stateLeftUnchanged: null,
  },

  // OLT-04 — unreachable. ONU state stays exactly as it was.
  {
    id: "job-1011",
    oltId: "olt-4",
    type: "onu-status",
    result: "failed",
    lastRunAt: minutesAgo(21),
    durationMs: 15000,
    error: {
      kind: "timeout",
      message: "Telnet session did not reach a prompt within 15 s after 3 attempts",
    },
    stateLeftUnchanged: 6,
  },
  {
    id: "job-1012",
    oltId: "olt-4",
    type: "onu-optical",
    result: "skipped",
    lastRunAt: minutesAgo(21),
    durationMs: null,
    error: null,
    stateLeftUnchanged: null,
  },
  {
    id: "job-1013",
    oltId: "olt-4",
    type: "device-info",
    result: "failed",
    lastRunAt: minutesAgo(21),
    durationMs: 15240,
    error: {
      kind: "network",
      message: "Connection reset by peer during login banner",
    },
    stateLeftUnchanged: null,
  },
];

const collectors: Collector[] = [
  {
    id: "collector-01",
    name: "collector-01",
    state: "running",
    runningJobs: 2,
    queuedJobs: 1,
    heartbeatAt: minutesAgo(0.4),
  },
  {
    id: "collector-02",
    name: "collector-02",
    state: "idle",
    runningJobs: 0,
    queuedJobs: 0,
    heartbeatAt: minutesAgo(0.6),
  },
  {
    id: "collector-03",
    name: "collector-03",
    state: "degraded",
    runningJobs: 0,
    queuedJobs: 4,
    heartbeatAt: minutesAgo(6),
  },
];

const staleSources: StaleSource[] = [
  {
    oltId: "olt-2",
    lastSuccessAt: minutesAgo(47),
    ageMinutes: 47,
    intervalMinutes: 5,
    reason: "last run succeeded but is 9× the collection interval old",
  },
  {
    oltId: "olt-4",
    lastSuccessAt: minutesAgo(21),
    intervalMinutes: 5,
    ageMinutes: 21,
    reason: "OLT unreachable — Telnet session timed out",
  },
];

export const MOCK_COLLECTION_HEALTH: CollectionHealthResponse = {
  generatedAt: new Date(NOW).toISOString(),
  collectors,
  jobs,
  // Identity and reachability only; the ONU list lives with the overview.
  olts: MOCK_OVERVIEW.olts,
  staleSources,
};
