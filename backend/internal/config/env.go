// Package config loads the same .secrets/app.env that app.sh sources, so a CLI
// behaves like the script does.
//
// Without this, `userctl create` needs DATABASE_URL exported by hand while
// `./app.sh` already knows it — a difference nobody should have to remember.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Candidate paths, relative to the working directory. The first one that exists
// is loaded; running from the repo root and from a subdirectory both work.
var candidates = []string{
	".secrets/app.env",
	filepath.Join("..", ".secrets", "app.env"),
	filepath.Join("..", "..", ".secrets", "app.env"),
}

// LoadEnvFile reads KEY=VALUE lines and sets only variables that are not already
// set, so an explicit export always wins over the file.
func LoadEnvFile() (string, error) {
	for _, path := range candidates {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		// #nosec G304 - the path is one of three fixed candidates.
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if key == "" {
				continue
			}
			if _, present := os.LookupEnv(key); !present {
				if err := os.Setenv(key, value); err != nil {
					_ = file.Close()
					return "", err
				}
			}
		}
		err = scanner.Err()
		_ = file.Close()
		return path, err
	}
	return "", os.ErrNotExist
}
