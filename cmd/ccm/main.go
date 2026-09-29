// ccm — manage Claude Code sessions across machines.
//
//	ccm agent                     run on every machine that hosts sessions
//	ccm token                     print this machine's agent token
//	ccm hosts                     check which configured agents are reachable
//	ccm ls [host]                 list sessions on all (or one) hosts
//	ccm new <host> [flags] [-- claude args...]
//	ccm attach <host>/<id>        attach; Ctrl-] detaches
//	ccm kill <host>/<id>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"ccm/internal/agent"
	"ccm/internal/api"
	"ccm/internal/hub"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "agent":
		err = runAgent(args)
	case "token":
		err = runToken(args)
	case "hosts":
		err = runHosts(args)
	case "ls", "list":
		err = runList(args)
	case "new":
		err = runNew(args)
	case "attach", "a":
		err = runAttach(args)
	case "kill", "rm":
		err = runKill(args)
	case "version":
		fmt.Println("ccm", api.Version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ccm:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: ccm <command> [args]

agent side (run on each machine):
  agent [--listen :7420] [--claude claude] [--token-file path]
  token [--token-file path]

hub side (run anywhere; reads hosts.toml):
  hosts                              reachability of every configured agent
  ls [host]                          list sessions
  new <host> [--dir d] [--name n] [--detached] [-- claude args...]
  attach <host>/<id>                 attach (id prefix ok); Ctrl-] detaches
  kill <host>/<id>                   stop and remove a session

hub commands accept --config (default: `+hub.DefaultConfigPath()+`)
`)
}

// ---------- agent side ----------

func runAgent(args []string) error {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	listen := fs.String("listen", ":7420", "address to listen on")
	claude := fs.String("claude", "claude", "claude executable (name on PATH or full path)")
	tokenFile := fs.String("token-file", agent.DefaultTokenPath(), "bearer token file (created on first run)")
	scrollback := fs.Int("scrollback", 2<<20, "bytes of output kept per session for replay")
	_ = fs.Parse(args)

	token, created, err := agent.LoadOrCreateToken(*tokenFile)
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}
	if created {
		log.Printf("generated new token in %s", *tokenFile)
		log.Printf("add it to hosts.toml on your hub machine:  token = %q", token)
	}

	m := agent.NewManager(agent.Options{Command: *claude, Scrollback: *scrollback})
	srv := &http.Server{
		Addr:              *listen,
		Handler:           agent.NewServer(m, token).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Print("shutting down: stopping sessions")
		m.Shutdown()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	log.Printf("ccm agent %s listening on %s (claude: %s)", api.Version, *listen, *claude)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runToken(args []string) error {
	fs := flag.NewFlagSet("token", flag.ExitOnError)
	tokenFile := fs.String("token-file", agent.DefaultTokenPath(), "bearer token file")
	_ = fs.Parse(args)
	token, _, err := agent.LoadOrCreateToken(*tokenFile)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

// ---------- hub side ----------

func hubFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cfg := fs.String("config", hub.DefaultConfigPath(), "hosts config file")
	return fs, cfg
}

// parseInterspersed lets flags appear before or after positional args.
// Everything after a literal "--" is returned as passthrough.
func parseInterspersed(fs *flag.FlagSet, args []string) (pos, passthrough []string) {
	for {
		_ = fs.Parse(args)
		rest := fs.Args()
		consumed := len(args) - len(rest)
		if consumed > 0 && args[consumed-1] == "--" {
			return pos, rest
		}
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func splitTarget(t string) (host, id string, err error) {
	host, id, ok := strings.Cut(t, "/")
	if !ok || host == "" || id == "" {
		return "", "", fmt.Errorf("target must be <host>/<id>, got %q", t)
	}
	return host, id, nil
}

func runHosts(args []string) error {
	fs, cfgPath := hubFlags("hosts")
	parseInterspersed(fs, args)
	cfg, err := hub.LoadConfig(*cfgPath)
	if err != nil {
		return err
	}
	type row struct{ name, url, status string }
	rows := make([]row, len(cfg.Hosts))
	var wg sync.WaitGroup
	for i, h := range cfg.Hosts {
		wg.Add(1)
		go func(i int, h hub.Host) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			c := hub.NewClient(h)
			status := "ok"
			if hl, err := c.Health(ctx); err != nil {
				status = "unreachable: " + err.Error()
			} else if _, err := c.List(ctx); err != nil {
				status = "auth failed: " + err.Error()
			} else {
				status = fmt.Sprintf("ok (%s, %s, v%s)", hl.Host, hl.OS, hl.Version)
			}
			rows[i] = row{h.Name, h.URL, status}
		}(i, h)
	}
	wg.Wait()
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HOST\tURL\tSTATUS")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.name, r.url, r.status)
	}
	return tw.Flush()
}

func runList(args []string) error {
	fs, cfgPath := hubFlags("ls")
	pos, _ := parseInterspersed(fs, args)
	cfg, err := hub.LoadConfig(*cfgPath)
	if err != nil {
		return err
	}
	hosts := cfg.Hosts
	if len(pos) > 0 {
		h, err := cfg.Find(pos[0])
		if err != nil {
			return err
		}
		hosts = []hub.Host{h}
	}

	type result struct {
		host     string
		sessions []api.Session
		err      error
	}
	results := make([]result, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h hub.Host) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			s, err := hub.NewClient(h).List(ctx)
			results[i] = result{h.Name, s, err}
		}(i, h)
	}
	wg.Wait()
	sort.SliceStable(results, func(i, j int) bool { return results[i].host < results[j].host })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tNAME\tSTATUS\tVIEWERS\tAGE\tDIR")
	var failed []result
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, r)
			continue
		}
		for _, s := range r.sessions {
			status := string(s.Status)
			if s.ExitCode != nil {
				status = fmt.Sprintf("exited(%d)", *s.ExitCode)
			}
			fmt.Fprintf(tw, "%s/%s\t%s\t%s\t%d\t%s\t%s\n",
				r.host, s.ID, s.Name, status, s.Viewers, age(s.Created), s.Dir)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, r := range failed {
		fmt.Fprintf(os.Stderr, "! %s unreachable: %v\n", r.host, r.err)
	}
	return nil
}

func runNew(args []string) error {
	fs, cfgPath := hubFlags("new")
	dir := fs.String("dir", "", "working directory on the remote machine (default: remote home)")
	name := fs.String("name", "", "session name (default: dir basename)")
	detached := fs.Bool("detached", false, "don't attach after creating")
	pos, claudeArgs := parseInterspersed(fs, args)
	if len(pos) != 1 {
		return errors.New("usage: ccm new <host> [--dir d] [--name n] [--detached] [-- claude args...]")
	}
	cfg, err := hub.LoadConfig(*cfgPath)
	if err != nil {
		return err
	}
	h, err := cfg.Find(pos[0])
	if err != nil {
		return err
	}
	req := api.CreateRequest{Name: *name, Dir: *dir, Args: claudeArgs}
	if cols, rows, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		req.Cols, req.Rows = cols, rows
	}
	c := hub.NewClient(h)
	s, err := c.Create(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("created %s/%s in %s\n", h.Name, s.ID, s.Dir)
	if *detached {
		return nil
	}
	return hub.Attach(context.Background(), c, s.ID)
}

func runAttach(args []string) error {
	fs, cfgPath := hubFlags("attach")
	pos, _ := parseInterspersed(fs, args)
	if len(pos) != 1 {
		return errors.New("usage: ccm attach <host>/<id>")
	}
	hostName, id, err := splitTarget(pos[0])
	if err != nil {
		return err
	}
	cfg, err := hub.LoadConfig(*cfgPath)
	if err != nil {
		return err
	}
	h, err := cfg.Find(hostName)
	if err != nil {
		return err
	}
	return hub.Attach(context.Background(), hub.NewClient(h), id)
}

func runKill(args []string) error {
	fs, cfgPath := hubFlags("kill")
	pos, _ := parseInterspersed(fs, args)
	if len(pos) != 1 {
		return errors.New("usage: ccm kill <host>/<id>")
	}
	hostName, id, err := splitTarget(pos[0])
	if err != nil {
		return err
	}
	cfg, err := hub.LoadConfig(*cfgPath)
	if err != nil {
		return err
	}
	h, err := cfg.Find(hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hub.NewClient(h).Kill(ctx, id); err != nil {
		return err
	}
	fmt.Printf("killed %s/%s\n", hostName, id)
	return nil
}

func age(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return d.String()
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
