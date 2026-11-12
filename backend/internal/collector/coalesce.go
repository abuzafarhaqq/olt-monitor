package collector

import (
	"strings"
	"sync"
	"time"
)

// Coalescer batches device output before publishing it.
//
// The SSH reader delivers whatever the kernel hands over: a 6 kB table can
// arrive as one chunk or as sixty. Publishing each one as its own event makes
// the stream chattier than the data and multiplies per-event overhead in the
// ring buffer. This merges them to one event per 64 kB or 60 ms, whichever comes
// first — and always flushes before a non-output event, so ordering survives.
type Coalescer struct {
	mu       sync.Mutex
	buf      strings.Builder
	publish  func(kind, data string)
	maxBytes int
	interval time.Duration
	timer    *time.Timer
	closed   bool
}

func NewCoalescer(publish func(kind, data string), maxBytes int, interval time.Duration) *Coalescer {
	if maxBytes <= 0 {
		maxBytes = 64 << 10
	}
	if interval <= 0 {
		interval = 60 * time.Millisecond
	}
	return &Coalescer{publish: publish, maxBytes: maxBytes, interval: interval}
}

// Handle routes an observer event: output is batched, everything else is a
// boundary and flushes first.
func (c *Coalescer) Handle(kind, data string) {
	if kind == "out" {
		c.Out(data)
		return
	}
	c.Emit(kind, data)
}

func (c *Coalescer) Out(data string) {
	if data == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	c.buf.WriteString(data)

	if c.buf.Len() >= c.maxBytes {
		c.flushLocked()
		return
	}
	if c.timer == nil {
		c.timer = time.AfterFunc(c.interval, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.timer = nil
			c.flushLocked()
		})
	}
}

// Emit flushes buffered output first so a command line never appears after the
// output it produced.
func (c *Coalescer) Emit(kind, data string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.flushLocked()
	if c.publish != nil {
		c.publish(kind, data)
	}
}

func (c *Coalescer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.flushLocked()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.closed = true
}

func (c *Coalescer) flushLocked() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.buf.Len() == 0 {
		return
	}
	payload := c.buf.String()
	c.buf.Reset()
	if c.publish != nil {
		c.publish("out", payload)
	}
}
