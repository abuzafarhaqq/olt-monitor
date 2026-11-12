package collector

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Crawler walks a device CLI with the help character.
//
// Capability discovery stops being a hand-written Go slice per vendor: send
// `show onu ?`, read the options, send `show onu status ?`, and so on. The tree
// that comes back IS the command list, stored per device so a firmware upgrade
// changes data instead of code.
//
// Safety is structural, not a promise: every probe ends with '?' and the CR that
// would execute it is never sent.
type Crawler struct {
	// ProbeHelp sends "<path> ?" and returns what the device printed. It must
	// not press Enter.
	ProbeHelp func(ctx context.Context, path string) (string, error)

	// Limits: a full CLI tree is not worth minutes of a production device's time.
	// Depth 4 is what `show onu status pon ?` needs: show→onu→status→pon.
	MaxDepth    int // default 4
	MaxNodes    int // default 400
	MaxChildren int // default 40 per node
	Timeout     time.Duration
}

// CommandKind decides whether a discovered command may ever be executed.
type CommandKind string

const (
	KindRead    CommandKind = "read"    // show/display/list — safe to run
	KindAction  CommandKind = "action"  // reboot/register/reset — never run
	KindParam   CommandKind = "param"   // <cr>, <MAC> — needs an argument
	KindUnknown CommandKind = "unknown" // not classified; treated as unsafe
)

// Node is one entry of the discovered tree.
type Node struct {
	// Path is the full command, e.g. ["show","onu","status","pon"].
	Path     []string
	Help     string
	Kind     CommandKind
	Children int
	Depth    int
}

// Command renders the path the way the device would see it.
func (n Node) Command() string { return strings.Join(n.Path, " ") }

// helpLine matches the two-column output of a Cisco-like CLI:
//
//	basic-info  Specify onu basic information.
var helpLine = regexp.MustCompile(`^\s{2,}(\S+)\s{2,}(.+)$`)

// actionVerbs may change the device, its clients or its configuration. They are
// recorded so an operator can see them, and never executed by this platform.
var actionVerbs = map[string]bool{
	"reboot": true, "reset": true, "restart": true, "register": true,
	"deregister": true, "discover": true, "delete": true, "remove": true,
	"clear": true, "write": true, "copy": true, "save": true, "load": true,
	"restore": true, "upgrade": true, "download": true, "upload": true,
	"set": true, "enable": true, "disable": true, "configure": true,
	"command": true, "pool": true, "timer": true, "schedule": true,
	"no": true, "format": true, "erase": true, "apply": true, "commit": true,
}

var readVerbs = map[string]bool{
	"show": true, "display": true, "list": true, "get": true, "info": true,
	"who": true,
}

// Classify decides what may be done with a discovered token.
func Classify(token, help string) CommandKind {
	lower := strings.ToLower(token)

	// <cr>, <HHHH:HHHH>, <MAC> — a slot for an argument, not a command.
	if strings.HasPrefix(lower, "<") || strings.Contains(help, "Just Press Enter") {
		return KindParam
	}
	if actionVerbs[lower] {
		return KindAction
	}
	if readVerbs[lower] {
		return KindRead
	}
	return KindUnknown
}

// Crawl walks the tree breadth-first and returns every node it found.
//
// KindRead/KindAction describe what may be *executed*; they do not gate
// navigation, because a help probe executes nothing.
//
// It stops on the limits rather than the tree's end: `show ?` on a real OLT
// offers ~70 verbs and each may have children, so an unbounded walk is a
// self-inflicted denial of service against the device.
func (c *Crawler) Crawl(ctx context.Context) ([]Node, error) {
	if c.ProbeHelp == nil {
		return nil, fmt.Errorf("crawler has no help probe")
	}
	if c.MaxDepth == 0 {
		c.MaxDepth = 4
	}
	if c.MaxNodes == 0 {
		c.MaxNodes = 400
	}
	if c.MaxChildren == 0 {
		c.MaxChildren = 40
	}
	if c.Timeout == 0 {
		c.Timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(c.Timeout)

	type queued struct {
		path  []string
		depth int
	}
	queue := []queued{{path: nil, depth: 0}}
	seen := map[string]bool{}
	nodes := make([]Node, 0, 64)

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nodes, err
		}
		if time.Now().After(deadline) {
			return nodes, fmt.Errorf("crawl stopped at its %s budget with %d nodes", c.Timeout, len(nodes))
		}
		if len(nodes) >= c.MaxNodes {
			return nodes, fmt.Errorf("crawl stopped at its %d-node budget", c.MaxNodes)
		}

		current := queue[0]
		queue = queue[1:]

		// The probe never carries a CR; "?" is answered as it is typed.
		probe := strings.TrimSpace(strings.Join(current.path, " ") + " ?")
		if !strings.HasSuffix(probe, "?") {
			return nodes, fmt.Errorf("probe %q would execute a command", probe)
		}

		output, err := c.ProbeHelp(ctx, probe)
		if err != nil {
			// Record the dead branch and carry on: one unprobeable subtree must
			// not abandon the whole crawl.
			if len(current.path) > 0 {
				nodes = append(nodes, Node{
					Path: current.path, Help: "probe failed: " + err.Error(),
					Kind: KindUnknown, Depth: current.depth,
				})
			}
			continue
		}

		children := ParseHelp(output)
		if len(children) > c.MaxChildren {
			children = children[:c.MaxChildren]
		}

		for _, child := range children {
			// Budget is enforced per child, not per probe: one answer can add 40
			// nodes and overshoot the cap the loop checked a moment ago.
			if len(nodes) >= c.MaxNodes {
				return nodes, fmt.Errorf("crawl stopped at its %d-node budget", c.MaxNodes)
			}

			path := append(append([]string{}, current.path...), child.Token)
			command := strings.Join(path, " ")
			if seen[command] {
				continue
			}
			seen[command] = true

			kind := Classify(child.Token, child.Help)
			nodes = append(nodes, Node{
				Path: path, Help: child.Help, Kind: kind, Depth: current.depth + 1,
			})

			// Descend into nouns too: `onu` and `status` are KindUnknown, and
			// without them `show onu status pon ?` is unreachable. Probing is
			// harmless — a probe never presses Enter, so nothing executes — and
			// the only tokens excluded are action verbs and argument slots.
			if kind != KindAction && kind != KindParam && current.depth+1 < c.MaxDepth {
				queue = append(queue, queued{path: path, depth: current.depth + 1})
			}
		}
	}
	return nodes, nil
}

// HelpEntry is one parsed line of a "?" answer.
type HelpEntry struct {
	Token string
	Help  string
}

// ParseHelp reads the option list out of a help answer, ignoring the echoed
// command, the prompt and any paging noise.
func ParseHelp(output string) []HelpEntry {
	entries := make([]HelpEntry, 0, 16)
	for _, line := range strings.Split(output, "\n") {
		match := helpLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		token := strings.TrimSpace(match[1])
		help := strings.TrimSpace(match[2])
		if token == "" || strings.HasPrefix(token, "%") || strings.HasPrefix(token, "!") {
			continue
		}
		entries = append(entries, HelpEntry{Token: token, Help: help})
	}
	return entries
}
