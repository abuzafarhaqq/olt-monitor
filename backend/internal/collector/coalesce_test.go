package collector_test

import (
	"strings"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
)

func TestCoalescerBatchesSmallWrites(t *testing.T) {
	var events []string
	c := collector.NewCoalescer(func(kind, data string) {
		events = append(events, kind+":"+data)
	}, 1<<20, 15*time.Millisecond)

	for _, chunk := range []string{"a", "b", "c", "d"} {
		c.Out(chunk)
	}
	if len(events) != 0 {
		t.Fatalf("published before the interval: %v", events)
	}

	time.Sleep(60 * time.Millisecond)
	if len(events) != 1 || events[0] != "out:abcd" {
		t.Fatalf("events = %v, want one batched write", events)
	}
	c.Close()
}

func TestCoalescerFlushesOnSize(t *testing.T) {
	var events []string
	c := collector.NewCoalescer(func(kind, data string) {
		events = append(events, data)
	}, 8, time.Hour)

	c.Out("12345")
	c.Out("6789") // crosses 8 bytes
	if len(events) != 1 || events[0] != "123456789" {
		t.Fatalf("events = %v", events)
	}
	c.Close()
}

// A command line must never be published before the output that preceded it.
func TestCoalescerFlushesBeforeNonOutput(t *testing.T) {
	var order []string
	c := collector.NewCoalescer(func(kind, data string) {
		order = append(order, kind+":"+data)
	}, 1<<20, time.Hour)

	c.Handle("out", "table row")
	c.Handle("cmd", "show version")

	if len(order) != 2 || !strings.HasPrefix(order[0], "out:") || !strings.HasPrefix(order[1], "cmd:") {
		t.Fatalf("order = %v", order)
	}
	c.Close()
}

func TestCoalescerCloseFlushes(t *testing.T) {
	var events []string
	c := collector.NewCoalescer(func(kind, data string) {
		events = append(events, data)
	}, 1<<20, time.Hour)

	c.Out("tail")
	c.Close()
	if len(events) != 1 || events[0] != "tail" {
		t.Fatalf("Close lost the buffered output: %v", events)
	}
}
