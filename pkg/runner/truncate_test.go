package runner

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		truncated   bool
		wantLen     int
		wantHasTail bool
	}{
		{
			name:      "short unchanged",
			input:     strings.Repeat("a", 5),
			want:      strings.Repeat("a", 5),
			truncated: false,
			wantLen:   5,
		},
		{
			name:      "exactly max unchanged",
			input:     strings.Repeat("b", 1024),
			want:      strings.Repeat("b", 1024),
			truncated: false,
			wantLen:   1024,
		},
		{
			name:        "one over max truncated",
			input:       strings.Repeat("c", 1025),
			want:        strings.Repeat("c", 1024) + "... (truncated)",
			truncated:   true,
			wantLen:     1039,
			wantHasTail: true,
		},
		{
			name:        "long truncated",
			input:       strings.Repeat("d", 2048),
			want:        strings.Repeat("d", 1024) + "... (truncated)",
			truncated:   true,
			wantLen:     1039,
			wantHasTail: true,
		},
		{
			name:      "empty unchanged",
			input:     "",
			want:      "",
			truncated: false,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateOutput(tt.input)
			if got != tt.want {
				t.Fatalf("truncateOutput() mismatch: got len=%d want len=%d", len(got), len(tt.want))
			}
			if len(got) != tt.wantLen {
				t.Fatalf("truncateOutput() length mismatch: got=%d want=%d", len(got), tt.wantLen)
			}
			if tt.truncated != (len(tt.input) > 1024) {
				t.Fatalf("invalid test case: truncated flag must match input length")
			}
			if tt.wantHasTail && !strings.HasSuffix(got, "... (truncated)") {
				t.Fatalf("truncateOutput() missing truncation suffix")
			}
		})
	}
}

func TestTruncateOutputUTF8(t *testing.T) {
	const marker = "... (truncated)"

	// "é" is 2 bytes (0xC3 0xA9). Fill 1023 bytes of ASCII then append "é"
	// so the 2-byte rune straddles byte 1024. A naive slice at 1024 would
	// split the rune; the fix must backtrack to 1023.
	t.Run("2-byte rune at boundary", func(t *testing.T) {
		input := strings.Repeat("a", 1023) + "é" // 1025 bytes total
		got := truncateOutput(input)
		if !utf8.ValidString(got) {
			t.Fatalf("truncateOutput produced invalid UTF-8")
		}
		if !strings.HasSuffix(got, marker) {
			t.Fatalf("missing truncation marker")
		}
		body := strings.TrimSuffix(got, marker)
		if len(body) > maxToolOutputBytes {
			t.Fatalf("body exceeds max bytes: got %d", len(body))
		}
	})

	// "€" is 3 bytes (0xE2 0x82 0xAC). Fill 1022 ASCII bytes then "€"
	// so byte 1024 lands on the second continuation byte.
	t.Run("3-byte rune at boundary", func(t *testing.T) {
		input := strings.Repeat("a", 1022) + "€" + strings.Repeat("a", 10) // 1035 bytes
		got := truncateOutput(input)
		if !utf8.ValidString(got) {
			t.Fatalf("truncateOutput produced invalid UTF-8")
		}
		if !strings.HasSuffix(got, marker) {
			t.Fatalf("missing truncation marker")
		}
		body := strings.TrimSuffix(got, marker)
		if len(body) > maxToolOutputBytes {
			t.Fatalf("body exceeds max bytes: got %d", len(body))
		}
	})

	// "🔥" is 4 bytes (0xF0 0x9F 0x94 0xA5). Fill 1021 ASCII bytes then "🔥"
	// so the rune spans bytes 1021-1024.
	t.Run("4-byte rune at boundary", func(t *testing.T) {
		input := strings.Repeat("a", 1021) + "🔥" + strings.Repeat("a", 10) // 1035 bytes
		got := truncateOutput(input)
		if !utf8.ValidString(got) {
			t.Fatalf("truncateOutput produced invalid UTF-8")
		}
		if !strings.HasSuffix(got, marker) {
			t.Fatalf("missing truncation marker")
		}
		body := strings.TrimSuffix(got, marker)
		if len(body) > maxToolOutputBytes {
			t.Fatalf("body exceeds max bytes: got %d", len(body))
		}
	})

	// All multi-byte: 342 × "é" (2 bytes each) = 684 bytes, under limit.
	t.Run("all multibyte under limit unchanged", func(t *testing.T) {
		input := strings.Repeat("é", 342) // 684 bytes
		got := truncateOutput(input)
		if got != input {
			t.Fatalf("expected unchanged output for input under limit")
		}
	})
}
