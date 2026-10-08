package hub

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/varogonz95/clawsh/internal/paths"
)

type Host struct {
	Name  string `toml:"name"`
	URL   string `toml:"url"`   // e.g. http://192.168.1.20:7420
	Token string `toml:"token"` // from `clawsh token` on that machine
}

type Config struct {
	Hosts []Host `toml:"host"`
}

func DefaultConfigPath() string {
	return filepath.Join(paths.ConfigDir(), "hosts.toml")
}

func LoadConfig(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("load %s: %w (see hosts.example.toml)", path, err)
	}
	seen := map[string]bool{}
	for i, h := range c.Hosts {
		if h.Name == "" || h.URL == "" {
			return nil, fmt.Errorf("%s: host #%d needs name and url", path, i+1)
		}
		if seen[h.Name] {
			return nil, fmt.Errorf("%s: duplicate host name %q", path, h.Name)
		}
		seen[h.Name] = true
		c.Hosts[i].URL = strings.TrimRight(h.URL, "/")
	}
	return &c, nil
}

func (c *Config) Find(name string) (Host, error) {
	for _, h := range c.Hosts {
		if h.Name == name {
			return h, nil
		}
	}
	return Host{}, fmt.Errorf("unknown host %q", name)
}
