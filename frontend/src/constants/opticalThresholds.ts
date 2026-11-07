import type { Technology, Vendor } from "../types/domain";

/**
 * Optical thresholds are per vendor/model/firmware, never global (§5.4, §5.11).
 * One universal RX threshold would flag healthy EPON ONUs as critical.
 */
export interface OpticalThreshold {
  /** Above this, the reading is degraded. */
  warningDbm: number;
  /** At or below this, the reading is critical. */
  criticalDbm: number;
}

/**
 * "unconfigured" is separate from "unsupported": the adapter may return a
 * reading, but no calibrated threshold exists for this vendor/model pair, so
 * the platform must not call it healthy or critical. Inventing a verdict there
 * is how a NOC learns to ignore its own alarms.
 */
export type OpticalSeverity =
  | "ok"
  | "warning"
  | "critical"
  | "missing"
  | "unsupported"
  | "unconfigured";

/**
 * Partial on purpose: we only declare the combinations we have actually seen.
 * Inventing a threshold for an unseen vendor/model pair would be worse than
 * refusing to judge the reading.
 */
const THRESHOLDS: Partial<Record<`${Vendor}/${Technology}`, OpticalThreshold>> = {
  // VSOL EPON — measured values sit around -18 to -22 dBm.
  "VSOL/EPON": { warningDbm: -24, criticalDbm: -27 },
  // VSOL GPON, class B+ optics.
  "VSOL/GPON": { warningDbm: -27, criticalDbm: -29 },
  // C-Data GPON, class B+ optics with a wider tolerance window.
  "C-Data/GPON": { warningDbm: -26, criticalDbm: -28 },
};

export function getThreshold(vendor: Vendor, technology: Technology): OpticalThreshold {
  const threshold = THRESHOLDS[`${vendor}/${technology}`];
  if (!threshold) {
    // Loud failure: a missing threshold must be configured, not guessed.
    throw new Error(`No optical threshold configured for ${vendor}/${technology}`);
  }
  return threshold;
}

/**
 * Classify one RX reading. Missing data is "missing", never "critical":
 * absence of a measurement is not evidence of a fault.
 */
export function classifyRx(
  rxPowerDbm: number | null,
  vendor: Vendor,
  technology: Technology,
  opticalSupported: boolean,
): OpticalSeverity {
  if (!opticalSupported) return "unsupported";
  if (rxPowerDbm === null) return "missing";

  const threshold = THRESHOLDS[`${vendor}/${technology}`];
  if (!threshold) return "unconfigured";

  if (rxPowerDbm <= threshold.criticalDbm) return "critical";
  if (rxPowerDbm <= threshold.warningDbm) return "warning";
  return "ok";
}

/**
 * Non-throwing lookup for UI that must render *something* when a vendor/model
 * pair has no calibrated threshold. null = "no verdict available".
 */
export function findThreshold(
  vendor: Vendor,
  technology: Technology,
): OpticalThreshold | null {
  return THRESHOLDS[`${vendor}/${technology}`] ?? null;
}

/** Read-only view of the calibrated pairs, for the settings page. */
export function listThresholds(): { pair: string; warningDbm: number; criticalDbm: number }[] {
  return Object.entries(THRESHOLDS)
    .filter((entry): entry is [string, OpticalThreshold] => entry[1] !== undefined)
    .map(([pair, threshold]) => ({
      pair,
      warningDbm: threshold.warningDbm,
      criticalDbm: threshold.criticalDbm,
    }));
}
