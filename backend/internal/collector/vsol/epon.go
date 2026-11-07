// Package vsol implements the VSOL EPON adapter.
//
// The command list and the parsing shapes come from the device discovery notes
// (model EPON-OLT, software V2.03.75R). They are a starting point, not a
// guarantee: the real column layout has not been captured yet, so the parsers
// are tolerant and report a parse error instead of guessing. Anything they
// cannot read is an error, never a zero.
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
	{ID: "opm-diag", Line: "show onu opm-diag all", Parse: parseOptical},
}

// --- version -------------------------------------------------------------

type Version struct {
	Hardware string
	Software string
}

var (
	reHardware = regexp.MustCompile(`(?i)hardware\s+version\s*[:：]\s*(\S+)`)
	reSoftware = regexp.MustCompile(`(?i)software\s+version\s*[:：]\s*(\S+)`)
)

func parseVersion(output string) (any, error) {
	version := Version{}
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
	DeviceTime string
	Trusted    bool
}

var reTime = regexp.MustCompile(`(?i)(?:current\s+time|^time)\s*[:：]\s*(.+)$`)

// parseTime flags an obviously wrong clock. A device reporting 2000 makes every
// timestamp it emits unusable, and the platform must know that.
func parseTime(output string) (any, error) {
	clock := Clock{}
	for _, line := range strings.Split(output, "\n") {
		if m := reTime.FindStringSubmatch(strings.TrimSpace(line)); m != nil && clock.DeviceTime == "" {
			clock.DeviceTime = strings.TrimSpace(m[1])
		}
	}
	if clock.DeviceTime == "" {
		return nil, fmt.Errorf("no clock line matched")
	}
	clock.Trusted = !strings.Contains(clock.DeviceTime, "2000-")
	return clock, nil
}

// --- ONU status ----------------------------------------------------------

type OnuRow struct {
	Pon      string
	OnuID    int
	MAC      string
	Status   string
	Distance *int
}

var (
	reMAC = regexp.MustCompile(`(?i)^[0-9a-f]{2}([:-][0-9a-f]{2}){5}$`)
	rePon = regexp.MustCompile(`^\d+/\d+$`)
	reOnu = regexp.MustCompile(`^\d+$`)
)

var statusWords = map[string]string{
	"online": "online", "up": "online", "active": "online",
	"offline": "offline", "down": "offline", "los": "offline",
}

// parseOnuStatus reads the status table line by line: a PON (0/1), an ONU
// number, a MAC, a status word and optionally a distance. Rows without a MAC or
// a status are skipped, so a changed layout shows up as "no rows recognised"
// instead of an empty estate.
func parseOnuStatus(output string) (any, error) {
	rows := make([]OnuRow, 0, 64)

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}

		var row OnuRow
		var haveStatus bool

		for _, field := range fields {
			lower := strings.ToLower(field)
			switch {
			case rePon.MatchString(field) && row.Pon == "":
				row.Pon = field
			case reMAC.MatchString(field) && row.MAC == "":
				row.MAC = strings.ToUpper(field)
			case statusWords[lower] != "" && !haveStatus:
				row.Status = statusWords[lower]
				haveStatus = true
			case row.Pon != "" && row.OnuID == 0 && reOnu.MatchString(field):
				if n, err := strconv.Atoi(field); err == nil {
					row.OnuID = n
				}
			case haveStatus && reOnu.MatchString(field) && row.Distance == nil:
				if n, err := strconv.Atoi(field); err == nil {
					row.Distance = &n
				}
			}
		}

		if row.Pon == "" || row.MAC == "" || !haveStatus {
			continue
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no ONU rows recognised — the column layout may have changed")
	}
	return rows, nil
}

// --- optical -------------------------------------------------------------

type OpticalRow struct {
	Pon   string
	OnuID int
	RxDbm *float64
	TxDbm *float64
	Raw   string
}

var reNumber = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// parseOptical keeps a missing reading as nil. "--" is not zero: turning it into
// one would report "no measurement" as "dead optics".
func parseOptical(output string) (any, error) {
	rows := make([]OpticalRow, 0, 64)

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		var row OpticalRow
		var numbers []float64

		for _, field := range fields {
			if row.Pon == "" && rePon.MatchString(field) {
				row.Pon = field
				continue
			}
			if row.Pon != "" && row.OnuID == 0 && reOnu.MatchString(field) {
				if n, err := strconv.Atoi(field); err == nil {
					row.OnuID = n
				}
				continue
			}
			if reNumber.MatchString(field) {
				if value, err := strconv.ParseFloat(field, 64); err == nil {
					numbers = append(numbers, value)
				}
			}
		}

		if row.Pon == "" || row.OnuID == 0 {
			continue
		}
		row.Raw = line
		if len(numbers) >= 2 {
			rx, tx := numbers[0], numbers[1]
			row.RxDbm, row.TxDbm = &rx, &tx
		} else if len(numbers) == 1 {
			rx := numbers[0]
			row.RxDbm = &rx
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no optical rows recognised")
	}
	return rows, nil
}
