package collector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHConfig describes one connection. There is no Telnet implementation here on
// purpose: the design permits it only for legacy devices on an isolated network,
// and nothing should make that easy by accident.
type SSHConfig struct {
	Host     string
	Port     int
	User     string
	Password string

	// This device logs in three times: SSH, then a character-mode CLI login
	// ("Login:" / "Password:"), then "enable" asks for a third secret. All three
	// are separate credentials and must be supplied separately.
	CLIUser        string
	CLIPassword    string
	EnablePassword string

	// Timeout bounds dialling and waiting for a prompt: seconds.
	Timeout time.Duration
	// CommandTimeout bounds one command's output. A 25,000-ONU table takes
	// minutes to print, so the prompt timeout cannot serve both purposes.
	CommandTimeout time.Duration
	KnownHosts     string
	// Prompt marks the end of a command's output. An EPON CLI is character mode,
	// so "finished" means the prompt came back — not EOF.
	Prompt *regexp.Regexp
}

// DefaultPrompt matches the prompts seen on VSOL EPON: user, privileged and
// configuration modes all end in # or >.
var DefaultPrompt = regexp.MustCompile(`[#>]\s*$`)

type SSHSession struct {
	client     *ssh.Client
	session    *ssh.Session
	stdin      io.WriteCloser
	prompt     *regexp.Regexp
	timeout    time.Duration
	cmdTimeout time.Duration

	cliUser        string
	cliPassword    string
	enablePassword string

	// observe is set only after login completes: the login banner and prompts
	// are not streamed, so a half-authenticated client never sees them.
	observe Observer

	mu     sync.Mutex
	buffer bytes.Buffer
}

// DialSSH verifies the host key against a known_hosts file. An unknown key is
// refused: silently trusting a first connection is how a device on a hijacked
// address gets your credentials.
func DialSSH(ctx context.Context, cfg SSHConfig) (*SSHSession, error) {
	if cfg.Password == "" {
		return nil, fmt.Errorf("no password supplied")
	}
	if cfg.KnownHosts == "" {
		return nil, fmt.Errorf("no known_hosts file: record the device key deliberately first")
	}
	if _, err := os.Stat(cfg.KnownHosts); err != nil {
		return nil, fmt.Errorf("known_hosts %s: %w", cfg.KnownHosts, err)
	}

	hostKeyCallback, err := knownhosts.New(cfg.KnownHosts)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts: %w", err)
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	address := net.JoinHostPort(cfg.Host, fmt.Sprint(port))
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.Password(cfg.Password)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}

	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("session: %w", err)
	}

	// A PTY is required: the CLI will not print tables without one.
	if err := session.RequestPty("vt100", 60, 200, ssh.TerminalModes{}); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("stdin: %w", err)
	}

	cmdTimeout := cfg.CommandTimeout
	if cmdTimeout == 0 {
		cmdTimeout = timeout
	}

	s := &SSHSession{
		client: client, session: session, stdin: stdin, timeout: timeout, cmdTimeout: cmdTimeout,
		cliUser: cfg.CLIUser, cliPassword: cfg.CLIPassword, enablePassword: cfg.EnablePassword,
	}
	if cfg.Prompt != nil {
		s.prompt = cfg.Prompt
	} else {
		s.prompt = DefaultPrompt
	}

	session.Stdout = &lockedWriter{session: s}
	session.Stderr = &lockedWriter{session: s}

	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("shell: %w", err)
	}
	if err := s.login(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// login walks the prompts the device actually shows: a character-mode login,
// then privileged mode. Each step waits for its prompt with the same timeout as
// a command, so a device that never answers fails instead of hanging.
func (s *SSHSession) login(ctx context.Context) error {
	if s.cliUser != "" {
		if err := s.answer(ctx, regexp.MustCompile(`(?i)login\s*:\s*$`), s.cliUser); err != nil {
			return fmt.Errorf("cli login prompt: %w", err)
		}
		if err := s.answer(ctx, regexp.MustCompile(`(?i)password\s*:\s*$`), s.cliPassword); err != nil {
			return fmt.Errorf("cli password prompt: %w", err)
		}
	}

	// User mode prompt.
	if _, err := s.readUntil(ctx, regexp.MustCompile(`[#>]\s*$`)); err != nil {
		return fmt.Errorf("user-mode prompt: %w", err)
	}

	if s.enablePassword != "" {
		s.reset()
		if _, err := io.WriteString(s.stdin, "enable\r"); err != nil {
			return fmt.Errorf("enable: %w", err)
		}
		if err := s.answer(ctx, regexp.MustCompile(`(?i)password\s*:\s*$`), s.enablePassword); err != nil {
			return fmt.Errorf("enable password prompt: %w", err)
		}
		if _, err := s.readUntil(ctx, regexp.MustCompile(`#\s*$`)); err != nil {
			return fmt.Errorf("privileged prompt: %w", err)
		}

		// Long tables are paged with "--More--", which stalls a scripted reader
		// forever. Ask for no pager; if this device rejects the syntax the
		// command fails harmlessly and Run() pages manually instead.
		_ = s.pagerOff(ctx)
	}
	return nil
}

// pagerOff tries the two usual incantations and ignores the result. Failing is
// expected on devices that do not support it; Run() copes either way.
func (s *SSHSession) pagerOff(ctx context.Context) error {
	for _, line := range []string{"terminal length 0", "terminal datadump"} {
		s.reset()
		if _, err := io.WriteString(s.stdin, line+"\r"); err != nil {
			return err
		}
		if _, err := s.readUntil(ctx, regexp.MustCompile(`[#>]\s*$`)); err != nil {
			return err
		}
	}
	return nil
}

// awaitEcho waits until the device echoes the whole line back. A partial echo
// means characters were dropped on the way in, and running the command anyway
// would produce a result for the wrong command.
func (s *SSHSession) awaitEcho(ctx context.Context, line string) error {
	// The window is the command timeout, not a fixed few seconds: `show onu
	// opm-diag …` polls every ONU before printing anything, and a short window
	// reports "no echo" for a command that is simply still running.
	deadline := time.Now().Add(s.cmdTimeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		seen := s.snapshot()
		// Echo arrived, or the command has already completed and returned the
		// prompt — some CLIs swallow the echo of slow commands.
		if strings.Contains(seen, line) || (seen != "" && s.prompt.MatchString(trimRight(seen))) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("device did not echo %q back within %s (saw %q)", line, s.cmdTimeout, tail(seen, 80))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// answer waits for a prompt and replies with one line.
func (s *SSHSession) answer(ctx context.Context, prompt *regexp.Regexp, value string) error {
	if _, err := s.readUntil(ctx, prompt); err != nil {
		return err
	}
	s.reset()
	if _, err := io.WriteString(s.stdin, value+"\r"); err != nil {
		return err
	}
	return nil
}

// readUntil blocks until the accumulated output matches, the context ends or
// the timeout expires. It returns whatever was seen, so a caller can put it in
// an error message that is worth reading.
func (s *SSHSession) readUntil(ctx context.Context, match *regexp.Regexp) (string, error) {
	deadline := time.Now().Add(s.timeout)
	for {
		if ctx.Err() != nil {
			return s.snapshot(), ctx.Err()
		}
		output := s.snapshot()
		if match.MatchString(trimRight(output)) {
			return output, nil
		}
		if time.Now().After(deadline) {
			return output, fmt.Errorf("timed out after %s; last output: %q", s.timeout, tail(output, 120))
		}
		time.Sleep(80 * time.Millisecond)
	}
}

// stripPager removes the paging markers and the control characters that arrive
// with them, so a parser sees a continuous table.
func stripPager(s string) string {
	s = strings.ReplaceAll(s, " --More-- ", "")
	s = strings.ReplaceAll(s, "--More--", "")
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\x08", "")
	return s
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

type lockedWriter struct{ session *SSHSession }

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.session.mu.Lock()
	observer := w.session.observe
	w.session.mu.Unlock()

	// Publish outside the buffer lock: a slow SSE subscriber must never hold up
	// the reader that is draining the device.
	if observer != nil && len(p) > 0 {
		observer("out", string(p))
	}

	w.session.mu.Lock()
	defer w.session.mu.Unlock()
	return w.session.buffer.Write(p)
}

func (s *SSHSession) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String()
}

func (s *SSHSession) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffer.Reset()
}

// Run writes one allow-listed line and waits for the prompt to return.
//
// This is the only path by which a command reaches a device, and the collector
// binary refuses to dial at all without -confirm.
func (s *SSHSession) Run(ctx context.Context, line string) (string, error) {
	s.reset()

	// A character-mode CLI loses leading characters if the line arrives while it
	// is still settling after the previous prompt. A short pause plus an echo
	// check is what makes this reliable, and the check is what turns a silent
	// "Unknown command" into a diagnostic.
	time.Sleep(150 * time.Millisecond)
	if _, err := io.WriteString(s.stdin, line+"\r"); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	if err := s.awaitEcho(ctx, line); err != nil {
		return s.snapshot(), err
	}

	deadline := time.Now().Add(s.cmdTimeout)
	pages := 0

	for {
		if ctx.Err() != nil {
			return stripPager(s.snapshot()), ctx.Err()
		}

		output := s.snapshot()

		// A paging prompt is not a finished command: advance it. Without this a
		// 135-row table stalls until the timeout and yields one truncated page.
		if seen := strings.Count(output, "--More--"); seen > pages {
			pages = seen
			if pages > 500 {
				return stripPager(output), fmt.Errorf("pager did not finish after %d pages", pages)
			}
			if _, err := io.WriteString(s.stdin, " "); err != nil {
				return stripPager(output), fmt.Errorf("advance pager: %w", err)
			}
		}

		if time.Now().After(deadline) {
			// Timeout is a result, not a crash: the partial output is evidence.
			return stripPager(output), fmt.Errorf("timed out after %s waiting for the prompt", s.cmdTimeout)
		}

		trimmed := trimRight(output)
		if s.prompt.MatchString(trimmed) && trimmed != "" && !strings.HasSuffix(trimmed, "--More--") {
			// Drop the echoed command and the trailing prompt.
			return stripEcho(stripPager(trimmed), line), nil
		}
		time.Sleep(80 * time.Millisecond)
	}
}

// Probe sends text ending in "?" WITHOUT a carriage return.
//
// On this CLI the CR is the Enter key: sending "show onu ?" plus CR would print
// the option list and then execute "show onu". The question mark is answered as
// it is typed, so the line is never submitted — it is discarded afterwards with
// Ctrl-C, which aborts the pending input without running it.
func (s *SSHSession) Probe(ctx context.Context, text string) (string, error) {
	s.reset()
	time.Sleep(150 * time.Millisecond)

	if _, err := io.WriteString(s.stdin, text); err != nil {
		return "", fmt.Errorf("write probe: %w", err)
	}

	// The CLI redraws the prompt with the half-typed line after the options, so
	// waiting for a prompt is unreliable; wait for silence instead.
	deadline := time.Now().Add(s.timeout)
	last := ""
	quiet := 0
	pages := 0
	for {
		if ctx.Err() != nil {
			return stripPager(s.snapshot()), ctx.Err()
		}
		current := s.snapshot()

		// Option lists page too. A paging prompt is not silence: advance it.
		if seen := strings.Count(current, "--More--"); seen > pages {
			pages = seen
			if pages > 200 {
				return stripPager(current), fmt.Errorf("pager did not finish after %d pages", pages)
			}
			if _, err := io.WriteString(s.stdin, " "); err != nil {
				return stripPager(current), err
			}
			last, quiet = current, 0
			time.Sleep(80 * time.Millisecond)
			continue
		}

		if current == last {
			quiet++
			if quiet >= 8 { // ~400 ms with no new bytes
				break
			}
		} else {
			last = current
			quiet = 0
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	output := stripPager(s.snapshot())

	// Abandon the pending line with backspaces, then Ctrl-U as a belt.
	//
	// Not Ctrl-C: on this CLI Ctrl-C leaves the configuration prompt entirely,
	// so the next probe ran in privileged mode and answered "no matched
	// command". Backspaces only edit the line that was never submitted.
	s.reset()
	if _, err := io.WriteString(s.stdin, strings.Repeat("\x08", len(text)+4)); err != nil {
		return output, fmt.Errorf("clear pending line: %w", err)
	}
	if _, err := io.WriteString(s.stdin, "\x15"); err != nil {
		return output, fmt.Errorf("kill pending line: %w", err)
	}
	_, _ = s.readUntil(ctx, s.prompt)

	return strings.TrimSpace(output), nil
}

// SetObserver starts streaming device output. Called after login, so the
// credentials sequence and its prompts are never part of the stream.
func (s *SSHSession) SetObserver(o Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observe = o
}

func (s *SSHSession) Close() error {
	if s.session != nil {
		_ = s.session.Close()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func trimRight(s string) string {
	return string(bytes.TrimRight([]byte(s), " \t\r\n"))
}

// promptLine matches a real device prompt as a whole line: "epon-olt#",
// "epon-olt(config)#", "switch>". The earlier version only accepted prompts of
// three characters or fewer, so "epon-olt(config)#" stayed in the evidence.
var promptLine = regexp.MustCompile(`(?m)^[A-Za-z0-9._\-()/]*[#>]\s*$`)

// stripEcho removes the echoed command and the trailing prompt(s), so callers
// see the device's answer and nothing else.
func stripEcho(output, line string) string {
	if len(output) >= len(line) && output[:len(line)] == line {
		output = output[len(line):]
	}
	output = promptLine.ReplaceAllString(output, "")
	return string(bytes.TrimSpace([]byte(output)))
}
