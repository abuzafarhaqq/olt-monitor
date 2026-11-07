package collector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHConfig describes one connection. There is no Telnet implementation here on
// purpose: the design permits it only for legacy devices on an isolated network,
// and nothing should make that easy by accident.
type SSHConfig struct {
	Host       string
	Port       int
	User       string
	Password   string
	Timeout    time.Duration
	KnownHosts string
	// Prompt marks the end of a command's output. An EPON CLI is character mode,
	// so "finished" means the prompt came back — not EOF.
	Prompt *regexp.Regexp
}

// DefaultPrompt matches the prompts seen on VSOL EPON: user, privileged and
// configuration modes all end in # or >.
var DefaultPrompt = regexp.MustCompile(`[#>]\s*$`)

type SSHSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	prompt  *regexp.Regexp
	timeout time.Duration

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
	if err := session.RequestPty("vt100", 60, 200, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
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

	s := &SSHSession{client: client, session: session, stdin: stdin, timeout: timeout}
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
	return s, nil
}

type lockedWriter struct{ session *SSHSession }

func (w *lockedWriter) Write(p []byte) (int, error) {
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

	if _, err := io.WriteString(s.stdin, line+"\r"); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	deadline := time.Now().Add(s.timeout)
	for {
		if ctx.Err() != nil {
			return s.snapshot(), ctx.Err()
		}
		if time.Now().After(deadline) {
			// Timeout is a result, not a crash: the partial output is evidence.
			return s.snapshot(), fmt.Errorf("timed out after %s waiting for the prompt", s.timeout)
		}

		output := s.snapshot()
		if trimmed := trimRight(output); s.prompt.MatchString(trimmed) && trimmed != "" {
			// Drop the echoed command and the trailing prompt.
			return stripEcho(trimmed, line), nil
		}
		time.Sleep(80 * time.Millisecond)
	}
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

// stripEcho removes the echoed command line and the trailing prompt, so callers
// see the device's answer and nothing else.
func stripEcho(output, line string) string {
	if len(output) >= len(line) && output[:len(line)] == line {
		output = output[len(line):]
	}
	if index := bytes.LastIndexByte([]byte(output), '\n'); index >= 0 {
		tail := output[index+1:]
		if bytes.ContainsAny([]byte(tail), "#>") && len(bytes.TrimSpace([]byte(tail))) <= 3 {
			output = output[:index]
		}
	}
	return string(bytes.TrimSpace([]byte(output)))
}
