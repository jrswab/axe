package artifact

import (
	"sync"
	"testing"
)

func TestTracker(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() *Tracker
		verify func(t *testing.T, tr *Tracker)
	}{
		{
			name:  "NewTracker returns non-nil with empty entries",
			setup: func() *Tracker { return NewTracker() },
			verify: func(t *testing.T, tracker *Tracker) {
				if tracker == nil {
					t.Fatal("NewTracker() returned nil")
				}

				entries := tracker.Entries()
				if len(entries) != 0 {
					t.Errorf("Expected empty entries, got %d entries", len(entries))
				}
			},
		},
		{
			name: "Record appends entry",
			setup: func() *Tracker {
				tracker := NewTracker()
				entry := Entry{
					Path:  "/path/to/file.txt",
					Agent: "test-agent",
					Size:  1024,
				}
				tracker.Record(entry)
				return tracker
			},
			verify: func(t *testing.T, tracker *Tracker) {
				entry := Entry{
					Path:  "/path/to/file.txt",
					Agent: "test-agent",
					Size:  1024,
				}

				entries := tracker.Entries()
				if len(entries) != 1 {
					t.Errorf("Expected 1 entry, got %d", len(entries))
				}

				if entries[0].Path != entry.Path {
					t.Errorf("Expected Path %q, got %q", entry.Path, entries[0].Path)
				}

				if entries[0].Agent != entry.Agent {
					t.Errorf("Expected Agent %q, got %q", entry.Agent, entries[0].Agent)
				}

				if entries[0].Size != entry.Size {
					t.Errorf("Expected Size %d, got %d", entry.Size, entries[0].Size)
				}
			},
		},
		{
			name: "Entries returns a copy",
			setup: func() *Tracker {
				tracker := NewTracker()
				entry := Entry{
					Path:  "/path/to/file.txt",
					Agent: "test-agent",
					Size:  1024,
				}
				tracker.Record(entry)
				return tracker
			},
			verify: func(t *testing.T, tracker *Tracker) {
				entries := tracker.Entries()
				if len(entries) != 1 {
					t.Fatalf("Expected 1 entry, got %d", len(entries))
				}

				entries[0].Path = "/modified/path.txt"

				entries2 := tracker.Entries()
				if entries2[0].Path != "/path/to/file.txt" {
					t.Errorf("Entries() did not return a copy - original was modified")
				}

				entries = append(entries, Entry{Path: "/another.txt"})
				if len(entries) != 2 {
					t.Errorf("Expected 2 entries in local slice after append, got %d", len(entries))
				}

				entries3 := tracker.Entries()
				if len(entries3) != 1 {
					t.Errorf("Appending to returned slice affected tracker: expected 1 entry, got %d", len(entries3))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := tc.setup()
			tc.verify(t, tr)
		})
	}

	t.Run("Concurrent writes are safe", func(t *testing.T) {
		tracker := NewTracker()
		numGoroutines := 100
		entriesPerGoroutine := 10

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < entriesPerGoroutine; j++ {
					tracker.Record(Entry{
						Path:  "/path/file.txt",
						Agent: "agent",
						Size:  int64(id*entriesPerGoroutine + j),
					})
				}
			}(i)
		}

		wg.Wait()

		entries := tracker.Entries()
		expectedCount := numGoroutines * entriesPerGoroutine
		if len(entries) != expectedCount {
			t.Errorf("Expected %d entries after concurrent writes, got %d", expectedCount, len(entries))
		}

		seen := make(map[int64]bool)
		for _, e := range entries {
			if seen[e.Size] {
				t.Errorf("Duplicate entry with Size %d found", e.Size)
			}
			seen[e.Size] = true
		}

		if len(seen) != expectedCount {
			t.Errorf("Expected %d unique sizes, got %d", expectedCount, len(seen))
		}
	})
}
