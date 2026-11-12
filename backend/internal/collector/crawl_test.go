package collector_test

import (
	"context"
	"strings"
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
)

// Real `show onu ?` output from the VSOL EPON.
const onuHelp = `show onu ?
  auth-info   Specify authenticated onu information.
  basic-info  Specify onu basic information.
  command     OLT send to onu command.
  discover    Specify discovered onu.
  opm-diag    Specify 0pticalTransceiverDiagnosis.
  pool        OLT send to onu pool.
  register    Specify onu register.
  special     Specify special for onu.
  statistics  Specify onu statistics information.
  status      Specify onu status information.
  timer       OLT send to onu command.
  <cr>        Just Press Enter to Execute command!
`

func TestParseHelp(t *testing.T) {
	entries := collector.ParseHelp(onuHelp)
	if len(entries) != 12 {
		t.Fatalf("entries = %d, want 12: %+v", len(entries), entries)
	}
	if entries[0].Token != "auth-info" || !strings.Contains(entries[0].Help, "authenticated") {
		t.Fatalf("first entry = %+v", entries[0])
	}
}

func TestClassifyNeverCallsAnActionRead(t *testing.T) {
	cases := map[string]collector.CommandKind{
		"show":             collector.KindRead,
		"status":           collector.KindUnknown, // a leaf named status is not a verb
		"command":          collector.KindAction,
		"register":         collector.KindAction,
		"reboot":           collector.KindAction,
		"pool":             collector.KindAction,
		"<cr>":             collector.KindParam,
		"<HHHH:HHHH:HHHH>": collector.KindParam,
	}
	for token, want := range cases {
		if got := collector.Classify(token, "desc"); got != want {
			t.Errorf("%q classified %q, want %q", token, got, want)
		}
	}
}

func TestCrawlStopsAtBudgetAndNeverExecutes(t *testing.T) {
	var probes []string
	crawler := &collector.Crawler{
		MaxDepth: 2,
		MaxNodes: 20,
		ProbeHelp: func(_ context.Context, path string) (string, error) {
			probes = append(probes, path)
			return onuHelp, nil // pretend every node has the same children
		},
	}

	nodes, err := crawler.Crawl(context.Background())
	if err == nil {
		t.Fatal("expected the node budget to stop the crawl")
	}
	if len(nodes) > 20 {
		t.Fatalf("crawl returned %d nodes, budget was 20", len(nodes))
	}

	for _, probe := range probes {
		if !strings.HasSuffix(probe, "?") {
			t.Fatalf("probe %q does not end in '?' — it would run a command", probe)
		}
		if strings.Contains(probe, "\r") || strings.Contains(probe, "\n") {
			t.Fatalf("probe %q contains a line ending", probe)
		}
	}

	// Actions are recorded but never descended into.
	for _, probe := range probes {
		for _, verb := range []string{"command", "register", "reboot", "pool", "timer"} {
			if strings.Contains(probe, " "+verb+" ") || strings.HasSuffix(probe, " "+verb+" ?") {
				t.Fatalf("crawl descended into an action verb: %q", probe)
			}
		}
	}
}
