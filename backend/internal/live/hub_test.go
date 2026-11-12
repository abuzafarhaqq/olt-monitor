package live_test

import (
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
	for i := 0; i < live.RingSize+500; i++ {
		hub.Publish(1, "out", "x")
	}
	if got := len(hub.History(1, 0)); got != live.RingSize {
		t.Fatalf("history = %d, want the ring size %d", got, live.RingSize)
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
