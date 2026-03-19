package budget

import (
	"sync"
	"testing"
)

func TestBudgetTracker(t *testing.T) {
	tests := []struct {
		name         string
		maxTokens    int
		adds         [][2]int // pairs of [input, output] to add
		wantExceeded bool
		wantUsed     int
		wantMax      int
	}{
		{
			name:         "unlimited budget (maxTokens=0)",
			maxTokens:    0,
			adds:         [][2]int{{5000, 5000}},
			wantExceeded: false,
			wantUsed:     10000,
			wantMax:      0,
		},
		{
			name:         "within budget",
			maxTokens:    100,
			adds:         [][2]int{{30, 20}},
			wantExceeded: false,
			wantUsed:     50,
			wantMax:      100,
		},
		{
			name:         "exceeded budget",
			maxTokens:    100,
			adds:         [][2]int{{50, 30}, {20, 10}},
			wantExceeded: true,
			wantUsed:     110,
			wantMax:      100,
		},
		{
			name:         "exactly at limit",
			maxTokens:    100,
			adds:         [][2]int{{60, 40}},
			wantExceeded: true,
			wantUsed:     100,
			wantMax:      100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.maxTokens)

			for _, add := range tt.adds {
				b.Add(add[0], add[1])
			}

			if got := b.Exceeded(); got != tt.wantExceeded {
				t.Errorf("Exceeded() = %v, want %v", got, tt.wantExceeded)
			}

			if got := b.Used(); got != tt.wantUsed {
				t.Errorf("Used() = %v, want %v", got, tt.wantUsed)
			}

			if got := b.Max(); got != tt.wantMax {
				t.Errorf("Max() = %v, want %v", got, tt.wantMax)
			}
		})
	}
}

func TestBudgetTracker_ConcurrentSafety(t *testing.T) {
	b := New(1000000)

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Add(100, 100)
		}()
	}

	wg.Wait()

	wantUsed := numGoroutines * 200 // 100 * (100 + 100)
	if got := b.Used(); got != wantUsed {
		t.Errorf("Used() after concurrent adds = %v, want %v", got, wantUsed)
	}
}
