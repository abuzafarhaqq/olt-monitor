// Package collecting runs one collection against one device, on behalf of the
// API. It exists because "refresh this OLT" has to be a server action: the
// browser never speaks SSH, and the credential only lives in the secret store.
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
)

// SecretResolver is the narrow slice of secrets.Service this package needs.
type SecretResolver interface {
	Resolve(ctx context.Context, oltID string) (username, password string, err error)
}

type Service struct {
	Pool *pgxpool.Pool

	Secrets SecretResolver
	// CLIPass and EnablePass fall back to the stored password: on the VSOL EPON
	// all three prompts accept the same account.
	CLIPass    string
	EnablePass string

	// KnownHosts verifies the device key. Empty means every connection is
	// refused — the same rule the CLI has, and the reason this failed the first
	// time it ran from the API.
	KnownHosts string

	// AllowRun is the safety switch: false means "report the plan, dial nothing".
	AllowRun bool
	// Timeout is the whole-run budget; CommandTimeout is per command. A 25,000
	// ONU estate prints megabytes and the two must not share one number.
	Timeout        time.Duration
	CommandTimeout time.Duration

	Hub Hub

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
}

// CommandResult is one executed command, for the UI and the log page.
type CommandResult struct {
	ID      string `json:"id"`
	Line    string `json:"line"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Summary string `json:"summary,omitempty"`
	Raw     string `json:"raw,omitempty"`
}

// Plan is the command list a run would execute. The UI shows it before anything
// is dialled — that is why it is a separate call.
type Plan struct {
	Adapter  string   `json:"adapter"`
	Commands []string `json:"commands"`
	Lines    []string `json:"lines"`
}

// AdapterFor picks the adapter for a device vendor.
func AdapterFor(vendor string) (collector.Adapter, string, error) {
	switch vendor {
	case "VSOL":
		return vsol.New(), "vsol-epon", nil
	default:
		return nil, "", fmt.Errorf("no adapter for vendor %q yet", vendor)
	}
}

// BuildPlan answers "what would you run" without touching the device.
func (s *Service) BuildPlan(device Device) (Plan, error) {
	adapter, name, err := AdapterFor(device.Vendor)
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

// Run executes the plan and ingests the result in one transaction.
func (s *Service) Run(ctx context.Context, device Device) (Plan, int64, []CommandResult, error) {
	plan, err := s.BuildPlan(device)
	if err != nil {
		return plan, 0, nil, err
	}
	if !s.AllowRun {
		return plan, 0, nil, fmt.Errorf("collection is disabled on this server")
	}
	if s.Secrets == nil {
		return plan, 0, nil, fmt.Errorf("secret storage is not configured")
	}

	username, password, err := s.Secrets.Resolve(ctx, device.ID)
	if err != nil {
		return plan, 0, nil, fmt.Errorf("credential for %s: %w", device.Name, err)
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 90 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := collector.DialSSH(dialCtx, collector.SSHConfig{
		Host: device.ManagementAddress, Port: device.ManagementPort, User: username,
		Password:       password,
		CLIUser:        username,
		CLIPassword:    firstNonEmpty(s.CLIPass, password),
		EnablePassword: firstNonEmpty(s.EnablePass, password),
		Timeout:        20 * time.Second,
		KnownHosts:     s.KnownHosts,
	})
	if err != nil {
		return plan, 0, nil, fmt.Errorf("connect: %w", err)
	}
	defer session.Close()

	adapter, _, err := AdapterFor(device.Vendor)
	if err != nil {
		return plan, 0, nil, err
	}
	if adapter.RequiresConfigMode() {
		if _, err := session.Run(dialCtx, "configure terminal"); err != nil {
			return plan, 0, nil, fmt.Errorf("enter configuration mode: %w", err)
		}
	}

	started := time.Now().UTC()
	results, err := collector.Runner{Adapter: adapter, Session: session}.Run(dialCtx, plan.Commands)
	if err != nil {
		return plan, 0, nil, err
	}

	out := make([]CommandResult, 0, len(results))
	for _, result := range results {
		entry := CommandResult{ID: result.CommandID, Line: result.Line, OK: result.Err == nil, Raw: result.Raw}
		if result.Err != nil {
			entry.Error = result.Err.Error()
		} else {
			entry.Summary = summarise(result.Parsed)
		}
		out = append(out, entry)
	}

	ing := collector.Ingest{Pool: s.Pool, ParserVersion: plan.Adapter + "@0.4"}
	run := collector.Run{OltID: device.ID, At: started}
	for _, result := range results {
		job := collector.Job{ResultID: result.CommandID, Line: result.Line, Raw: result.Raw, Err: result.Err}
		switch rows := result.Parsed.(type) {
		case []vsol.OnuRow:
			job.Rows = vsol.ToStatusRows(rows)
		case []vsol.OnuIdentity:
			job.Rows = vsol.ToIdentityRows(rows)
		case []vsol.OpticalRow:
			job.Rows = vsol.ToOpticalRows(rows)
		}
		if result.Err != nil {
			run.Failed = true
			run.Reason = result.Err.Error()
		}
		run.Jobs = append(run.Jobs, job)
	}

	runID, err := ing.Save(ctx, run)
	return plan, runID, out, err
}

// summarise is what the drawer shows per command: counts, not a wall of text.
func summarise(parsed any) string {
	switch rows := parsed.(type) {
	case []vsol.OnuRow:
		online := 0
		for _, row := range rows {
			if row.Status == "online" {
				online++
			}
		}
		return fmt.Sprintf("%d ONUs, %d online", len(rows), online)
	case []vsol.OnuIdentity:
		return fmt.Sprintf("%d ONUs with identity", len(rows))
	case []vsol.OpticalRow:
		return fmt.Sprintf("%d optical readings", len(rows))
	case vsol.Version:
		return fmt.Sprintf("%s %s / %s", rows.Model, rows.Hardware, rows.Software)
	case vsol.Clock:
		return fmt.Sprintf("device clock %q (trusted: %v)", rows.DeviceTime, rows.Trusted)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ErrAlreadyRunning is returned when a collection is in flight for that OLT.
var ErrAlreadyRunning = errors.New("a collection is already running for this device")

// Hub is the fan-out the terminal subscribes to.
type Hub interface {
	Open(runID int64)
	Publish(runID int64, kind, data string)
	Close(runID int64, kind, data string)
	Dropped(runID int64) int
}

// Start kicks off a collection and returns immediately.
//
// The request context is deliberately not used for the work: a browser that
// navigates away must not abort a run halfway through, leaving a device row in
// 'running' and no evidence. The goroutine gets its own timeout.
func (s *Service) Start(ctx context.Context, device Device) (int64, error) {
	if !s.AllowRun {
		return 0, fmt.Errorf("collection is disabled on this server")
	}
	if s.Secrets == nil {
		return 0, fmt.Errorf("secret storage is not configured")
	}
	if _, _, err := AdapterFor(device.Vendor); err != nil {
		return 0, err
	}
	if !s.tryLock(device.ID) {
		return 0, ErrAlreadyRunning
	}

	ing := collector.Ingest{Pool: s.Pool, ParserVersion: "vsol-epon@0.5"}
	runID, err := ing.StartRun(ctx, device.ID)
	if err != nil {
		s.unlock(device.ID)
		return 0, err
	}

	if s.Hub != nil {
		s.Hub.Open(runID)
	}
	go s.execute(device, runID, ing)
	return runID, nil
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

// execute is the background half. It owns its context, closes the run and
// releases the lock whatever happens.
func (s *Service) execute(device Device, runID int64, ing collector.Ingest) {
	defer s.unlock(device.ID)

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 45 * time.Minute
	}
	commandTimeout := s.CommandTimeout
	if commandTimeout == 0 {
		commandTimeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, reason := "successful", ""
	events := 0
	publish := func(kind, data string) {
		if s.Hub != nil {
			s.Hub.Publish(runID, kind, data)
			events++
		}
	}
	publish("start", device.Name)

	defer func() {
		if s.Hub != nil {
			s.Hub.Close(runID, "end", result)
			if dropped := s.Hub.Dropped(runID); dropped > 0 {
				reason = strings.TrimSpace(reason + " (stream dropped " + strconv.Itoa(dropped) + " events)")
			}
		}
		if err := ing.FinishRun(context.Background(), runID, result, reason, events); err != nil {
			s.log("finish run failed", err)
		}
	}()

	username, password, err := s.Secrets.Resolve(ctx, device.ID)
	if err != nil {
		result, reason = "failed", "credential: "+err.Error()
		publish("error", reason)
		return
	}

	cliPass := firstNonEmpty(s.CLIPass, password)
	enablePass := firstNonEmpty(s.EnablePass, password)

	session, err := collector.DialSSH(ctx, collector.SSHConfig{
		Host: device.ManagementAddress, Port: device.ManagementPort, User: username,
		Password: password, CLIUser: username, CLIPassword: cliPass, EnablePassword: enablePass,
		Timeout: 20 * time.Second, CommandTimeout: commandTimeout, KnownHosts: s.KnownHosts,
	})
	if err != nil {
		result, reason = "failed", "connect: "+err.Error()
		publish("error", reason)
		return
	}
	defer session.Close()

	// Streaming starts only now: the login sequence is never part of the stream.
	// Output is batched (64 kB / 60 ms) before it reaches the hub; the deferred
	// Close runs before the run is finished, so the tail is never lost.
	coalescer := collector.NewCoalescer(func(kind, data string) { publish(kind, data) }, 64<<10, 60*time.Millisecond)
	defer coalescer.Close()

	observer := collector.Tee(collector.NewRedactor(password, cliPass, enablePass), coalescer.Handle)
	session.SetObserver(observer)
	publish("info", "logged in to "+device.Name)

	adapter, _, _ := AdapterFor(device.Vendor)
	if adapter.RequiresConfigMode() {
		if _, err := session.Run(ctx, "configure terminal"); err != nil {
			result, reason = "failed", "config mode: "+err.Error()
			publish("error", reason)
			return
		}
		publish("info", "configuration mode")
	}

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
	for _, res := range results {
		job := collector.Job{ResultID: res.CommandID, Line: res.Line, Raw: res.Raw, Err: res.Err}
		switch rows := res.Parsed.(type) {
		case []vsol.OnuRow:
			job.Rows = vsol.ToStatusRows(rows)
		case []vsol.OnuIdentity:
			job.Rows = vsol.ToIdentityRows(rows)
		case []vsol.OpticalRow:
			job.Rows = vsol.ToOpticalRows(rows)
		}
		if res.Err != nil {
			result = "partial"
			reason = res.CommandID + ": " + res.Err.Error()
			publish("error", reason)
		}
		jobs = append(jobs, job)
	}

	if err := ing.Append(ctx, runID, device.ID, started, jobs); err != nil {
		result, reason = "failed", "store: "+err.Error()
		publish("error", reason)
		return
	}
	publish("info", "stored — "+result)
}

func (s *Service) log(msg string, err error) {
	// The service has no logger by design; the API logs the run outcome from
	// the row it can read back.
	_ = msg
	_ = err
}
