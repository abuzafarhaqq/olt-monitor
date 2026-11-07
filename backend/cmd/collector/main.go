// Command collector runs allow-listed read-only commands against one OLT.
//
// This device (VSOL EPON) asks for credentials three times: SSH auth, then a
// character-mode CLI login ("Login:" / "Password:"), then "enable" for
// privileged mode. All three secrets come from the environment:
//
//	OLT_PASSWORD          ssh password
//	OLT_CLI_PASSWORD      the CLI login password (pass -cli-user to enable)
//	OLT_ENABLE_PASSWORD   the enable secret (privileged mode)
//
// Dry by default: without -confirm it prints what it would do and exits.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector/vsol"
)

func main() {
	host := flag.String("host", "", "device address (no default: there is none)")
	port := flag.Int("port", 22, "SSH port")
	user := flag.String("user", "", "SSH username")
	adapterName := flag.String("adapter", "vsol-epon", "vendor adapter")
	commandList := flag.String("commands", "version,time", "comma-separated ids from the adapter allow-list")
	knownHosts := flag.String("known-hosts", "", "known_hosts file used to verify the device key")
	cliUser := flag.String("cli-user", "", "character-mode CLI login (the device asks again after SSH)")
	timeout := flag.Duration("timeout", 20*time.Second, "per-command timeout")
	confirm := flag.Bool("confirm", false, "actually connect; without it this prints the plan and exits")
	acceptKey := flag.Bool("accept-key", false, "trust-on-first-use: record the device key and exit")
	discover := flag.Bool("discover", false, "ask the CLI with '?' what it supports, print the answers, run nothing")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "store this run in Postgres (empty = print only)")
	oltID := flag.String("olt-id", "", "database id of this OLT, required with -dsn")
	flag.Parse()

	adapter, err := resolveAdapter(*adapterName)
	if err != nil {
		exit(2, "%v", err)
	}

	ids := splitList(*commandList)
	plan := make([]string, 0, len(ids))
	for _, id := range ids {
		command, ok := adapter.Command(id)
		if !ok {
			exit(2, "%q is not in the %s allow-list", id, adapter.Name())
		}
		plan = append(plan, fmt.Sprintf("  %-12s %s", command.ID, command.Line))
	}

	fmt.Printf("adapter : %s\n", adapter.Name())
	if *cliUser != "" {
		fmt.Println("login   : ssh → CLI login → enable (three prompts; secrets from env)")
	} else {
		fmt.Println("login   : ssh only (-cli-user not given; enable password not used)")
	}
	fmt.Printf("device  : %s:%d (user %q)\n", *host, *port, *user)
	fmt.Printf("commands:\n%s\n", strings.Join(plan, "\n"))

	if *host == "" || *user == "" {
		exit(2, "-host and -user are required")
	}

	if *acceptKey {
		if *knownHosts == "" {
			exit(2, "-accept-key needs -known-hosts")
		}
		if err := recordKey(*host, *port, *knownHosts); err != nil {
			exit(1, "record key: %v", err)
		}
		fmt.Printf("\nkey recorded in %s — compare the fingerprint against the device out of band\n", *knownHosts)
		return
	}

	if !*confirm {
		fmt.Println("\ndry run: nothing was sent. Re-run with -confirm to execute these commands.")
		return
	}

	password := os.Getenv("OLT_PASSWORD")
	if password == "" {
		exit(2, "OLT_PASSWORD is not set (never a flag: it would land in shell history)")
	}
	cliPassword := os.Getenv("OLT_CLI_PASSWORD")
	enablePassword := os.Getenv("OLT_ENABLE_PASSWORD")
	if *cliUser != "" && cliPassword == "" {
		exit(2, "OLT_CLI_PASSWORD is not set but -cli-user was given")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout*time.Duration(len(ids)+2))
	defer cancel()

	session, err := collector.DialSSH(ctx, collector.SSHConfig{
		Host: *host, Port: *port, User: *user, Password: password,
		CLIUser: *cliUser, CLIPassword: cliPassword, EnablePassword: enablePassword,
		Timeout: *timeout, KnownHosts: *knownHosts,
	})
	if err != nil {
		exit(1, "connect: %v", err)
	}
	defer session.Close()

	if *discover {
		// Read-only: each probe ends in '?', which the CLI answers with its
		// option list instead of executing anything.
		if adapter.RequiresConfigMode() {
			if _, err := session.Run(ctx, "configure terminal"); err != nil {
				exit(1, "enter configuration mode: %v", err)
			}
		}
		// Probes clean up with backspaces and never press Enter, so the session
		// stays in configuration mode. Re-sending "configure terminal" from
		// inside config mode is rejected ("% Unknown command"), so it is not
		// done here.
		for _, probe := range adapter.Probes() {
			// Probe does not send CR: the '?' is answered without executing.
			output, err := session.Probe(ctx, probe)
			fmt.Printf("\n=== %s\n", probe)
			if err != nil {
				fmt.Printf("! %v\n", err)
			}
			fmt.Println(strings.TrimSpace(output))
		}
		return
	}

	if adapter.RequiresConfigMode() {
		// Entering the configuration prompt is required for `show onu …`; no
		// configuration command is ever sent from here.
		if _, err := session.Run(ctx, "configure terminal"); err != nil {
			exit(1, "enter configuration mode: %v", err)
		}
	}

	startedAt := time.Now().UTC()
	results, err := collector.Runner{Adapter: adapter, Session: session}.Run(ctx, ids)
	if err != nil {
		exit(1, "run: %v", err)
	}

	if *dsn != "" {
		if *oltID == "" {
			exit(2, "-dsn needs -olt-id (the database id of this device)")
		}
		if err := persist(ctx, *dsn, *oltID, results, startedAt); err != nil {
			exit(1, "store run: %v", err)
		}
		fmt.Printf("\nstored run for %s\n", *oltID)
	}

	for _, result := range results {
		fmt.Printf("\n=== %s :: %s\n", result.CommandID, result.Line)
		if result.Err != nil {
			fmt.Printf("! %v\n", result.Err)
		}
		if encoded, err := json.MarshalIndent(result.Parsed, "", "  "); err == nil && result.Parsed != nil {
			fmt.Printf("parsed:\n%s\n", encoded)
		}
		fmt.Printf("raw:\n%s\n", result.Raw)
	}
}

// persist maps the adapter's rows to the ingest shapes and writes one run.
// Evidence is written by the ingest layer from the raw output kept in Result.
func persist(ctx context.Context, dsn, oltID string, results []collector.Result, at time.Time) error {
	ing, err := collector.NewIngest(ctx, dsn, "vsol-epon@0.3")
	if err != nil {
		return err
	}
	defer ing.Close()

	run := collector.Run{OltID: oltID, At: at}
	for _, result := range results {
		job := collector.Job{ResultID: result.CommandID, Line: result.Line, Raw: result.Raw, Err: result.Err}

		switch rows := result.Parsed.(type) {
		case []vsol.OnuRow:
			job.Rows = vsol.ToStatusRows(rows)
		case []vsol.OnuIdentity:
			job.Rows = vsol.ToIdentityRows(rows)
		case []vsol.OpticalRow:
			job.Rows = vsol.ToOpticalRows(rows)
		}
		if result.Err != nil {
			run.Failed = true
			run.Reason = result.Err.Error()
		}
		run.Jobs = append(run.Jobs, job)
	}
	_, err = ing.Save(ctx, run)
	return err
}

func resolveAdapter(name string) (collector.Adapter, error) {
	switch name {
	case "vsol-epon":
		return vsol.New(), nil
	default:
		// A second vendor is a new package, not a new flag value here.
		return nil, fmt.Errorf("unknown adapter %q (available: vsol-epon)", name)
	}
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// recordKey enrols the host key on purpose. Authentication is expected to fail;
// the callback runs before it, which is exactly why this is a separate flag.
func recordKey(host string, port int, knownHostsPath string) error {
	address := net.JoinHostPort(host, fmt.Sprint(port))

	callback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer file.Close()

		fmt.Printf("fingerprint: %s\n", ssh.FingerprintSHA256(key))
		_, err = fmt.Fprintln(file, knownhosts.Line([]string{address}, key))
		return err
	}

	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User:            "enrolment-probe",
		Auth:            []ssh.AuthMethod{ssh.Password("unused")},
		HostKeyCallback: callback,
		Timeout:         10 * time.Second,
	})
	if err != nil {
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil
		}
		return err
	}
	return client.Close()
}

func exit(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}
