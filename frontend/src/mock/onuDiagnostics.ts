/**
 * MOCK DATA — per-ONU diagnostics. Derived from MOCK_OVERVIEW so the numbers
 * agree with the tables, then given deterministic detail (measurements,
 * 24-hour optical history, raw CLI evidence).
 *
 * Nothing here is a live measurement, and every screen that shows it carries
 * the MOCK DATA flag.
 */

import type {
  Measurement,
  Onu,
  OnuDiagnostics,
  OpticalHistoryPoint,
  RawEvidence,
} from "../types/domain";
import { MOCK_OVERVIEW } from "./mockData";

const HOUR = 3_600_000;
const NOW = Date.now();

const isoAgo = (ms: number): string => new Date(NOW - ms).toISOString();

/** Small deterministic hash so each ONU gets stable, different detail. */
function hash(id: string): number {
  let value = 2166136261;
  for (let index = 0; index < id.length; index += 1) {
    value ^= id.charCodeAt(index);
    value = Math.imul(value, 16777619);
  }
  return Math.abs(value);
}

const measurement = (value: number | null, supported = true): Measurement => ({ value, supported });

function buildHistory(onu: Onu, seed: number): OpticalHistoryPoint[] {
  const points: OpticalHistoryPoint[] = [];
  if (onu.rxPowerDbm === null) {
    // No reading is not the same as a reading of zero: the series stays empty.
    return Array.from({ length: 24 }, (_, index) => ({
      at: isoAgo((23 - index) * HOUR),
      rxPowerDbm: null,
    }));
  }

  for (let index = 23; index >= 0; index -= 1) {
    const wobble = Math.sin((seed % 17) + index / 2.6) * 0.9;
    const dip = index > 14 && index < 18 ? -6.5 : 0; // a visible outage window
    const gap = seed % 7 === 0 && index === 9; // one missing sample, never filled
    points.push({
      at: isoAgo(index * HOUR),
      rxPowerDbm: gap ? null : Math.round((onu.rxPowerDbm + wobble + dip) * 10) / 10,
    });
  }
  return points;
}

function buildEvidence(onu: Onu, seed: number): RawEvidence[] {
  if (!onu.opticalSupported) return [];

  const ponNumber = Number(onu.ponId.split("-pon")[1]);
  const command =
    onu.vendor === "C-Data"
      ? `show onu optics ${onu.onuId}`
      : `show onu opm-diag pon ${ponNumber}`;

  const body = [
    "ONU-ID  RX(dBm)  TX(dBm)  Temp(C)  Volt(V)  Bias(mA)",
    `${String(onu.onuId).padEnd(7)} ${String(onu.rxPowerDbm ?? "--").padEnd(8)} 2.41     46.2     3.29     11.4`,
    "",
    `# captured by collector-0${1 + (seed % 3)} at ${new Date(NOW).toISOString()}`,
  ].join("\n");

  const evidence: RawEvidence[] = [
    {
      id: `${onu.id}-ev-1`,
      command,
      capturedAt: isoAgo((3 + (seed % 9)) * 60_000),
      parserVersion: onu.vendor === "C-Data" ? "cdata-gpon@0.4.1" : "vsol-epon@0.6.2",
      completeness: seed % 5 === 0 ? 0.9 : 1,
      text: body,
    },
  ];

  if (seed % 5 === 0) {
    // A second capture that failed to parse fully — kept, not overwritten.
    evidence.push({
      id: `${onu.id}-ev-2`,
      command,
      capturedAt: isoAgo((60 + (seed % 20)) * 60_000),
      parserVersion: onu.vendor === "C-Data" ? "cdata-gpon@0.3.9" : "vsol-epon@0.6.1",
      completeness: 0.5,
      text: `${body}\n\n# WARN parser: 1 of 2 rows matched — column layout changed after firmware upgrade`,
    });
  }

  return evidence;
}

function buildDiagnostics(onu: Onu): OnuDiagnostics {
  const seed = hash(onu.id);
  const online = onu.status === "online";
  const opticalOk = onu.opticalSupported && onu.rxPowerDbm !== null;
  const gpon = onu.oltId === "olt-1" ? false : true;

  return {
    onuId: onu.id,
    measurements: {
      rxPowerDbm: measurement(onu.rxPowerDbm, onu.opticalSupported),
      // EPON adapters here expose RX only; GPON also reports TX, temperature,
      // voltage and bias. Unsupported is a capability fact, not a zero.
      txPowerDbm: measurement(opticalOk && gpon ? Math.round((2 + (seed % 14) / 10) * 100) / 100 : null, opticalOk && gpon),
      temperatureC: measurement(opticalOk && gpon ? Math.round((38 + (seed % 120) / 10) * 10) / 10 : null, opticalOk && gpon),
      supplyVoltageV: measurement(opticalOk && gpon ? Math.round((320 + (seed % 15)) / 100 * 100) / 100 : null, opticalOk && gpon),
      txBiasCurrentMa: measurement(opticalOk && gpon ? Math.round((7 + (seed % 90) / 10) * 10) / 10 : null, opticalOk && gpon),
      distanceMeters: measurement(onu.distanceMeters, onu.distanceMeters !== null),
      measuredAt: online ? isoAgo((3 + (seed % 7)) * 60_000) : null,
      collectionStatus: onu.collectionStatus,
    },
    history: buildHistory(onu, seed),
    evidence: buildEvidence(onu, seed),
  };
}

export const ONU_DIAGNOSTICS: Map<string, OnuDiagnostics> = new Map(
  MOCK_OVERVIEW.onus.map((onu) => [onu.id, buildDiagnostics(onu)]),
);
