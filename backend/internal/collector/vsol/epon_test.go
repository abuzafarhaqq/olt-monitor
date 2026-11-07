package vsol_test

import (
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector/vsol"
)

// The samples below follow the shape described in the device discovery notes.
// They are NOT captures from the real OLT: the parsers must be re-run against a
// real `show onu status all` before anyone trusts the numbers.

const versionSample = `        VSOL EPON OLT
Hardware Version : V1.1.4
Software Version : V2.03.75R_231213152618
Build Date       : 2023-12-13
`

func TestParseVersion(t *testing.T) {
	adapter := vsol.New()
	command, ok := adapter.Command("version")
	if !ok {
		t.Fatal("version is not allow-listed")
	}

	parsed, err := command.Parse(versionSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	version := parsed.(vsol.Version)
	if version.Hardware != "V1.1.4" {
		t.Fatalf("hardware = %q", version.Hardware)
	}
	if version.Software != "V2.03.75R_231213152618" {
		t.Fatalf("software = %q", version.Software)
	}
}

const timeSample = `Current time : 2000-05-20 04:12:33
Uptime       : 41 days, 0 hours, 12 minutes
`

// A clock from 2000 is unusable, and the parser has to say so rather than pass
// it along as if it were a real timestamp.
func TestClockIsFlaggedUntrusted(t *testing.T) {
	command, _ := vsol.New().Command("time")
	parsed, err := command.Parse(timeSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.(vsol.Clock).Trusted {
		t.Fatal("a year-2000 clock was marked trusted")
	}
}

const statusSample = `PON   ONU  MAC                Status   Distance(m)
0/1   1    02:1A:2B:00:00:01  online   420
0/1   2    02:1A:2B:00:00:02  online   1180
0/2   6    02:1A:2B:00:00:06  offline  --
0/2   7    02:1A:2B:00:00:07  LOS      --
`

func TestParseOnuStatus(t *testing.T) {
	command, _ := vsol.New().Command("onu-status")
	parsed, err := command.Parse(statusSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rows := parsed.([]vsol.OnuRow)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if rows[0].Pon != "0/1" || rows[0].OnuID != 1 || rows[0].Status != "online" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[0].Distance == nil || *rows[0].Distance != 420 {
		t.Fatalf("distance = %v", rows[0].Distance)
	}
	if rows[2].Status != "offline" {
		t.Fatalf("third row status = %q", rows[2].Status)
	}
	if rows[2].Distance != nil {
		t.Fatalf("an unknown distance became %d", *rows[2].Distance)
	}
	// LOS maps to offline: the ONU is unreachable, whatever the vendor calls it.
	if rows[3].Status != "offline" {
		t.Fatalf("LOS mapped to %q", rows[3].Status)
	}
}

const opticalSample = `PON   ONU  RX(dBm)  TX(dBm)
0/1   1    -18.40   2.41
0/1   2    -31.40   2.38
0/2   6    --       --
`

func TestParseOpticalKeepsMissingAsNil(t *testing.T) {
	command, _ := vsol.New().Command("opm-diag")
	parsed, err := command.Parse(opticalSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rows := parsed.([]vsol.OpticalRow)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[1].RxDbm == nil || *rows[1].RxDbm != -31.4 {
		t.Fatalf("critical reading = %v", rows[1].RxDbm)
	}
	if rows[2].RxDbm != nil {
		t.Fatalf(`"--" became %v instead of nil`, *rows[2].RxDbm)
	}
}

func TestUnrecognisedOutputIsAnError(t *testing.T) {
	command, _ := vsol.New().Command("onu-status")
	if _, err := command.Parse("error: permission denied"); err == nil {
		t.Fatal("garbage parsed as if it were a table")
	}
}

// The allow-list is the security boundary: an unknown id must not resolve.
func TestOnlyAllowListedCommandsResolve(t *testing.T) {
	adapter := vsol.New()
	if _, ok := adapter.Command("shell"); ok {
		t.Fatal("shell resolved as a command")
	}
	for _, command := range adapter.Commands() {
		if command.Line == "" {
			t.Fatalf("%s has no command line", command.ID)
		}
	}
}
