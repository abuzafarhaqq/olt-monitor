package collector

import "strings"

// Observer receives what the terminal needs to show: the line that was sent
// ("cmd") and the bytes that came back ("out").
type Observer func(kind, data string)

// Redactor masks the credentials used for a session.
//
// The device does not echo a password, but banners, help text and error
// messages sometimes contain the username, and any future command could print a
// config line with a secret in it. Masking on the way out costs nothing and
// removes a class of accident from a stream that a browser renders as text.
type Redactor struct {
	secrets []string
	mask    string
}

func NewRedactor(secrets ...string) *Redactor {
	unique := make([]string, 0, len(secrets))
	seen := map[string]bool{}
	for _, secret := range secrets {
		// Anything shorter than 4 characters would mask ordinary words.
		if len(secret) < 4 || seen[secret] {
			continue
		}
		seen[secret] = true
		unique = append(unique, secret)
	}
	// Longest first: "abc12345" must be masked before "abc123", or the tail of
	// the longer secret survives.
	for i := 0; i < len(unique); i++ {
		for j := i + 1; j < len(unique); j++ {
			if len(unique[j]) > len(unique[i]) {
				unique[i], unique[j] = unique[j], unique[i]
			}
		}
	}
	return &Redactor{secrets: unique, mask: "********"}
}

func (r *Redactor) Redact(s string) string {
	if r == nil || len(r.secrets) == 0 || s == "" {
		return s
	}
	out := s
	for _, secret := range r.secrets {
		out = strings.ReplaceAll(out, secret, r.mask)
	}
	return out
}

// Tee builds an Observer that redacts, then publishes.
func Tee(redactor *Redactor, publish func(kind, data string)) Observer {
	return func(kind, data string) {
		if publish == nil {
			return
		}
		publish(kind, redactor.Redact(data))
	}
}
