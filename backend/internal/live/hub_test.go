package live_test

import (
	"strings"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/live"
)

func TestRingBufferServesLateSubscribers(t *testing.T) {
	hub := live.NewHub()
	hub.Open(7)

	for i := 0; i < 5; i++ {
		hub.Publish(7, "out", "line")
	}

	history := hub.History(7, 0)
	if len(history) != 5 {
		t.Fatalf("history = %d, want 5", len(history))
	}
	if history[0].Seq != 1 || history[4].Seq != 5 {
		t.Fatalf("sequence not monotonic: %d..%d", history[0].Seq, history[4].Seq)
	}

	// Resume: everything after seq 2.
	resumed := hub.History(7, 2)
	if len(resumed) != 3 || resumed[0].Seq != 3 {
		t.Fatalf("resume = %d events starting at %d", len(resumed), resumed[0].Seq)
	}
}

func TestRingIsBounded(t *testing.T) {
	hub := live.NewHub()
	hub.Open(1)
	for i := 0; i < live.RingEvents+500; i++ {
		hub.Publish(1, "out", "x")
	}
	if got := len(hub.History(1, 0)); got != live.RingEvents {
		t.Fatalf("history = %d, want the ring size %d", got, live.RingEvents)
	}
}

func TestSubscriberReceivesAndCloses(t *testing.T) {
	hub := live.NewHub()
	hub.Open(2)

	events, cancel := hub.Subscribe(2)
	defer cancel()

	hub.Publish(2, "cmd", "show version")

	select {
	case ev := <-events:
		if ev.Kind != "cmd" || ev.Data != "show version" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no event delivered")
	}

	hub.Close(2, "end", "done")

	select {
	case _, ok := <-events:
		if ok {
			// The closing event may still be buffered; drain it.
			_, ok2 := <-events
			if ok2 {
				t.Fatal("channel still open after close")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed")
	}
}

// A subscriber that stops reading must not block the publisher.
func TestSlowSubscriberIsDroppedNotBlocking(t *testing.T) {
	hub := live.NewHub()
	hub.Open(3)
	_, cancel := hub.Subscribe(3) // never read
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < live.SubscriberBuffer*3; i++ {
			hub.Publish(3, "out", "line")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked on a slow subscriber")
	}

	if hub.Dropped(3) == 0 {
		t.Fatal("expected dropped events for a non-reading subscriber")
	}
}

// The ring is bounded in bytes, not events: a run that prints megabytes must not
// pin them in memory, and the tail is what a terminal needs.
func TestRingIsByteBounded(t *testing.T) {
	hub := live.NewHub()
	hub.Open(9)

	chunk := strings.Repeat("x", 64<<10) // 64 kB per event
	for i := 0; i < 100; i++ {           // 6.4 MB total, over the 4 MB cap
		hub.Publish(9, "out", chunk)
	}

	history := hub.History(9, 0)
	bytes := 0
	for _, ev := range history {
		bytes += len(ev.Data)
	}
	if bytes > live.RingBytes+(64<<10) {
		t.Fatalf("ring holds %d bytes, cap is %d", bytes, live.RingBytes)
	}
	// The tail survives; the start does not.
	last := history[len(history)-1]
	if last.Seq < 100 {
		t.Fatalf("newest event is %d, want 100", last.Seq)
	}
	if first, _ := hub.FirstSeq(9); first <= 1 {
		t.Fatalf("first buffered seq = %d, expected the beginning to be evicted", first)
	}
}

func TestFinishedRunsAreEvicted(t *testing.T) {
	hub := live.NewHubWithRetention(30*time.Millisecond, 2)

	for id := int64(1); id <= 5; id++ {
		hub.Open(id)
		hub.Publish(id, "out", "x")
		hub.Close(id, "end", "done")
	}

	// Count cap: only the two most recent finished runs survive.
	alive := 0
	for id := int64(1); id <= 5; id++ {
		if _, ok := hub.FirstSeq(id); ok {
			alive++
		}
	}
	if alive > 2 {
		t.Fatalf("%d runs retained, cap is 2", alive)
	}

	// TTL: the rest disappear as well.
	time.Sleep(60 * time.Millisecond)
	for id := int64(1); id <= 5; id++ {
		if _, ok := hub.FirstSeq(id); ok {
			t.Fatalf("run %d outlived its retention", id)
		}
	}
}
