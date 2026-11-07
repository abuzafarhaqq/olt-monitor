package vsol_test

import (
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector/vsol"
)

// Fixtures are copied from a real `ssh admin@<olt>` session.
//
// MAC addresses are replaced with locally-administered values (02:…) and the
// OLT serial number is removed: this repository is public, and neither belongs
// in an example. Everything else — column widths, N/A markers, the 2000 clock —
// is exactly as the device printed it.

const versionSample = `  Olt Serial Number:           REDACTED
  Olt Device Model:            EPON-OLT
  Hardware Version:            V1.1.4
  Software Version:            V2.03.75R_231213152618
  Software Created Time:       Wed, 13 Dec 2023 15:26:18 +0800
  Copyright 2016-2023
`

func TestParseVersion(t *testing.T) {
	command, ok := vsol.New().Command("version")
	if !ok {
		t.Fatal("version is not allow-listed")
	}

	parsed, err := command.Parse(versionSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	version := parsed.(vsol.Version)
	if version.Model != "EPON-OLT" {
		t.Fatalf("model = %q", version.Model)
	}
	if version.Hardware != "V1.1.4" {
		t.Fatalf("hardware = %q", version.Hardware)
	}
	if version.Software != "V2.03.75R_231213152618" {
		t.Fatalf("software = %q", version.Software)
	}
}

// The real device says "The current date/time is : Sun Jun 4 14:50:12 2000"
// while its uptime says 34 days. The parser must read both and refuse to call
// that clock trusted.
const timeSample = `The current date/time is : Sun Jun  4 14:50:12 2000

The olt has been running for 34 days 8 hours 49 minutes 30 seconds
`

func TestClockIsReadAndFlaggedUntrusted(t *testing.T) {
	command, _ := vsol.New().Command("time")
	parsed, err := command.Parse(timeSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	clock := parsed.(vsol.Clock)
	if clock.DeviceTime != "Sun Jun  4 14:50:12 2000" {
		t.Fatalf("device time = %q", clock.DeviceTime)
	}
	if clock.Uptime != "34 days 8 hours 49 minutes 30 seconds" {
		t.Fatalf("uptime = %q", clock.Uptime)
	}
	if clock.Trusted {
		t.Fatal("a year-2000 clock was marked trusted")
	}
}

const statusSample = `ONU-ID      Status    MAC  Address         Distance(m)  RTT(TQ) LastRegTime             LastDeregTime           LastDeregReason    AliveTime    Upgrade
------      ------    ------------         -----------  ------  -----------             -------------           ---------------   ------------ -------
EPON0/1:1   online    02:00:00:00:00:01    2469         1602    2000/05/30 11:24:09     2000/05/30 11:12:11     Power Off         4 03:25:58   N/A
EPON0/1:2   offline   02:00:00:00:00:02    0            0       N/A                     N/A                     N/A               00:00:00     N/A
EPON0/1:4   offline   02:00:00:00:00:04    542          427     2000/06/03 06:43:08     2000/06/03 09:48:16     Wire Down         03:05:07     N/A
EPON0/1:16  online    02:00:00:00:00:16    569          443     2000/04/30 06:01:34     N/A                     N/A               34 08:48:33  N/A
`

func TestParseOnuStatusFromRealOutput(t *testing.T) {
	command, _ := vsol.New().Command("onu-status")
	parsed, err := command.Parse(statusSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rows := parsed.([]vsol.OnuRow)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 (header and separator must not match)", len(rows))
	}

	first := rows[0]
	if first.Pon != "0/1" || first.OnuID != 1 || first.Status != "online" {
		t.Fatalf("first row = %+v", first)
	}
	if first.MAC != "02:00:00:00:00:01" {
		t.Fatalf("mac = %q", first.MAC)
	}
	if first.Distance == nil || *first.Distance != 2469 {
		t.Fatalf("distance = %v", first.Distance)
	}
	if first.LastRegTime != "2000/05/30 11:24:09" {
		t.Fatalf("last register = %q", first.LastRegTime)
	}

	// A dead ONU reports distance 0. Zero metres is not a measurement.
	if rows[1].Distance != nil {
		t.Fatalf("distance 0 became %d", *rows[1].Distance)
	}
	if rows[1].LastRegTime != "" {
		t.Fatalf("N/A became %q", rows[1].LastRegTime)
	}

	// Wire Down is still an offline ONU with a real distance.
	if rows[2].Status != "offline" || rows[2].Distance == nil || *rows[2].Distance != 542 {
		t.Fatalf("third row = %+v", rows[2])
	}

	// Two-digit ONU ids and a missing deregistration must both survive.
	if rows[3].OnuID != 16 || rows[3].LastDeregTime != "" {
		t.Fatalf("fourth row = %+v", rows[3])
	}
}

func TestUnrecognisedOutputIsAnError(t *testing.T) {
	command, _ := vsol.New().Command("onu-status")
	if _, err := command.Parse("% Error: permission denied"); err == nil {
		t.Fatal("garbage parsed as if it were a table")
	}
}

// opm-diag is allow-listed but has no parser until its output is captured: raw
// evidence only, never a guess.
func TestOpticalCommandIsRawUntilCaptured(t *testing.T) {
	command, ok := vsol.New().Command("opm-diag")
	if !ok {
		t.Fatal("opm-diag should stay allow-listed")
	}
	if command.Parse != nil {
		t.Fatal("opm-diag has a parser that was written without a real sample")
	}
}

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
