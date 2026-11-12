// Package live is the in-memory fan-out for a running collection.
//
// Bounds are in BYTES, not events. A device emits whatever chunk sizes the
// kernel hands over — 1 KB or 32 KB — so "4000 events" is anywhere between 4 MB
// and 128 MB of RAM. The ring keeps the tail, because that is what a terminal
// shows, and it is capped in both dimensions so one pathological run cannot
// exhaust the process.
package live

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// RingBytes is how much output a late subscriber can recover: the last
	// 4 MB, which is roughly half a full 25,000-ONU collection.
	RingBytes = 4 << 20
	// RingEvents caps the count as well, so a flood of tiny writes cannot make
	// the ring bookkeeping heavier than the payload.
	RingEvents = 20000

	// SubscriberBuffer bounds one client in events.
	SubscriberBuffer = 512

	// RetainAfterClose keeps a finished run available for reconnects, then frees
	// it. Without this the hub is a memory leak with a comment.
	RetainAfterClose = 10 * time.Minute
	// MaxRetainedRuns bounds the number of finished runs kept at once.
	MaxRetainedRuns = 20
)

type Event struct {
	Seq  int64     `json:"seq"`
	Kind string    `json:"kind"`
	Data string    `json:"data,omitempty"`
	At   time.Time `json:"at"`
}

type subscriber struct {
	ch      chan Event
	dropped int
}

type run struct {
	mu        sync.Mutex
	ring      []Event
	ringBytes int
	nextSeq   atomic.Int64
	subs      map[*subscriber]struct{}
	closed    bool
	closedAt  time.Time
	dropped   int
}

type Hub struct {
	mu   sync.RWMutex
	runs map[int64]*run
	// order is insertion order, used to evict the oldest finished runs first.
	order []int64

	ttl   time.Duration
	limit int
}

func NewHub() *Hub {
	return &Hub{runs: make(map[int64]*run), ttl: RetainAfterClose, limit: MaxRetainedRuns}
}

// NewHubWithRetention exists for tests: production wants the defaults.
func NewHubWithRetention(ttl time.Duration, limit int) *Hub {
	return &Hub{runs: make(map[int64]*run), ttl: ttl, limit: limit}
}

func (h *Hub) Open(runID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.runs[runID]; exists {
		return
	}
	h.runs[runID] = &run{subs: make(map[*subscriber]struct{})}
	h.order = append(h.order, runID)
}

// Publish appends to the ring and fans out. Non-blocking by design: the SSH
// reader must never wait for a browser.
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
	r.ring = append(r.ring, ev)
	r.ringBytes += len(ev.Data) + 64 // rough per-event overhead

	for (r.ringBytes > RingBytes || len(r.ring) > RingEvents) && len(r.ring) > 1 {
		dropped := r.ring[0]
		r.ring = r.ring[1:]
		r.ringBytes -= len(dropped.Data) + 64
	}

	for sub := range r.subs {
		select {
		case sub.ch <- ev:
		default:
			// A client that stops reading is disconnected rather than silently
			// losing bytes: dropped bytes in the middle of a vt100 escape
			// sequence render as garbage. It reconnects with Last-Event-ID and
			// is told exactly how much it missed.
			sub.dropped++
			r.dropped++
			close(sub.ch)
			delete(r.subs, sub)
		}
	}
}

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
	r.closed = true
	r.closedAt = time.Now()
	for sub := range r.subs {
		close(sub.ch)
		delete(r.subs, sub)
	}
	r.mu.Unlock()

	// Free the run after the retention window, and enforce the count cap now.
	time.AfterFunc(h.ttl, func() {
		h.mu.Lock()
		delete(h.runs, runID)
		h.mu.Unlock()
	})
	h.evictFinished()
}

func (h *Hub) evictFinished() {
	h.mu.Lock()
	defer h.mu.Unlock()

	finished := 0
	for _, id := range h.order {
		if r := h.runs[id]; r != nil {
			r.mu.Lock()
			closed := r.closed
			r.mu.Unlock()
			if closed {
				finished++
			}
		}
	}

	for _, id := range h.order {
		if finished <= h.limit {
			break
		}
		if r := h.runs[id]; r != nil {
			r.mu.Lock()
			closed := r.closed
			r.mu.Unlock()
			if closed {
				delete(h.runs, id)
				finished--
			}
		}
	}
}

// History returns buffered events with Seq > after, oldest first.
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

// FirstSeq reports the oldest event still buffered, so the stream can tell a
// client that the beginning is gone instead of pretending it has everything.
func (h *Hub) FirstSeq(runID int64) (int64, bool) {
	h.mu.RLock()
	r := h.runs[runID]
	h.mu.RUnlock()
	if r == nil {
		return 0, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.ring) == 0 {
		return 0, false
	}
	return r.ring[0].Seq, true
}

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
