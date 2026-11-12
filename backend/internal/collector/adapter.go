// Package collector drives vendor adapters. An adapter owns three things: the
// exact commands a device allows, the parser for each, and the capability facts
// that follow (does this model expose optics at all?).
//
// Nothing here opens a socket. Dialling lives in Session, and every caller has
// to ask for it explicitly.
package collector

import (
	"context"
	"fmt"
)

// Command is one read-only query an adapter has approved.
type Command struct {
	ID    string
	Line  string
	Parse func(output string) (any, error)
}

// Result is one executed command with its raw evidence. Raw is stored verbatim:
// a parser bug must never destroy what the device actually said.
type Result struct {
	CommandID string
	Line      string
	Raw       string
	Parsed    any
	Err       error
}

// Adapter is what a vendor/model/firmware combination implements.
type Adapter interface {
	// Name is used in logs and evidence paths, e.g. "vsol-epon".
	Name() string
	// Commands is the allow-list. There is no free-form execution anywhere.
	Commands() []Command
	// Command finds one by id.
	Command(id string) (Command, bool)
	// RequiresConfigMode reports whether the commands only exist inside the
	// configuration prompt. On the VSOL EPON the whole `show onu …` family is
	// rejected in privileged mode with "% Unknown command".
	RequiresConfigMode() bool
	// Probes are read-only `?` queries used to discover what this firmware
	// actually offers. "?" is answered immediately by the CLI and changes
	// nothing; it is how the allow-list is checked against a real device
	// instead of being assumed.
	Probes() []string
}

// Session is the transport. The collector only ever sees this interface, so a
// test can drive it with recorded output instead of a device.
type Session interface {
	Run(ctx context.Context, line string) (string, error)
	Close() error
}

// Runner executes a fixed list of commands through a session.
type Runner struct {
	Adapter Adapter
	Session Session
	// Observe receives the outgoing line before it is sent, so the terminal
	// shows the command before its output arrives.
	Observe Observer
	// MaxOutputBytes truncates a runaway response instead of exhausting memory.
	MaxOutputBytes int
}

const defaultMaxOutput = 2 << 20 // 2 MiB

func (r Runner) Run(ctx context.Context, ids []string) ([]Result, error) {
	limit := r.MaxOutputBytes
	if limit == 0 {
		limit = defaultMaxOutput
	}

	results := make([]Result, 0, len(ids))
	for _, id := range ids {
		command, ok := r.Adapter.Command(id)
		if !ok {
			// An id outside the allow-list is a programming error, not a device
			// problem: refuse the whole batch rather than skipping silently.
			return results, fmt.Errorf("command %q is not allow-listed for %s", id, r.Adapter.Name())
		}

		if r.Observe != nil {
			r.Observe("cmd", command.Line)
		}

		raw, err := r.Session.Run(ctx, command.Line)
		if len(raw) > limit {
			raw = raw[:limit]
		}

		result := Result{CommandID: id, Line: command.Line, Raw: raw, Err: err}
		if err == nil && command.Parse != nil {
			parsed, parseErr := command.Parse(raw)
			result.Parsed = parsed
			result.Err = parseErr
		}
		results = append(results, result)
	}
	return results, nil
}
