package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/varogonz95/clawsh/internal/paths"
)

// DefaultTokenPath is where the agent keeps its bearer token.
func DefaultTokenPath() string {
	return filepath.Join(paths.ConfigDir(), "agent.token")
}

// LoadOrCreateToken reads the token at path, generating one on first run.
// created reports whether a new token was written.
func LoadOrCreateToken(path string) (token string, created bool, err error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, false, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", false, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false, err
	}
	token = hex.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", false, err
	}
	return token, true, nil
}
