/**
 * In-memory stand-in for the Go API.
 *
 * Same function signatures as `api`, so switching to the real backend is one
 * constant in client.ts. It also lets failure paths be exercised on purpose:
 * set `failureRate` to see how the console reports an error.
 */

import type { DiagnosticCommand, DiagnosticResult } from "../types/domain";
import { MOCK_OVERVIEW } from "../mock/mockData";

/**
 * Allow-list. The browser never sends arbitrary CLI — only these ids, and the
 * backend maps the id to a command an adapter has approved.
 */
export const DIAGNOSTIC_COMMANDS: DiagnosticCommand[] = [
  { id: "version", technology: "any", label: "Device version", command: "show version" },
  { id: "time", technology: "any", label: "Device clock and uptime", command: "show time" },
  { id: "onu-status", technology: "any", label: "ONU status, all ports", command: "show onu status all" },
  { id: "opm-diag", technology: "any", label: "Optical diagnostics, all ports", command: "show onu opm-diag all" },
  { id: "syslog-gasp", technology: "any", label: "Dying-gasp log", command: "show syslog type onu-dying-gasp" },
  { id: "alarm-catalog", technology: "EPON", label: "Alarm and event catalog", command: "show alarm-event information" },
  { id: "perf-15min", technology: "EPON", label: "15-minute performance stats", command: "show perf-stats 15min" },
];

export const mockConfig = {
  latencyMs: 220,
  /** 0 = always succeed. Set to 0.3 to rehearse the error path. */
  failureRate: 0,
};

const delay = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

function outputFor(commandId: string): string {
  switch (commandId) {
    case "version":
      return [
        "        VSOL EPON OLT",
        "Hardware Version : V1.1.4",
        "Software Version : demo-2.03.75",
        "Build Date       : 1970-01-01",
        "",
        "# MOCK OUTPUT - not read from a device",
      ].join("\n");
    case "time":
      return [
        "Current time : 2000-05-20 04:12:33",
        "Uptime       : 41 days, 0 hours, 12 minutes",
        "",
        "# device clock is 26 years behind: treat every device timestamp as suspect",
      ].join("\n");
    case "onu-status":
      return [
        "PON  ONU  MAC                Status   Distance(m)",
        "0/1  1    02:1A:2B:01:01:07  online   420",
        "0/1  2    02:1A:2B:01:02:14  online   1180",
        "0/2  6    02:1A:2B:02:06:42  offline  —",
        "",
        "# 213 rows expected, 3 shown - MOCK",
      ].join("\n");
    case "opm-diag":
      return [
        "PON  ONU  RX(dBm)  TX(dBm)",
        "0/1  1    -18.4    2.41",
        "0/1  2    -31.4    2.38",
        "0/2  9    --        --",
        "",
        "# 133 readings returned for 213 ONUs - not every ONU has optics",
      ].join("\n");
    case "syslog-gasp":
      return [
        "2000-05-20 04:12:33  onu 0/2:6   dying-gasp",
        "2000-05-20 04:12:33  onu 0/2:6   dying-gasp",
        "",
        "# duplicate records kept: two captures, same event",
      ].join("\n");
    case "alarm-catalog":
      return ["Alarm ID  Name                 Severity", "1         PON LOS              critical", "2         ONU dying gasp       warning", "", "# catalog only - does not prove an alarm fired"].join("\n");
    case "perf-15min":
      return ["Interface  InOctets    OutOctets   Errors", "epon 0/1   884201133   1029384421  0", "", "# 15-minute buckets, retained 24h on device"].join("\n");
    default:
      return "# no output";
  }
}

export async function runDiagnostic(oltId: string, commandId: string): Promise<DiagnosticResult> {
  const command = DIAGNOSTIC_COMMANDS.find((candidate) => candidate.id === commandId);
  const olt = MOCK_OVERVIEW.olts.find((candidate) => candidate.id === oltId);

  if (command === undefined || olt === undefined) {
    throw new Error("Unknown OLT or command");
  }

  const startedAt = new Date().toISOString();
  await delay(mockConfig.latencyMs);

  if (Math.random() < mockConfig.failureRate) {
    return {
      command: command.command,
      startedAt,
      durationMs: mockConfig.latencyMs,
      result: "failed",
      text: "Timed out waiting for the device prompt after 15 s.\n# injected failure (mockConfig.failureRate)",
      evidenceRef: `evidence/${oltId}/console/${startedAt}.txt`,
    };
  }

  return {
    command: command.command,
    startedAt,
    durationMs: mockConfig.latencyMs,
    result: "ok",
    text: outputFor(commandId),
    evidenceRef: `evidence/${oltId}/console/${startedAt}.txt`,
  };
}
