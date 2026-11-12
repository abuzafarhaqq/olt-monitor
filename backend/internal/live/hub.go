// Package live is the in-memory fan-out for a running collection.
//
// One process, one hub: a run's output is broadcast to every subscriber, kept in
// a ring buffer so a late or reconnecting client can catch up, and dropped for
// subscribers that stop reading rather than letting one slow browser stall the
// SSH session.
package live

import (
	"sync"
	"sync/atomic"
	"time"
)

// RingSize is how much output a late subscriber can recover. A full optical
// query on four ports is a few hundred lines; 4000 events is generous and bounded.
const RingSize = 4000

// SubscriberBuffer bounds a single client. Full buffer = dropped events, never a
// blocked publisher: the collector must not wait for a browser.
const SubscriberBuffer = 512

// Event kinds. "out" is raw device bytes, "cmd" is the line that was sent,
// "end" closes the run, "error" carries a failure.
type Event struct {
	Seq  int64     `json:"seq"`
	Kind string    `json:"kind"`
	Data string    `json:"data,omitempty"`
	At   time.Time `json:"at"`
}

type subscriber struct {
	ch chan Event
}

type run struct {
	mu      sync.Mutex
	ring    []Event // circular, oldest overwritten
	head    int     // next write position
	count   int     // events currently held
	nextSeq atomic.Int64
	subs    map[*subscriber]struct{}
	closed  bool
	dropped int
}

// Hub owns one run state per run id.
type Hub struct {
	mu   sync.RWMutex
	runs map[int64]*run
}

func NewHub() *Hub { return &Hub{runs: make(map[int64]*run)} }

// Open registers a run before collection starts. Publishing to an unknown run
// panics in a test but is ignored in production — a late event is not worth a crash.
func (h *Hub) Open(runID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.runs[runID]; !exists {
		h.runs[runID] = &run{subs: make(map[*subscriber]struct{})}
	}
}

// Publish appends to the ring and fans out. Non-blocking by design.
func (h *Hub) Publish(runID int64, kind, data string) {
	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	ev := Event{Seq: r.nextSeq.Add(1), Kind: kind, Data: data, At: time.Now().UTC()}

	if len(r.ring) < RingSize {
		r.ring = append(r.ring, ev)
	} else {
		r.ring[r.head] = ev
		r.head = (r.head + 1) % RingSize
	}
	r.count++

	for sub := range r.subs {
		select {
		case sub.ch <- ev:
		default:
			// Slow subscriber: drop the event for that client only. Closing the
			// channel here would break the session's view mid-stream.
			r.dropped++
		}
	}
}

// Close marks the run finished, sends the final event and detaches subscribers.
func (h *Hub) Close(runID int64, kind, data string) {
	if kind != "" {
		h.Publish(runID, kind, data)
	}

	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for sub := range r.subs {
		close(sub.ch)
		delete(r.subs, sub)
	}
}

// History returns buffered events with Seq > after, oldest first. Used for
// Last-Event-ID resume and for a client that connects after the run started.
func (h *Hub) History(runID, after int64) []Event {
	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Event, 0, len(r.ring))
	for _, ev := range r.ring {
		if ev.Seq > after {
			out = append(out, ev)
		}
	}
	return out
}

// Subscribe returns a channel and a cancel function. If the run is already
// closed the channel is closed immediately — the caller replays History first.
func (h *Hub) Subscribe(runID int64) (<-chan Event, func()) {
	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}

	sub := &subscriber{ch: make(chan Event, SubscriberBuffer)}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		close(sub.ch)
		return sub.ch, func() {}
	}
	r.subs[sub] = struct{}{}
	r.mu.Unlock()

	return sub.ch, func() {
		r.mu.Lock()
		if _, ok := r.subs[sub]; ok {
			delete(r.subs, sub)
			close(sub.ch)
		}
		r.mu.Unlock()
	}
}

// Dropped reports how many events a run's slow subscribers missed, for the log.
func (h *Hub) Dropped(runID int64) int {
	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}
