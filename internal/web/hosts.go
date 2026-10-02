package web

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"sync"
	"time"

	"ccm/internal/hub"
)

// hostsFile is hosts.toml, re-read whenever its modification time changes.
// A missing file means no hosts. A file that stops parsing keeps the last
// good hosts until it is fixed.
type hostsFile struct {
	path string

	mu      sync.Mutex
	modTime time.Time
	hosts   []hub.Host
}

// loadHostsFile reads the file once; a parse error here is a startup error.
func loadHostsFile(path string) (*hostsFile, error) {
	f := &hostsFile{path: path}
	if err := f.reloadLocked(); err != nil {
		return nil, err
	}
	return f, nil
}

// Hosts returns the current hosts, re-reading the file if it changed.
func (f *hostsFile) Hosts() []hub.Host {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.reloadLocked(); err != nil {
		log.Printf("ccm web: keeping previous hosts: %v", err)
	}
	return f.hosts
}

// Find returns the host with this name from the current file.
func (f *hostsFile) Find(name string) (hub.Host, bool) {
	for _, h := range f.Hosts() {
		if h.Name == name {
			return h, true
		}
	}
	return hub.Host{}, false
}

func (f *hostsFile) reloadLocked() error {
	st, err := os.Stat(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		f.hosts, f.modTime = nil, time.Time{}
		return nil
	}
	if err != nil {
		return err
	}
	if st.ModTime().Equal(f.modTime) {
		return nil
	}
	// Record the mtime even if parsing fails, so a broken file is reported
	// once per change rather than on every poll.
	f.modTime = st.ModTime()
	cfg, err := hub.LoadConfig(f.path)
	if err != nil {
		return err
	}
	f.hosts = cfg.Hosts
	return nil
}
