// Package collecting runs one collection against one device, on behalf of the
// API. It exists because "refresh this OLT" has to be a server action: the
// browser never speaks SSH, and the credential only lives in the secret store.
package collecting

import (
	"context"
	"fmt"
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
	Timeout  time.Duration
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
func (s Service) BuildPlan(device Device) (Plan, error) {
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
func (s Service) Run(ctx context.Context, device Device) (Plan, int64, []CommandResult, error) {
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
