// Command collector runs allow-listed read-only commands against one OLT.
//
// Dry by default: without -confirm it prints exactly what it would run and
// exits. Dialling a production OLT is a deliberate per-run act, and the output
// is evidence — both halves of that are enforced here, not promised in a README.
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
	timeout := flag.Duration("timeout", 20*time.Second, "per-command timeout")
	confirm := flag.Bool("confirm", false, "actually connect; without it this prints the plan and exits")
	acceptKey := flag.Bool("accept-key", false, "trust-on-first-use: record the device key and exit")
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout*time.Duration(len(ids)+2))
	defer cancel()

	session, err := collector.DialSSH(ctx, collector.SSHConfig{
		Host: *host, Port: *port, User: *user, Password: password,
		Timeout: *timeout, KnownHosts: *knownHosts,
	})
	if err != nil {
		exit(1, "connect: %v", err)
	}
	defer session.Close()

	results, err := collector.Runner{Adapter: adapter, Session: session}.Run(ctx, ids)
	if err != nil {
		exit(1, "run: %v", err)
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
