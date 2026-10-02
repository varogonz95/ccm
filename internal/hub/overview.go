package hub

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"ccm/internal/api"
)

// OverviewTimeout bounds how long one host may take to answer.
const OverviewTimeout = 3 * time.Second

// ErrMsgAuth is HostOverview.Error when the agent rejects the token.
const ErrMsgAuth = "access key rejected"

// Overview asks every host for its health and sessions in parallel. The
// result has one entry per host, in the order given. A host that fails is
// reported offline with the reason and never delays the others beyond
// OverviewTimeout.
func Overview(ctx context.Context, hosts []Host) []api.HostOverview {
	out := make([]api.HostOverview, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h Host) {
			defer wg.Done()
			out[i] = overviewOne(ctx, h)
		}(i, h)
	}
	wg.Wait()
	return out
}

func overviewOne(ctx context.Context, h Host) api.HostOverview {
	ctx, cancel := context.WithTimeout(ctx, OverviewTimeout)
	defer cancel()
	o := api.HostOverview{Name: h.Name, URL: h.URL, Sessions: []api.Session{}}
	c := NewClient(h)
	health, err := c.Health(ctx)
	if err != nil {
		o.Error = err.Error()
		return o
	}
	o.Health = &health
	sessions, err := c.List(ctx)
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) && he.Code == http.StatusUnauthorized {
			o.Error = ErrMsgAuth
		} else {
			o.Error = err.Error()
		}
		return o
	}
	o.Online = true
	if sessions != nil {
		o.Sessions = sessions
	}
	return o
}
