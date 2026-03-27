package artifact

import "sync"

type Entry struct {
	Path  string
	Agent string
	Size  int64
}

type Tracker struct {
	mu      sync.Mutex
	entries []Entry
}

func NewTracker() *Tracker {
	return &Tracker{
		entries: make([]Entry, 0),
	}
}

func (t *Tracker) Record(entry Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, entry)
}

func (t *Tracker) Entries() []Entry {
	t.mu.Lock()
	defer t.mu.Unlock()

	copy := make([]Entry, len(t.entries))
	for i, e := range t.entries {
		copy[i] = e
	}
	return copy
}
