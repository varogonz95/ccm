package agent

// Scrollback keeps the most recent max bytes of terminal output so a newly
// attached viewer can be brought up to date. Not safe for concurrent use;
// Session guards it with its own mutex.
type Scrollback struct {
	max int
	b   []byte
}

func NewScrollback(max int) *Scrollback { return &Scrollback{max: max} }

func (s *Scrollback) Write(p []byte) {
	s.b = append(s.b, p...)
	// Trim lazily (at 2x) so appends stay amortised O(1).
	if len(s.b) > 2*s.max {
		s.b = append([]byte(nil), s.b[len(s.b)-s.max:]...)
	}
}

func (s *Scrollback) Snapshot() []byte {
	b := s.b
	if len(b) > s.max {
		b = b[len(b)-s.max:]
	}
	return append([]byte(nil), b...)
}
