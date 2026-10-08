package web

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"sync"
	"time"

	"github.com/varogonz95/clawsh/internal/hub"
)

// hostsFile is hosts.toml, re-read whenever its modification time or size
// changes. A missing file means no hosts. A file that stops parsing keeps the
// last good hosts until it is fixed, and its error is reported to the UI.
type hostsFile struct {
	path string

	mu      sync.Mutex
	modTime time.Time
	size    int64
	hosts   []hub.Host
	err     error // why the file on disk doesn't parse; nil when it does
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
	hosts, _ := f.State()
	return hosts
}

// State returns the current hosts and, if the file on disk doesn't parse,
// why (the hosts are then the last ones that did).
func (f *hostsFile) State() ([]hub.Host, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.reloadLocked(); err != nil {
		log.Printf("clawsh web: keeping previous hosts: %v", err)
	}
	if f.err != nil {
		return f.hosts, f.err.Error()
	}
	return f.hosts, ""
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

// reloadLocked re-reads the file if it changed. It returns a parse error only
// when it first sees it, so a broken file is logged once per change.
func (f *hostsFile) reloadLocked() error {
	st, err := os.Stat(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		f.hosts, f.modTime, f.size, f.err = nil, time.Time{}, 0, nil
		return nil
	}
	if err != nil {
		return err
	}
	// Comparing the size too catches a second write within one mtime tick,
	// e.g. an editor that truncates and then writes.
	if st.ModTime().Equal(f.modTime) && st.Size() == f.size {
		return nil
	}
	f.modTime, f.size = st.ModTime(), st.Size()
	cfg, err := hub.LoadConfig(f.path)
	if err != nil {
		f.err = err
		return err
	}
	f.hosts, f.err = cfg.Hosts, nil
	return nil
}
