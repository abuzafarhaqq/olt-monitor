// Command userctl manages accounts before there is a user-management API.
//
// The password is never taken from a flag: a flag lands in shell history and in
// `ps`. It is read from the terminal without echo, or piped on stdin for
// automation.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := postgres.New(ctx, dsn)
	if err != nil {
		slog.Error("connect", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	switch os.Args[1] {
	case "create":
		os.Exit(create(ctx, st, os.Args[2:]))
	case "passwd":
		os.Exit(passwd(ctx, st, os.Args[2:]))
	case "list":
		os.Exit(list(ctx, st, os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `userctl — manage OLT monitor accounts

  userctl create -username alice -name "Alice Rahman" -email alice@example.net \
                 -role noc-operator [-phone +880...]
  userctl passwd -username alice
  userctl list

Roles: administrator | network-engineer | noc-operator | viewer
The password is read from the terminal (no echo) or stdin; it is never a flag.
`)
}

func create(ctx context.Context, st *postgres.Store, args []string) int {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	username := fs.String("username", "", "login name")
	name := fs.String("name", "", "full name")
	email := fs.String("email", "", "email address")
	phone := fs.String("phone", "", "phone number (optional)")
	role := fs.String("role", "viewer", "administrator|network-engineer|noc-operator|viewer")
	_ = fs.Parse(args)

	if *username == "" || *name == "" || *email == "" {
		fmt.Fprintln(os.Stderr, "-username, -name and -email are required")
		return 2
	}

	password, err := readPassword(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "password: %v\n", err)
		return 1
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash: %v\n", err)
		return 1
	}

	user, err := st.CreateUser(ctx, postgres.CreateUserParams{
		Username:     *username,
		FullName:     *name,
		Email:        *email,
		Phone:        *phone,
		Role:         *role,
		PasswordHash: hash,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create: %v\n", err)
		return 1
	}

	fmt.Printf("created %s (%s) with role %s\n", user.Username, user.ID, user.Role)
	return 0
}

func passwd(ctx context.Context, st *postgres.Store, args []string) int {
	fs := flag.NewFlagSet("passwd", flag.ExitOnError)
	username := fs.String("username", "", "login name")
	_ = fs.Parse(args)

	if *username == "" {
		fmt.Fprintln(os.Stderr, "-username is required")
		return 2
	}

	password, err := readPassword(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "password: %v\n", err)
		return 1
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash: %v\n", err)
		return 1
	}

	tag, err := st.Pool().Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE lower(username) = lower($2)`,
		hash, *username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update: %v\n", err)
		return 1
	}
	if tag.RowsAffected() == 0 {
		fmt.Fprintf(os.Stderr, "no such user: %s\n", *username)
		return 1
	}

	// Every session minted with the old password dies with it.
	if _, err := st.Pool().Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE revoked_at IS NULL
		  AND user_id = (SELECT id FROM users WHERE lower(username) = lower($1))`, *username); err != nil {
		fmt.Fprintf(os.Stderr, "revoke sessions: %v\n", err)
		return 1
	}

	fmt.Printf("password updated for %s; existing sessions revoked\n", *username)
	return 0
}

func list(ctx context.Context, st *postgres.Store, _ []string) int {
	rows, err := st.Pool().Query(ctx,
		`SELECT username, full_name, role::text, disabled, coalesce(last_login_at, to_timestamp(0))
		   FROM users ORDER BY username`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		return 1
	}
	defer rows.Close()

	fmt.Printf("%-20s %-28s %-18s %-9s %s\n", "USERNAME", "NAME", "ROLE", "DISABLED", "LAST LOGIN")
	for rows.Next() {
		var username, name, role string
		var disabled bool
		var lastLogin time.Time
		if err := rows.Scan(&username, &name, &role, &disabled, &lastLogin); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			return 1
		}
		last := "never"
		if !lastLogin.Equal(time.Unix(0, 0)) {
			last = lastLogin.Format(time.RFC3339)
		}
		fmt.Printf("%-20s %-28s %-18s %-9v %s\n", username, name, role, disabled, last)
	}
	return 0
}

// readPassword prompts twice when attached to a terminal, and reads one line
// when piped. Nothing is echoed and nothing is logged.
func readPassword(confirm bool) (string, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}

	if confirm {
		fmt.Fprint(os.Stderr, "Repeat: ")
		second, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			return "", fmt.Errorf("passwords do not match")
		}
	}

	return string(first), nil
}
