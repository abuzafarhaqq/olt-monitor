// Package memory is an in-memory Store used by tests and by `go run` before a
// database exists.
//
// The fixture is deliberately awkward: one unreachable OLT, one partial
// collection, one ONU with no optical command, a device clock from the year
// 2000 and two identical events. Those are the cases the rules exist for.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store"
)

type Store struct {
	mu        sync.RWMutex
	now       func() time.Time
	olts      []domain.Olt
	pons      []domain.Pon
	onus      []domain.Onu
	events    []domain.Event
	incidents []domain.Incident
	jobs      []domain.CollectionJob
	history   map[string][]domain.OpticalPoint
}

// Option keeps the constructor honest about time: tests fix the clock so
// timestamps are comparable.
type Option func(*Store)

func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

func New(opts ...Option) *Store {
	s := &Store{now: time.Now, history: map[string][]domain.OpticalPoint{}}
	for _, opt := range opts {
		opt(s)
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	now := s.now().UTC()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	trusted := func(at time.Time) *domain.DeviceTimestamps {
		return &domain.DeviceTimestamps{IngestedAt: at, Quality: domain.QualityTrusted}
	}
	// OLT-01's clock is 26 years behind; its device timestamps cannot be used.
	untrusted := func(at time.Time) *domain.DeviceTimestamps {
		device := time.Date(2000, 5, 20, 4, 12, 33, 0, time.UTC)
		return &domain.DeviceTimestamps{DeviceTime: &device, IngestedAt: at, Quality: domain.QualityUntrusted}
	}

	s.olts = []domain.Olt{
		{
			ID: "olt-1", Name: "EPON-OLT-01", Vendor: "VSOL", Model: "EPON-OLT",
			Technology: "EPON", ManagementAddress: "192.0.2.11", Connection: "ssh",
			Reachable: true, Firmware: "demo-2.03.75", HardwareVersion: "V1.1.4",
			UptimeSeconds: ptr(int64(3_542_400)), PonCount: 2,
			LastCollection: untrusted(ago(3 * time.Minute)), CollectionStatus: domain.CollectionSuccessful,
		},
		{
			// Unreachable. Its ONUs are unknown, not offline.
			ID: "olt-2", Name: "GPON-OLT-02", Vendor: "C-Data", Model: "FD-OLT",
			Technology: "GPON", ManagementAddress: "192.0.2.14", Connection: "telnet",
			Reachable: false, Firmware: "unknown", HardwareVersion: "unknown",
			PonCount: 1, LastCollection: trusted(ago(21 * time.Minute)),
			CollectionStatus: domain.CollectionFailed,
		},
	}

	s.pons = []domain.Pon{
		{ID: "olt-1-pon1", OltID: "olt-1", Name: "epon 0/1", LastCollection: untrusted(ago(3 * time.Minute)), CollectionStatus: domain.CollectionSuccessful},
		{ID: "olt-1-pon2", OltID: "olt-1", Name: "epon 0/2", LastCollection: untrusted(ago(3 * time.Minute)), CollectionStatus: domain.CollectionPartial},
		{ID: "olt-2-pon1", OltID: "olt-2", Name: "gpon 0/1", LastCollection: trusted(ago(21 * time.Minute)), CollectionStatus: domain.CollectionFailed},
	}

	online := func(id, pon string, n int, rx float64) domain.Onu {
		return domain.Onu{
			ID: id, OltID: "olt-1", PonID: pon, OnuID: n, MAC: "02:1A:2B:00:00:00",
			Vendor: "VSOL", Model: "HG323", Status: domain.OnuOnline, RxPowerDbm: ptr(rx),
			DistanceMeters: ptr(420), OpticalSupported: true,
			LastEvent: ptr("onu-register"), LastOnlineAt: untrusted(ago(4 * time.Minute)),
			CollectionStatus: domain.CollectionSuccessful,
		}
	}

	// Two healthy ONUs, one critical reading, one with no optical command,
	// one offline — and on the unreachable OLT, two unknown.
	onu1 := online("olt-1-pon1-onu1", "olt-1-pon1", 1, -18.4)
	onu2 := online("olt-1-pon1-onu2", "olt-1-pon1", 2, -31.4)
	onu3 := online("olt-1-pon2-onu3", "olt-1-pon2", 3, 0)
	onu3.OpticalSupported = false
	onu3.RxPowerDbm = nil
	onu3.CollectionStatus = domain.CollectionUnsupported

	onu4 := online("olt-1-pon2-onu4", "olt-1-pon2", 4, -19.2)
	onu4.Status = domain.OnuOffline
	onu4.RxPowerDbm = nil
	onu4.Deregistrations24h = 3
	onu4.LastEvent = ptr("onu-deregister")

	unknown := func(id string, n int) domain.Onu {
		return domain.Onu{
			ID: id, OltID: "olt-2", PonID: "olt-2-pon1", OnuID: n,
			MAC: "02:1A:2B:99:00:0" + string(rune('0'+n)), Vendor: "C-Data", Model: "FD611GW",
			Status: domain.OnuUnknown, OpticalSupported: true,
			CollectionStatus: domain.CollectionFailed,
		}
	}
	s.onus = []domain.Onu{onu1, onu2, onu3, onu4, unknown("olt-2-pon1-onu1", 1), unknown("olt-2-pon1-onu2", 2)}

	s.events = []domain.Event{
		{
			ID: "evt-1", Type: "onu-deregister", Severity: domain.SeverityCritical,
			OltID: "olt-1", PonID: ptr("olt-1-pon2"), OnuID: ptr(4),
			Message: "ONU deregistered — dying gasp recorded",
			Timestamps: domain.DeviceTimestamps{
				DeviceTime: ptrTime(time.Date(2000, 5, 20, 4, 12, 33, 0, time.UTC)),
				IngestedAt: ago(4 * time.Minute), Quality: domain.QualityUntrusted,
			},
			EvidenceRef: "evidence/olt-1/syslog-deregister/2026-11-06T01-12Z.txt",
		},
		{
			// Same event, second capture, different evidence file.
			ID: "evt-2", Type: "onu-deregister", Severity: domain.SeverityCritical,
			OltID: "olt-1", PonID: ptr("olt-1-pon2"), OnuID: ptr(4),
			Message: "ONU deregistered — dying gasp recorded",
			Timestamps: domain.DeviceTimestamps{
				DeviceTime: ptrTime(time.Date(2000, 5, 20, 4, 12, 33, 0, time.UTC)),
				IngestedAt: ago(4 * time.Minute), Quality: domain.QualityUntrusted,
			},
			EvidenceRef: "evidence/olt-1/syslog-dying-gasp/2026-11-06T01-12Z.txt",
		},
		{
			ID: "evt-3", Type: "olt-unreachable", Severity: domain.SeverityCritical,
			OltID: "olt-2", Message: "OLT unreachable — Telnet timeout after 3 attempts",
			Timestamps:  domain.DeviceTimestamps{IngestedAt: ago(21 * time.Minute), Quality: domain.QualityMissing},
			EvidenceRef: "evidence/olt-2/collector/2026-11-06T00-55Z.log",
		},
	}

	s.incidents = []domain.Incident{{
		ID: "INC-2411-007", Title: "PON 0/2 flapping with dying-gasp cluster",
		State: domain.IncidentInvestigating, Severity: domain.SeverityCritical,
		OltID: "olt-1", PonID: ptr("olt-1-pon2"), OpenedAt: ago(3 * time.Hour),
		Hypothesis:       ptr("Shared PON outage suspected, not confirmed: the OLT is reachable and PON 0/1 is unaffected."),
		EvidenceEventIDs: []string{"evt-1", "evt-2"},
		Notes: []domain.IncidentNote{{
			ID: "note-1", Author: "NOC Operator", At: ago(90 * time.Minute),
			Text: "Dying gasp on ONU 4 only; the others simply deregistered.",
		}},
	}}

	s.jobs = []domain.CollectionJob{
		{ID: "job-1", OltID: "olt-1", Type: "onu-status", Result: domain.JobSuccessful,
			LastRunAt: ago(3 * time.Minute), DurationMs: ptr(int64(820))},
		{ID: "job-2", OltID: "olt-1", Type: "onu-optical", Result: domain.JobPartial,
			LastRunAt: ago(3 * time.Minute), DurationMs: ptr(int64(1240)),
			Error:              &domain.JobError{Kind: "parser", Message: "9 of 10 rows parsed"},
			StateLeftUnchanged: ptr(1)},
		{ID: "job-3", OltID: "olt-2", Type: "onu-status", Result: domain.JobFailed,
			LastRunAt: ago(21 * time.Minute), DurationMs: ptr(int64(15000)),
			Error:              &domain.JobError{Kind: "timeout", Message: "no prompt within 15 s"},
			StateLeftUnchanged: ptr(2)},
	}

	s.history["olt-1-pon1-onu2"] = []domain.OpticalPoint{
		{At: ago(2 * time.Hour), RxPowerDbm: ptr(-21.1)},
		{At: ago(1 * time.Hour), RxPowerDbm: nil},
		{At: now, RxPowerDbm: ptr(-31.4)},
	}

	s.recount()
}

// recount derives every count from the ONU list. Hand-written counters drift
// the moment the fixture changes; derived ones cannot.
func (s *Store) recount() {
	for i := range s.pons {
		s.pons[i].Counts = domain.OnuCounts{}
	}
	for _, onu := range s.onus {
		for i := range s.pons {
			if s.pons[i].ID != onu.PonID {
				continue
			}
			s.pons[i].Counts.Total++
			switch onu.Status {
			case domain.OnuOnline:
				s.pons[i].Counts.Online++
			case domain.OnuOffline:
				s.pons[i].Counts.Offline++
			default:
				s.pons[i].Counts.Unknown++
			}
			if onu.RxPowerDbm != nil && *onu.RxPowerDbm <= -27 {
				s.pons[i].LowOpticalCount++
			}
			if onu.Deregistrations24h >= 3 {
				s.pons[i].FlappingCount++
			}
			if onu.LastEvent != nil && *onu.LastEvent == "onu-dying-gasp" {
				s.pons[i].DyingGaspCount24h++
			}
		}
	}
}

func (s *Store) Overview(context.Context) (domain.Overview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	overview := domain.Overview{
		GeneratedAt: s.now().UTC(),
		Olts:        append([]domain.Olt(nil), s.olts...),
		Pons:        append([]domain.Pon(nil), s.pons...),
		Onus:        append([]domain.Onu(nil), s.onus...),
		Events:      append([]domain.Event(nil), s.events...),
		Incidents:   append([]domain.Incident(nil), s.incidents...),
	}
	// Six hourly buckets, shaped from the current ONU population.
	online, offline, unknown := 0, 0, 0
	for _, onu := range s.onus {
		switch onu.Status {
		case domain.OnuOnline:
			online++
		case domain.OnuOffline:
			offline++
		default:
			unknown++
		}
	}
	for hour := 5; hour >= 0; hour-- {
		at := s.now().UTC().Add(-time.Duration(hour) * time.Hour)
		overview.AvailabilityTrend = append(overview.AvailabilityTrend, domain.AvailabilityPoint{
			HourLabel: at.Format("15:04"),
			Online:    online + hour%2,
			Offline:   offline,
			Unknown:   unknown,
		})
	}
	return overview, nil
}

func (s *Store) Olts(context.Context) ([]domain.Olt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Olt(nil), s.olts...), nil
}

func (s *Store) Olt(_ context.Context, id string) (domain.Olt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, olt := range s.olts {
		if olt.ID == id {
			return olt, nil
		}
	}
	return domain.Olt{}, store.ErrNotFound
}

func (s *Store) PonsByOlt(_ context.Context, oltID string) ([]domain.Pon, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Pon{}
	for _, pon := range s.pons {
		if pon.OltID == oltID {
			out = append(out, pon)
		}
	}
	if len(out) == 0 {
		return nil, store.ErrNotFound
	}
	return out, nil
}

func (s *Store) OnusByPon(_ context.Context, ponID string) ([]domain.Onu, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Onu{}
	for _, onu := range s.onus {
		if onu.PonID == ponID {
			out = append(out, onu)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OnuID < out[j].OnuID })
	return out, nil
}

func (s *Store) Onu(_ context.Context, id string) (domain.Onu, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, onu := range s.onus {
		if onu.ID == id {
			return onu, nil
		}
	}
	return domain.Onu{}, store.ErrNotFound
}

func (s *Store) OpticalHistory(_ context.Context, onuID string) ([]domain.OpticalPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	points, ok := s.history[onuID]
	if !ok {
		return []domain.OpticalPoint{}, nil
	}
	return append([]domain.OpticalPoint(nil), points...), nil
}

func (s *Store) Events(context.Context) ([]domain.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]domain.Event(nil), s.events...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamps.IngestedAt.After(out[j].Timestamps.IngestedAt)
	})
	return out, nil
}

func (s *Store) Incidents(context.Context) ([]domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Incident(nil), s.incidents...), nil
}

func (s *Store) Incident(_ context.Context, id string) (domain.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, incident := range s.incidents {
		if incident.ID == id {
			return incident, nil
		}
	}
	return domain.Incident{}, store.ErrNotFound
}

func (s *Store) CollectionJobs(context.Context) ([]domain.CollectionJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.CollectionJob(nil), s.jobs...), nil
}

func ptr[T any](v T) *T { return &v }

func ptrTime(t time.Time) *time.Time { return &t }
