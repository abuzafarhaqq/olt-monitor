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

/* -------------------------------------------------------------------------
 * Collection health (§5.8). Kept beside the device types because the whole
 * point is that they are separate: a job result never rewrites ONU state.
 * ---------------------------------------------------------------------- */

/** One adapter-approved query. Arbitrary CLI is not a job type. */
export type CollectionJobType =
  | "onu-status"
  | "onu-optical"
  | "onu-basic-info"
  | "syslog-onu-events"
  | "alarm-catalog"
  | "device-info";

/**
 * Why a run failed, in the vocabulary an operator can act on:
 * connectivity → network, auth → credentials, parser → our code,
 * capability → the device does not expose it.
 */
export type JobErrorKind = "timeout" | "auth" | "network" | "parser" | "capability";

export interface JobError {
  kind: JobErrorKind;
  message: string;
}

export interface CollectionJob {
  id: string;
  oltId: string;
  type: CollectionJobType;
  result: CollectionJobResult;
  /** Ingestion clock — the only timestamp we trust. */
  lastRunAt: string;
  durationMs: number | null;
  error: JobError | null;
  /**
   * ONUs whose stored status was deliberately left untouched because this run
   * produced no authoritative answer. null = the run did not touch ONU state.
   */
  stateLeftUnchanged: number | null;
}

export type CollectorState = "idle" | "running" | "degraded" | "stopped";

export interface Collector {
  id: string;
  name: string;
  state: CollectorState;
  runningJobs: number;
  queuedJobs: number;
  /** Collector heartbeat. null = never reported since start. */
  heartbeatAt: string | null;
}

export interface StaleSource {
  oltId: string;
  lastSuccessAt: string | null;
  ageMinutes: number;
  /** Collection profile interval, so "age 47m" can be judged against 5m. */
  intervalMinutes: number;
  reason: string;
}

export interface CollectionHealthResponse {
  generatedAt: string;
  collectors: Collector[];
  jobs: CollectionJob[];
  /** Device identity only — the page never needs full OLT records. */
  olts: Olt[];
  staleSources: StaleSource[];
}

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
  /** Hardware revision as printed by `show version`. */
  hardwareVersion: string;
  /** Boot uptime in seconds; null when the adapter cannot report it. */
  uptimeSeconds: number | null;
  ponCount: number;
  lastCollection: DeviceTimestamps | null;
  collectionStatus: CollectionStatus;
  /** Last time the device answered. lastCollection.ingestedAt is the last attempt. */
  lastSuccessAt?: string | null;
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
  /** Last successful collection for this port — per PON, not per OLT. */
  lastCollection: DeviceTimestamps | null;
  collectionStatus: CollectionStatus;
  /** Last time the device answered. lastCollection.ingestedAt is the last attempt. */
  lastSuccessAt?: string | null;
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
  /** Last time the device reported this ONU online. */
  lastOnlineAt: DeviceTimestamps | null;
  lastRegistrationAt: DeviceTimestamps | null;
  lastDeregistrationAt: DeviceTimestamps | null;
  /** Deregistrations in the last 24h — the evidence behind "flapping". */
  deregistrations24h: number;
  collectionStatus: CollectionStatus;

  /** ONU vendor from basic-info (MONU, HWTC, ZTE…) — not the OLT's vendor. */
  vendorId?: string;
  onuType?: string;
  interfaceType?: string;
  identitySource?: "basic-info" | "status-only" | "unknown";
  thresholdModel?: string;
  /** false = only ever seen in the status table: an unprovisioned entry. */
  provisioned?: boolean;
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

export interface IncidentNote {
  id: string;
  author: string;
  at: string;
  text: string;
}

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
  /** Events correlated into this case — the evidence behind the hypothesis. */
  evidenceEventIds: string[];
  notes: IncidentNote[];
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

/* -------------------------------------------------------------------------
 * ONU diagnostics (§5.4) and optical diagnostics (§5.6)
 * ---------------------------------------------------------------------- */

/**
 * One reading. `supported: false` means the adapter has no command for this
 * metric on this model — different from "the command ran and returned nothing",
 * which is `value: null`.
 */
export interface Measurement {
  value: number | null;
  supported: boolean;
}

export interface OnuOpticalMeasurements {
  rxPowerDbm: Measurement;
  txPowerDbm: Measurement;
  temperatureC: Measurement;
  supplyVoltageV: Measurement;
  txBiasCurrentMa: Measurement;
  /** Device-reported distance, in metres. */
  distanceMeters: Measurement;
  /** Platform time of the measurement, not the device clock. */
  measuredAt: string | null;
  collectionStatus: CollectionStatus;
}

export interface OpticalHistoryPoint {
  at: string;
  /** null = no sample for that bucket. Never interpolated. */
  rxPowerDbm: number | null;
}

export interface RawEvidence {
  id: string;
  command: string;
  capturedAt: string;
  parserVersion: string;
  /** 0..1 — share of expected rows the parser accounted for. */
  completeness: number;
  text: string;
}

export interface OnuDiagnostics {
  onuId: string;
  measurements: OnuOpticalMeasurements;
  history: OpticalHistoryPoint[];
  evidence: RawEvidence[];
}

/* -------------------------------------------------------------------------
 * Diagnostics console (§5.9)
 * ---------------------------------------------------------------------- */

/** An adapter-approved command. The browser sends an id, never raw CLI. */
export interface DiagnosticCommand {
  id: string;
  technology: "EPON" | "GPON" | "any";
  label: string;
  command: string;
}

export interface DiagnosticResult {
  command: string;
  startedAt: string;
  durationMs: number;
  result: "ok" | "failed";
  text: string;
  evidenceRef: string;
}
