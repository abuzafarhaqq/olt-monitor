/**
 * Domain types — the shape of the data, independent of how it is drawn.
 *
 * Two unions here are the heart of the product:
 *   OnuStatus        = what the device says about the ONU
 *   CollectionStatus = what our collector managed to learn
 * They are never merged. A failed collection does not make an ONU offline.
 */

export type OnuStatus = "online" | "offline" | "unknown";

export type CollectionStatus =
  | "successful"
  | "partial"
  | "failed"
  | "stale"
  | "unsupported";

/** Collection *job* results (§5.8) — a separate concept from ONU collection status. */
export type CollectionJobResult =
  | "successful"
  | "partial"
  | "failed"
  | "skipped";

export type Technology = "EPON" | "GPON";

export type Vendor = "VSOL" | "C-Data";

export type ConnectionMethod = "ssh" | "telnet";

/**
 * The device clock on the first VSOL EPON reports May 2000, so a timestamp is
 * only usable when we say how much we trust it.
 */
export type TimestampQuality = "trusted" | "untrusted" | "missing";

export interface DeviceTimestamps {
  /** Clock as reported by the OLT. May be wrong. */
  deviceTime: string | null;
  /** Clock of this platform, always trustworthy. */
  ingestedAt: string;
  quality: TimestampQuality;
}

export interface Olt {
  id: string;
  name: string;
  vendor: Vendor;
  model: string;
  technology: Technology;
  /** Management address. Never published outside the NOC. */
  managementAddress: string;
  connection: ConnectionMethod;
  reachable: boolean;
  firmware: string;
  ponCount: number;
  lastCollection: DeviceTimestamps | null;
  collectionStatus: CollectionStatus;
}

/** ONU counts per PON. `unknown` is a first-class bucket, never folded into offline. */
export interface OnuCounts {
  total: number;
  online: number;
  offline: number;
  unknown: number;
}

export interface Pon {
  id: string;
  oltId: string;
  /** Name as the adapter reports it — EPON and GPON do not name ports the same way. */
  name: string;
  counts: OnuCounts;
  lowOpticalCount: number;
  dyingGaspCount24h: number;
  flappingCount: number;
  collectionStatus: CollectionStatus;
}

export interface Onu {
  id: string;
  oltId: string;
  ponId: string;
  /** ONU ID inside its PON, as printed by the OLT. */
  onuId: number;
  /** Masked in mock data — real MACs never go into examples. */
  mac: string;
  vendor: Vendor;
  model: string;
  status: OnuStatus;
  /** null = no reading. Unsupported is not zero. */
  rxPowerDbm: number | null;
  distanceMeters: number | null;
  /** false = the adapter has no optical command for this model. */
  opticalSupported: boolean;
  lastEvent: string | null;
  collectionStatus: CollectionStatus;
}

export type EventType =
  | "onu-register"
  | "onu-deregister"
  | "onu-dying-gasp"
  | "onu-auth-success"
  | "onu-auth-failure"
  | "link-lost"
  | "pon-los"
  | "olt-unreachable"
  | "collector-failure";

export type EventSeverity = "info" | "warning" | "critical";

export interface NetworkEvent {
  id: string;
  type: EventType;
  severity: EventSeverity;
  oltId: string;
  ponId: string | null;
  onuId: number | null;
  message: string;
  timestamps: DeviceTimestamps;
  /** Pointer to the stored raw CLI output. Evidence is never discarded. */
  evidenceRef: string;
}

export type IncidentState =
  | "new"
  | "acknowledged"
  | "investigating"
  | "resolved"
  | "closed";

export interface Incident {
  id: string;
  title: string;
  state: IncidentState;
  severity: EventSeverity;
  oltId: string;
  ponId: string | null;
  openedAt: string;
  /** A suspected PON-wide outage is a hypothesis, not a root cause. */
  hypothesis?: string;
}

export interface AvailabilityPoint {
  /** Hour bucket label, e.g. "13:00". */
  hourLabel: string;
  online: number;
  offline: number;
  unknown: number;
}

/** Exactly what GET /api/v1/dashboard/overview will return in Stage 2. */
export interface OverviewResponse {
  generatedAt: string;
  olts: Olt[];
  pons: Pon[];
  onus: Onu[];
  events: NetworkEvent[];
  incidents: Incident[];
  availabilityTrend: AvailabilityPoint[];
}
