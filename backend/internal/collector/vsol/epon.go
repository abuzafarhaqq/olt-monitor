// Package vsol implements the VSOL EPON adapter.
//
// The command list and every parser below are written against a real session
// capture from the device (model EPON-OLT, software V2.03.75R_231213152618).
// Where the output has not been captured yet — optical diagnostics — there is a
// command in the allow-list and no parser: raw evidence only, never a guess.
package vsol

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
)

type Adapter struct{}

func New() Adapter { return Adapter{} }

func (Adapter) Name() string { return "vsol-epon" }

func (a Adapter) Commands() []collector.Command { return commands }

func (a Adapter) Command(id string) (collector.Command, bool) {
	for _, command := range commands {
		if command.ID == id {
			return command, true
		}
	}
	return collector.Command{}, false
}

// The allow-list. Nothing outside it can be executed — not by the CLI, not by a
// future API, not by a typo.
var commands = []collector.Command{
	{ID: "version", Line: "show version", Parse: parseVersion},
	{ID: "time", Line: "show time", Parse: parseTime},
	{ID: "onu-status", Line: "show onu status all", Parse: parseOnuStatus},
	// No parser yet: the output format has not been captured from a real
	// device. The command stays allow-listed so the raw text can be collected as
	// evidence, and the parser gets written against that text rather than
	// guessed.
	{ID: "opm-diag", Line: "show onu opm-diag all"},
}

// --- version -------------------------------------------------------------

type Version struct {
	Model    string
	Hardware string
	Software string
}

// The serial number is printed by this command and is deliberately not parsed:
// it is a device identifier that must not end up in a database row, a log line
// or an example.
var (
	reModel    = regexp.MustCompile(`(?i)olt device model\s*:\s*(\S+)`)
	reHardware = regexp.MustCompile(`(?i)hardware version\s*:\s*(\S+)`)
	reSoftware = regexp.MustCompile(`(?i)software version\s*:\s*(\S+)`)
)

func parseVersion(output string) (any, error) {
	version := Version{}
	if m := reModel.FindStringSubmatch(output); m != nil {
		version.Model = m[1]
	}
	if m := reHardware.FindStringSubmatch(output); m != nil {
		version.Hardware = m[1]
	}
	if m := reSoftware.FindStringSubmatch(output); m != nil {
		version.Software = m[1]
	}
	if version.Hardware == "" && version.Software == "" {
		return nil, fmt.Errorf("no version fields matched")
	}
	return version, nil
}

// --- clock ---------------------------------------------------------------

type Clock struct {
	// Kept exactly as printed ("Sun Jun  4 14:50:12 2000"): the format is the
	// device's, and parsing it into a time.Time would quietly "fix" a clock that
	// is 26 years wrong.
	DeviceTime string
	Uptime     string
	Trusted    bool
}

var (
	reClock  = regexp.MustCompile(`(?i)current date/time is\s*:\s*(.+)$`)
	reUptime = regexp.MustCompile(`(?i)running for\s+(.+)$`)
)

// parseTime flags an obviously wrong clock. This device reports June 2000 while
// its own uptime says 34 days, so every timestamp it prints is unusable and
// anything derived from it must be marked untrusted rather than stored as fact.
func parseTime(output string) (any, error) {
	clock := Clock{}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := reClock.FindStringSubmatch(trimmed); m != nil && clock.DeviceTime == "" {
			clock.DeviceTime = strings.TrimSpace(m[1])
		}
		if m := reUptime.FindStringSubmatch(trimmed); m != nil && clock.Uptime == "" {
			clock.Uptime = strings.TrimSpace(m[1])
		}
	}
	if clock.DeviceTime == "" {
		return nil, fmt.Errorf("no clock line matched")
	}
	clock.Trusted = !strings.Contains(clock.DeviceTime, " 2000") && !strings.Contains(clock.DeviceTime, " 199")
	return clock, nil
}

// --- ONU status ----------------------------------------------------------

type OnuRow struct {
	// Pon is the port as printed, split out of "EPON0/1:1".
	Pon      string
	OnuID    int
	MAC      string
	Status   string
	Distance *int
	RTT      *int
	// Device clock strings, kept as printed; this device's clock is in 2000.
	LastRegTime   string
	LastDeregTime string
}

// One row of `show onu status all`:
//
//	EPON0/1:1   online   80:14:a8:4f:e0:a0   2469   1602   2000/05/30 11:24:09   2000/05/30 11:12:11   Power Off   4 03:25:58   N/A
//
// The trailing columns (deregistration reason, alive time, upgrade) are not
// parsed yet: nothing consumes them, and guessing at variable-width fields is
// how a parser breaks without anyone noticing.
var reOnuRow = regexp.MustCompile(
	`^(?:EPON)?(\d+/\d+):(\d+)\s+(\S+)\s+([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5})\s+(\d+)\s+(\d+)` +
		`(?:\s+(N/A|\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}))?` +
		`(?:\s+(N/A|\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}))?`)

var statusWords = map[string]string{
	"online": "online", "up": "online", "active": "online",
	"offline": "offline", "down": "offline", "los": "offline",
}

func parseOnuStatus(output string) (any, error) {
	rows := make([]OnuRow, 0, 64)

	for _, line := range strings.Split(output, "\n") {
		match := reOnuRow.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}

		status := statusWords[strings.ToLower(match[3])]
		if status == "" {
			continue
		}

		row := OnuRow{
			Pon:    match[1],
			OnuID:  atoi(match[2]),
			MAC:    strings.ToUpper(match[4]),
			Status: status,
		}

		// A dead ONU reports distance 0. Zero metres is not a measurement, so it
		// stays nil rather than becoming a suspiciously close ONU.
		if distance := atoi(match[5]); distance > 0 {
			row.Distance = &distance
		}
		if rtt := atoi(match[6]); rtt > 0 {
			row.RTT = &rtt
		}
		if match[7] != "" && match[7] != "N/A" {
			row.LastRegTime = match[7]
		}
		if match[8] != "" && match[8] != "N/A" {
			row.LastDeregTime = match[8]
		}

		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no ONU rows recognised — the column layout may have changed")
	}
	return rows, nil
}

func atoi(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
