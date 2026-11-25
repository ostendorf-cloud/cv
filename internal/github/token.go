package github

import (
	"errors"
	"fmt"
	"strings"
)

// Environment variable names of the token. TokenFileEnv wins over TokenEnv.
const (
	TokenEnv     = "GITHUB_TOKEN"
	TokenFileEnv = "GITHUB_TOKEN_FILE"
)

// TokenFromEnv returns the token for the API calls, or "" when none is set,
// which is a valid state: public repositories work anonymously at the lower
// rate limit. GITHUB_TOKEN_FILE names a file holding the token and wins over
// GITHUB_TOKEN. A set but unreadable or empty file is an error, because the
// operator asked for a token and would otherwise silently get none. No error
// carries the token.
func TokenFromEnv(getenv func(string) string, readFile func(string) ([]byte, error)) (string, error) {
	if path := getenv(TokenFileEnv); path != "" {
		data, err := readFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", TokenFileEnv, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("%s: %w", TokenFileEnv, errEmptyTokenFile)
		}
		return token, nil
	}
	return strings.TrimSpace(getenv(TokenEnv)), nil
}

var errEmptyTokenFile = errors.New("file is empty")
