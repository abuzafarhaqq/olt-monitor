package collector_test

import (
	"testing"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
)

func TestRedactorMasksLongestFirst(t *testing.T) {
	r := collector.NewRedactor("abc123", "abc12345")

	if got := r.Redact("password is abc12345 now"); got != "password is ******** now" {
		t.Fatalf("longest secret not masked first: %q", got)
	}
	if got := r.Redact("short abc123 here"); got != "short ******** here" {
		t.Fatalf("short secret not masked: %q", got)
	}
}

func TestRedactorIgnoresShortSecrets(t *testing.T) {
	r := collector.NewRedactor("abc", "")
	// Three characters would mask ordinary words; it is left alone.
	if got := r.Redact("abc def"); got != "abc def" {
		t.Fatalf("short secret was masked: %q", got)
	}
}

func TestTeeRedactsBeforePublishing(t *testing.T) {
	var seen []string
	tee := collector.Tee(collector.NewRedactor("hunter2x"), func(kind, data string) {
		seen = append(seen, kind+":"+data)
	})

	tee("out", "login as admin with hunter2x ok")
	if len(seen) != 1 || seen[0] != "out:login as admin with ******** ok" {
		t.Fatalf("published %v", seen)
	}
}
