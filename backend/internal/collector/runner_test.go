package collector_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector/vsol"
)

type fakeSession struct {
	outputs map[string]string
	fail    string
	seen    []string
}

func (f *fakeSession) Run(_ context.Context, line string) (string, error) {
	f.seen = append(f.seen, line)
	if line == f.fail {
		return "", errors.New("device closed the connection")
	}
	return f.outputs[line], nil
}

func (f *fakeSession) Close() error { return nil }

// A command id outside the allow-list aborts the batch instead of being skipped:
// a silent skip reads as "nothing to report".
func TestRunnerRefusesUnknownCommand(t *testing.T) {
	session := &fakeSession{outputs: map[string]string{}}
	runner := collector.Runner{Adapter: vsol.New(), Session: session}

	if _, err := runner.Run(context.Background(), []string{"version", "shell"}); err == nil {
		t.Fatal("an allow-list miss was accepted")
	}
	if len(session.seen) != 1 {
		t.Fatalf("the batch did not stop at the unknown command: %v", session.seen)
	}
}

func TestRunnerParsesAndKeepsRawEvidence(t *testing.T) {
	session := &fakeSession{outputs: map[string]string{
		"show version": "Hardware Version : V1.1.4\nSoftware Version : V2.03.75R\n",
	}}
	results, err := (collector.Runner{Adapter: vsol.New(), Session: session}).
		Run(context.Background(), []string{"version"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d", len(results))
	}
	if !strings.Contains(results[0].Raw, "V1.1.4") {
		t.Fatal("raw output was not preserved")
	}
	if results[0].Parsed == nil {
		t.Fatal("nothing was parsed")
	}
}

// A command that fails must not be reported as if the device answered.
func TestRunnerSurfacesSessionError(t *testing.T) {
	session := &fakeSession{outputs: map[string]string{}, fail: "show time"}
	results, err := (collector.Runner{Adapter: vsol.New(), Session: session}).
		Run(context.Background(), []string{"time"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if results[0].Err == nil {
		t.Fatal("a failed command came back as a success")
	}
	if results[0].Parsed != nil {
		t.Fatal("a failed command produced parsed data")
	}
}

// The device pages long tables with "--More--". A reader that waits for the
// prompt without advancing the pager returns one truncated page and then times
// out — exactly what the first live run did.
func TestStripPagerRemovesMarkers(t *testing.T) {
	// stripPager is unexported; the behaviour is covered through the session
	// tests in ssh_test.go, so here we only assert the contract that parsing a
	// paged transcript still works end to end.
	session := &fakeSession{outputs: map[string]string{
		"show onu status all": "ONU-ID Status MAC Address Distance(m) RTT(TQ)\n" +
			"EPON0/1:1 online 02:00:00:00:00:01 246 160\n --More-- \n" +
			"EPON0/1:2 offline 02:00:00:00:00:02 0 0\n",
	}}
	results, err := (collector.Runner{Adapter: vsol.New(), Session: session}).
		Run(context.Background(), []string{"onu-status"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if results[0].Parsed == nil {
		t.Fatal("paged output did not parse")
	}
}
