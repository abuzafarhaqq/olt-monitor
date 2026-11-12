// Package collecting runs collection and crawl jobs against a device, on behalf
// of the API. Both are asynchronous: the request returns a run id and the work
// happens in a goroutine with its own context, streamed to the terminal.
package collecting

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector/vsol"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/live"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

// ErrAlreadyRunning is returned when a job is in flight for that OLT.
var ErrAlreadyRunning = errors.New("a job is already running for this device")

// SecretResolver is the narrow slice of secrets.Service this package needs.
type SecretResolver interface {
	Resolve(ctx context.Context, oltID string) (username, password string, err error)
}

// KeyStore records a host-key fingerprint trusted on first contact.
type KeyStore interface {
	SetHostKeyFingerprint(ctx context.Context, oltID, fingerprint string) error
}

// CommandStore persists a discovered CLI tree.
type CommandStore interface {
	SaveCommands(ctx context.Context, oltID, adapter string, commands []postgres.DiscoveredCommand, at time.Time) error
}

type Service struct {
	Pool *pgxpool.Pool

	Secrets  SecretResolver
	Keys     KeyStore
	Commands CommandStore
	Hub      *live.Hub

	// CLIPass and EnablePass fall back to the stored password: on the VSOL EPON
	// all three prompts accept the same account.
	CLIPass    string
	EnablePass string

	// FallbackUser and FallbackPass are used when no credential is stored for
	// the device — the account the CLI takes from .secrets/app.env.
	FallbackUser string
	FallbackPass string

	// AllowRun is the safety switch: false means "report the plan, dial nothing".
	AllowRun bool

	// Timeout is the whole-run budget; CommandTimeout is per command.
	Timeout        time.Duration
	CommandTimeout time.Duration

	KnownHosts string

	mu      sync.Mutex
	running map[string]bool
}

// Device is what is needed to dial an OLT.
type Device struct {
	ID                string
	Name              string
	Vendor            string
	Model             string
	Technology        string
	ManagementAddress string
	ManagementPort    int
	Connection        string
	TrustHostKey      bool
}

// Plan is the command list a run would execute, shown before anything is dialled.
type Plan struct {
	Adapter  string   `json:"adapter"`
	Commands []string `json:"commands"`
	Lines    []string `json:"lines"`
}

// CommandResult is one executed command, for the drawer and the log page.
type CommandResult struct {
	ID      string `json:"id"`
	Line    string `json:"line"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Summary string `json:"summary,omitempty"`
	Raw     string `json:"raw,omitempty"`
}

// AdapterFor resolves an adapter by vendor AND technology.
//
// The vendor alone is not enough: a VSOL GPON OLT is not a VSOL EPON OLT, and the
// EPON command set against it returns "% Unknown command" at best. An
// unregistered combination names the gap instead of guessing.
func AdapterFor(vendor, technology string) (collector.Adapter, string, error) {
	switch vendor + "/" + technology {
	case "VSOL/EPON":
		return vsol.New(), "vsol-epon", nil
	default:
		return nil, "", fmt.Errorf(
			"no adapter for %s %s yet — crawl it and its commands can be added as data", vendor, technology)
	}
}

// BuildPlan answers "what would you run" without touching the device.
func (s *Service) BuildPlan(device Device) (Plan, error) {
	adapter, name, err := AdapterFor(device.Vendor, device.Technology)
	if err != nil {
		return Plan{}, err
	}

	ids := []string{"version", "time"}
	for pon := 1; pon <= 4; pon++ {
		ids = append(ids, fmt.Sprintf("onu-status-pon%d", pon), fmt.Sprintf("opm-diag-pon%d", pon))
	}
	ids = append(ids, "onu-basic-info")

	plan := Plan{Adapter: name}
	for _, id := range ids {
		command, ok := adapter.Command(id)
		if !ok {
			continue
		}
		plan.Commands = append(plan.Commands, id)
		plan.Lines = append(plan.Lines, command.Line)
	}
	return plan, nil
}

/* -------------------------------------------------------------------------
 * Collection
 * ---------------------------------------------------------------------- */

// Start kicks off a collection and returns immediately.
//
// The request context is deliberately not used for the work: a browser that
// navigates away must not abort a run halfway and leave a row in 'running'.
func (s *Service) Start(ctx context.Context, device Device, triggeredBy string) (int64, error) {
	if !s.AllowRun {
		return 0, fmt.Errorf("device access is disabled on this server")
	}
	if _, _, err := AdapterFor(device.Vendor, device.Technology); err != nil {
		return 0, err
	}
	return s.startRun(ctx, device, "full", triggeredBy, s.executeCollection)
}

// startRun is the shared begin: lock, allocate the id, open the hub, go.
func (s *Service) startRun(
	ctx context.Context,
	device Device,
	jobType, triggeredBy string,
	work func(device Device, runID int64, ing collector.Ingest),
) (int64, error) {
	if !s.tryLock(device.ID) {
		return 0, ErrAlreadyRunning
	}

	ing := collector.Ingest{Pool: s.Pool, ParserVersion: "vsol-epon@0.6"}
	runID, err := ing.StartRun(ctx, device.ID, jobType, triggeredBy)
	if err != nil {
		s.unlock(device.ID)
		return 0, err
	}
	if s.Hub != nil {
		s.Hub.Open(runID)
	}

	go work(device, runID, ing)
	return runID, nil
}

func (s *Service) executeCollection(device Device, runID int64, ing collector.Ingest) {
	defer s.unlock(device.ID)

	ctx, cancel := context.WithTimeout(context.Background(), s.runBudget())
	defer cancel()

	result, reason, events := "successful", "", 0
	publish := s.publisher(runID, &events)
	defer s.finish(runID, ing, &result, &reason, &events)

	publish("start", "collecting "+device.Name)

	session, err := s.dial(ctx, device, publish)
	if err != nil {
		result, reason = "failed", err.Error()
		publish("error", reason)
		_ = ing.RecordDeviceState(context.Background(), device.ID, collector.DeviceState{
			Reachable: false, CollectionStatus: "failed",
		}, time.Now().UTC())
		return
	}
	defer session.Close()

	// Streaming starts only now: the login sequence is never part of the stream.
	coalescer := collector.NewCoalescer(func(kind, data string) { publish(kind, data) }, 64<<10, 60*time.Millisecond)
	defer coalescer.Close()
	observer := collector.Tee(collector.NewRedactor(s.sessionSecrets(device)...), coalescer.Handle)
	session.SetObserver(observer)

	adapter, _, _ := AdapterFor(device.Vendor, device.Technology)
	plan, err := s.BuildPlan(device)
	if err != nil {
		result, reason = "failed", err.Error()
		publish("error", reason)
		return
	}

	runner := collector.Runner{Adapter: adapter, Session: session, Observe: observer}
	started := time.Now().UTC()
	results, err := runner.Run(ctx, plan.Commands)
	if err != nil {
		result, reason = "failed", err.Error()
		publish("error", reason)
		return
	}

	jobs := make([]collector.Job, 0, len(results))
	state := collector.DeviceState{Reachable: true, CollectionStatus: "successful", ClockTrusted: true}
	for _, res := range results {
		job := collector.Job{ResultID: res.CommandID, Line: res.Line, Raw: res.Raw, Err: res.Err}
		switch rows := res.Parsed.(type) {
		case []vsol.OnuRow:
			job.Rows = vsol.ToStatusRows(rows)
		case []vsol.OnuIdentity:
			job.Rows = vsol.ToIdentityRows(rows)
		case []vsol.OpticalRow:
			job.Rows = vsol.ToOpticalRows(rows)
		case vsol.Version:
			state.Firmware, state.Hardware = rows.Software, rows.Hardware
		case vsol.Clock:
			state.ClockTrusted = rows.Trusted
		}
		if res.Err != nil {
			result = "partial"
			reason = res.CommandID + ": " + res.Err.Error()
			publish("error", reason)
		}
		jobs = append(jobs, job)
	}
	state.CollectionStatus = result

	// What the device said about itself — this is what the inventory's
	// "last collection" column reads.
	if err := ing.RecordDeviceState(ctx, device.ID, state, started); err != nil {
		publish("error", "device state: "+err.Error())
	}

	if err := ing.Append(ctx, runID, device.ID, started, jobs); err != nil {
		result, reason = "failed", "store: "+err.Error()
		publish("error", reason)
		return
	}
	publish("info", "stored — "+result)
}

/* -------------------------------------------------------------------------
 * Crawl
 * ---------------------------------------------------------------------- */

// StartCrawl discovers the device's CLI tree and stores it. Same 202 shape as a
// collection, and the same per-device lock: two jobs on one shell would
// interleave prompts.
func (s *Service) StartCrawl(ctx context.Context, device Device, triggeredBy string) (int64, error) {
	if !s.AllowRun {
		return 0, fmt.Errorf("device access is disabled on this server")
	}
	if s.Commands == nil {
		return 0, fmt.Errorf("command storage is not configured")
	}
	return s.startRun(ctx, device, "crawl", triggeredBy, s.executeCrawl)
}

func (s *Service) executeCrawl(device Device, runID int64, ing collector.Ingest) {
	defer s.unlock(device.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, reason, events := "successful", "", 0
	publish := s.publisher(runID, &events)
	defer s.finish(runID, ing, &result, &reason, &events)

	publish("start", "crawling "+device.Name)

	session, err := s.dial(ctx, device, publish)
	if err != nil {
		result, reason = "failed", err.Error()
		publish("error", reason)
		return
	}
	defer session.Close()

	_, adapterName, err := AdapterFor(device.Vendor, device.Technology)
	if err != nil {
		result, reason = "failed", err.Error()
		publish("error", reason)
		return
	}

	crawler := collector.Crawler{
		ProbeHelp: func(ctx context.Context, path string) (string, error) {
			publish("info", "probing "+path)
			return session.Probe(ctx, path)
		},
	}
	nodes, crawlErr := crawler.Crawl(ctx)
	if crawlErr != nil {
		// A partial tree is still worth storing.
		reason = crawlErr.Error()
		publish("error", reason)
	}

	commands := make([]postgres.DiscoveredCommand, 0, len(nodes))
	for _, node := range nodes {
		commands = append(commands, postgres.DiscoveredCommand{
			Path: node.Command(), Help: node.Help, Kind: string(node.Kind), Depth: node.Depth,
		})
	}

	if err := s.Commands.SaveCommands(ctx, device.ID, adapterName, commands, time.Now().UTC()); err != nil {
		result, reason = "failed", "store: "+err.Error()
		publish("error", reason)
		return
	}

	publish("info", fmt.Sprintf("%d commands stored", len(commands)))
	if crawlErr != nil && len(commands) == 0 {
		result = "failed"
	}
}

/* -------------------------------------------------------------------------
 * Shared plumbing
 * ---------------------------------------------------------------------- */

// dial resolves credentials, connects, trusts a new host key if the device
// record allows it, and enters configuration mode.
func (s *Service) dial(ctx context.Context, device Device, publish func(kind, data string)) (*collector.SSHSession, error) {
	if s.Secrets == nil {
		return nil, fmt.Errorf("secret storage is not configured")
	}

	username, password, err := s.Secrets.Resolve(ctx, device.ID)
	if err != nil {
		if s.FallbackUser == "" || s.FallbackPass == "" {
			return nil, fmt.Errorf(
				"no stored credential for %s — set it on the device page", device.Name)
		}
		username, password = s.FallbackUser, s.FallbackPass
		publish("info", "using credentials from the environment (no stored credential)")
	}

	cliPass := firstNonEmpty(s.CLIPass, password)
	enablePass := firstNonEmpty(s.EnablePass, password)

	commandTimeout := s.CommandTimeout
	if commandTimeout == 0 {
		commandTimeout = 5 * time.Minute
	}

	session, err := collector.DialSSH(ctx, collector.SSHConfig{
		Host: device.ManagementAddress, Port: device.ManagementPort, User: username,
		Password: password, CLIUser: username, CLIPassword: cliPass, EnablePassword: enablePass,
		Timeout: 20 * time.Second, CommandTimeout: commandTimeout,
		KnownHosts: s.KnownHosts, AcceptNewKey: device.TrustHostKey,
		OnNewKey: func(fingerprint string) {
			publish("info", "trusting new host key "+fingerprint)
			if s.Keys != nil {
				_ = s.Keys.SetHostKeyFingerprint(context.Background(), device.ID, fingerprint)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	adapter, _, err := AdapterFor(device.Vendor, device.Technology)
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	if adapter.RequiresConfigMode() {
		if _, err := session.Run(ctx, "configure terminal"); err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("enter configuration mode: %w", err)
		}
		publish("info", "configuration mode")
	}
	return session, nil
}

// publisher returns a hub-publishing function that counts events.
func (s *Service) publisher(runID int64, events *int) func(kind, data string) {
	return func(kind, data string) {
		if s.Hub == nil {
			return
		}
		s.Hub.Publish(runID, kind, data)
		*events++
	}
}

// finish closes the hub and the run row whatever happened.
func (s *Service) finish(runID int64, ing collector.Ingest, result, reason *string, events *int) {
	if s.Hub != nil {
		s.Hub.Close(runID, "end", *result)
		if dropped := s.Hub.Dropped(runID); dropped > 0 {
			*reason = strings.TrimSpace(*reason + " (stream dropped " + strconv.Itoa(dropped) + " events)")
		}
	}
	if err := ing.FinishRun(context.Background(), runID, *result, *reason, *events); err != nil {
		_ = err
	}
}

func (s *Service) runBudget() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 45 * time.Minute
}

// sessionSecrets are the strings masked in the stream.
func (s *Service) sessionSecrets(device Device) []string {
	if s.Secrets == nil {
		return nil
	}
	_, password, err := s.Secrets.Resolve(context.Background(), device.ID)
	if err != nil {
		return []string{s.FallbackPass}
	}
	return []string{password, firstNonEmpty(s.CLIPass, password), firstNonEmpty(s.EnablePass, password)}
}

func (s *Service) tryLock(oltID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		s.running = map[string]bool{}
	}
	if s.running[oltID] {
		return false
	}
	s.running[oltID] = true
	return true
}

func (s *Service) unlock(oltID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, oltID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
