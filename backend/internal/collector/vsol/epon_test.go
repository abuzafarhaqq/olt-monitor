package vsol_test

import (
	"strings"
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

// Real rows from `show onu opm-diag all`. No MACs appear in this output, so the
// values are kept as captured.
const opticalSample = `ONU-ID      Temperature(C)    Supply Voltage(V)   TX Bias Current(mA)   TX Power(dBm)   RX Power(dBm)
------      --------------    -----------------   -------------------   -------------   -------------
EPON0/1:1   58.88             3.22                12.69                 3.02            -16.60
EPON0/1:9   35.24             3.33                18.05                 1.73            -28.86
EPON0/3:64  40.39             3.41                16.85                 1.78            -25.09
`

func TestParseOpticalFromRealOutput(t *testing.T) {
	command, ok := vsol.New().Command("opm-diag")
	if !ok || command.Parse == nil {
		t.Fatal("opm-diag has no parser")
	}

	parsed, err := command.Parse(opticalSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := parsed.([]vsol.OpticalRow)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (header and separator must not match)", len(rows))
	}

	first := rows[0]
	if first.Pon != "0/1" || first.OnuID != 1 {
		t.Fatalf("identity = %s:%d", first.Pon, first.OnuID)
	}
	// This device reports the full optical set for every ONU, not just RX.
	if first.TemperatureC != 58.88 || first.SupplyVoltageV != 3.22 ||
		first.TxBiasMa != 12.69 || first.TxDbm != 3.02 || first.RxDbm != -16.60 {
		t.Fatalf("values = %+v", first)
	}

	if rows[1].RxDbm != -28.86 {
		t.Fatalf("critical reading = %v", rows[1].RxDbm)
	}
	if rows[2].OnuID != 64 {
		t.Fatalf("two-digit onu id = %d", rows[2].OnuID)
	}
}

func TestOpticalRejectsGarbage(t *testing.T) {
	command, _ := vsol.New().Command("opm-diag")
	if _, err := command.Parse("% unknown command"); err == nil {
		t.Fatal("garbage parsed as optical rows")
	}
}

// The whole `show onu …` family is rejected in privileged mode: the device
// answers "% Unknown command" there. The adapter has to say so, or the
// collector runs every command one mode too low.
func TestAdapterRequiresConfigMode(t *testing.T) {
	if !vsol.New().RequiresConfigMode() {
		t.Fatal("vsol-epon must declare config mode; its show commands live there")
	}
}

// From a live `show onu basic-info all`. The ID column (ONU serial) is replaced
// with zeros here and is never parsed by the adapter.
const basicInfoSample = `ONU-ID      VendorID  Model     ID            hwVer     SwVer               Type                Interface Type
------      --------  -----     --            -----     --------            ---------           ---------
EPON0/1:1   MONU      D301      000000000000  V8.0      V6.0.1P1T2          SFU                 1GE
EPON0/1:5   HWTC      XPON      000000000000  V9.0      V9.0.51P1T11        SFU                 1GE
EPON0/2:9   ZTE       112E      000000000000  V1.0      V1.0.0              HGU                 1GE+CATV
`

func TestParseBasicInfo(t *testing.T) {
	command, ok := vsol.New().Command("onu-basic-info")
	if !ok || command.Parse == nil {
		t.Fatal("onu-basic-info is not allow-listed with a parser")
	}

	parsed, err := command.Parse(basicInfoSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := parsed.([]vsol.OnuIdentity)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}

	first := rows[0]
	if first.Pon != "0/1" || first.OnuID != 1 || first.Vendor != "MONU" || first.Model != "D301" {
		t.Fatalf("identity = %+v", first)
	}
	if first.Type != "SFU" || first.InterfaceType != "1GE" {
		t.Fatalf("type/interface = %+v", first)
	}
	if rows[2].InterfaceType != "1GE+CATV" {
		t.Fatalf("interface with a plus = %q", rows[2].InterfaceType)
	}
}

// The serial column must never survive parsing: it is a device identifier.
func TestBasicInfoDoesNotKeepTheSerial(t *testing.T) {
	command, _ := vsol.New().Command("onu-basic-info")
	parsed, _ := command.Parse(basicInfoSample)
	for _, row := range parsed.([]vsol.OnuIdentity) {
		if strings.Contains(row.Vendor+row.Model+row.HardwareVer, "000000000000") {
			t.Fatal("the serial column leaked into a parsed field")
		}
	}
}
