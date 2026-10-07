package agent

import (
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/varogonz95/clawsh/internal/api"
)

// DefaultExternalTTL is how long an announced external session is kept
// without a renewal. `clawsh mcp` renews every ExternalTTL/3.
const DefaultExternalTTL = 90 * time.Second

// externals tracks sessions announced by `clawsh mcp` stubs running inside
// Claude Code sessions the agent does not own.
type externals struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]api.External
}

func newExternals(ttl time.Duration) *externals {
	if ttl <= 0 {
		ttl = DefaultExternalTTL
	}
	return &externals{ttl: ttl, m: map[string]api.External{}}
}

var errBadID = errors.New("id must be 1-64 characters of [0-9a-zA-Z_-]")

func validExternalID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// Announce creates or renews an external session lease.
func (e *externals) announce(id string, req api.AnnounceRequest) (api.External, error) {
	if !validExternalID(id) {
		return api.External{}, errBadID
	}
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	x, ok := e.m[id]
	if !ok {
		x = api.External{ID: id, Created: now}
	}
	x.Dir, x.Pid, x.Name = req.Dir, req.Pid, req.Name
	if x.Name == "" && x.Dir != "" {
		x.Name = filepath.Base(x.Dir)
	}
	x.Seen = now
	e.m[id] = x
	return x, nil
}

func (e *externals) withdraw(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.m[id]; !ok {
		return ErrNotFound
	}
	delete(e.m, id)
	return nil
}

// list returns live entries, oldest first, dropping expired leases.
func (e *externals) list() []api.External {
	cutoff := time.Now().Add(-e.ttl)
	e.mu.Lock()
	out := make([]api.External, 0, len(e.m))
	for id, x := range e.m {
		if x.Seen.Before(cutoff) {
			delete(e.m, id)
			continue
		}
		out = append(out, x)
	}
	e.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}
