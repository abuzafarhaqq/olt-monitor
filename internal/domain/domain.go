// Package domain holds the types the API serves. JSON field names match the
// TypeScript types in the frontend's src/types/domain.ts exactly — the contract
// was written first, and these structs exist to satisfy it, not the other way
// round.
package domain

import "time"

// OnuStatus is the device's own state. It is never inferred from a failed
// collection: "we could not ask" is OnuUnknown, not OnuOffline.
type OnuStatus string

const (
	OnuOnline  OnuStatus = "online"
	OnuOffline OnuStatus = "offline"
	OnuUnknown OnuStatus = "unknown"
)

// CollectionStatus describes how much we know, independently of OnuStatus.
type CollectionStatus string

const (
	CollectionSuccessful  CollectionStatus = "successful"
	CollectionPartial     CollectionStatus = "partial"
	CollectionFailed      CollectionStatus = "failed"
	CollectionStale       CollectionStatus = "stale"
	CollectionUnsupported CollectionStatus = "unsupported"
)

// CollectionJobResult is the outcome of one job run (§5.8). Separate from
// CollectionStatus because a job and an ONU are different subjects.
type CollectionJobResult string

const (
	JobSuccessful CollectionJobResult = "successful"
	JobPartial    CollectionJobResult = "partial"
	JobFailed     CollectionJobResult = "failed"
	JobSkipped    CollectionJobResult = "skipped"
)

// TimestampQuality marks whether the device clock can be trusted. The first
// VSOL EPON reports May 2000, so this is not a theoretical field.
type TimestampQuality string

const (
	QualityTrusted   TimestampQuality = "trusted"
	QualityUntrusted TimestampQuality = "untrusted"
	QualityMissing   TimestampQuality = "missing"
)

// DeviceTimestamps keeps the device's claim and our observation apart.
type DeviceTimestamps struct {
	DeviceTime *time.Time       `json:"deviceTime"`
	IngestedAt time.Time        `json:"ingestedAt"`
	Quality    TimestampQuality `json:"quality"`
}

type Olt struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Vendor            string            `json:"vendor"`
	Model             string            `json:"model"`
	Technology        string            `json:"technology"`
	ManagementAddress string            `json:"managementAddress"`
	Connection        string            `json:"connection"`
	Reachable         bool              `json:"reachable"`
	Firmware          string            `json:"firmware"`
	HardwareVersion   string            `json:"hardwareVersion"`
	UptimeSeconds     *int64            `json:"uptimeSeconds"`
	PonCount          int               `json:"ponCount"`
	LastCollection    *DeviceTimestamps `json:"lastCollection"`
	CollectionStatus  CollectionStatus  `json:"collectionStatus"`
}

// OnuCounts keeps unknown as a first-class bucket.
type OnuCounts struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
	Unknown int `json:"unknown"`
}

type Pon struct {
	ID                string            `json:"id"`
	OltID             string            `json:"oltId"`
	Name              string            `json:"name"`
	Counts            OnuCounts         `json:"counts"`
	LowOpticalCount   int               `json:"lowOpticalCount"`
	DyingGaspCount24h int               `json:"dyingGaspCount24h"`
	FlappingCount     int               `json:"flappingCount"`
	LastCollection    *DeviceTimestamps `json:"lastCollection"`
	CollectionStatus  CollectionStatus  `json:"collectionStatus"`
}

type Onu struct {
	ID                   string            `json:"id"`
	OltID                string            `json:"oltId"`
	PonID                string            `json:"ponId"`
	OnuID                int               `json:"onuId"`
	MAC                  string            `json:"mac"`
	Vendor               string            `json:"vendor"`
	Model                string            `json:"model"`
	Status               OnuStatus         `json:"status"`
	RxPowerDbm           *float64          `json:"rxPowerDbm"`
	DistanceMeters       *int              `json:"distanceMeters"`
	OpticalSupported     bool              `json:"opticalSupported"`
	LastEvent            *string           `json:"lastEvent"`
	LastOnlineAt         *DeviceTimestamps `json:"lastOnlineAt"`
	LastRegistrationAt   *DeviceTimestamps `json:"lastRegistrationAt"`
	LastDeregistrationAt *DeviceTimestamps `json:"lastDeregistrationAt"`
	Deregistrations24h   int               `json:"deregistrations24h"`
	CollectionStatus     CollectionStatus  `json:"collectionStatus"`
}

type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityCritical EventSeverity = "critical"
)

type Event struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Severity    EventSeverity    `json:"severity"`
	OltID       string           `json:"oltId"`
	PonID       *string          `json:"ponId"`
	OnuID       *int             `json:"onuId"`
	Message     string           `json:"message"`
	Timestamps  DeviceTimestamps `json:"timestamps"`
	EvidenceRef string           `json:"evidenceRef"`
}

type IncidentState string

const (
	IncidentNew           IncidentState = "new"
	IncidentAcknowledged  IncidentState = "acknowledged"
	IncidentInvestigating IncidentState = "investigating"
	IncidentResolved      IncidentState = "resolved"
	IncidentClosed        IncidentState = "closed"
)

type IncidentNote struct {
	ID     string    `json:"id"`
	Author string    `json:"author"`
	At     time.Time `json:"at"`
	Text   string    `json:"text"`
}

type Incident struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	State            IncidentState  `json:"state"`
	Severity         EventSeverity  `json:"severity"`
	OltID            string         `json:"oltId"`
	PonID            *string        `json:"ponId"`
	OpenedAt         time.Time      `json:"openedAt"`
	Hypothesis       *string        `json:"hypothesis"`
	EvidenceEventIDs []string       `json:"evidenceEventIds"`
	Notes            []IncidentNote `json:"notes"`
}

type JobError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type CollectionJob struct {
	ID                 string              `json:"id"`
	OltID              string              `json:"oltId"`
	Type               string              `json:"type"`
	Result             CollectionJobResult `json:"result"`
	LastRunAt          time.Time           `json:"lastRunAt"`
	DurationMs         *int64              `json:"durationMs"`
	Error              *JobError           `json:"error"`
	StateLeftUnchanged *int                `json:"stateLeftUnchanged"`
}

type AvailabilityPoint struct {
	HourLabel string `json:"hourLabel"`
	Online    int    `json:"online"`
	Offline   int    `json:"offline"`
	Unknown   int    `json:"unknown"`
}

// Overview is exactly what GET /api/v1/dashboard/overview returns.
type Overview struct {
	GeneratedAt       time.Time           `json:"generatedAt"`
	Olts              []Olt               `json:"olts"`
	Pons              []Pon               `json:"pons"`
	Onus              []Onu               `json:"onus"`
	Events            []Event             `json:"events"`
	Incidents         []Incident          `json:"incidents"`
	AvailabilityTrend []AvailabilityPoint `json:"availabilityTrend"`
}

// OpticalPoint is one stored RX sample. A missing sample is a gap in the
// series, never a zero.
type OpticalPoint struct {
	At         time.Time `json:"at"`
	RxPowerDbm *float64  `json:"rxPowerDbm"`
}
