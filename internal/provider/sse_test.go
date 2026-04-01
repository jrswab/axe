package provider

import (
	"io"
	"strings"
	"testing"
)

func TestSSEParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   []SSEEvent
	}{
		{
			name:  "single data event",
			input: "data: hello world\n\n",
			want:  []SSEEvent{{Data: "hello world"}},
		},
		{
			name:  "multi-line data joined with newline",
			input: "data: line1\ndata: line2\ndata: line3\n\n",
			want:  []SSEEvent{{Data: "line1\nline2\nline3"}},
		},
		{
			name:  "event type field",
			input: "event: message_start\ndata: {}\n\n",
			want:  []SSEEvent{{Event: "message_start", Data: "{}"}},
		},
		{
			name:  "comment lines skipped",
			input: ": this is a comment\ndata: hello\n\n",
			want:  []SSEEvent{{Data: "hello"}},
		},
		{
			name:  "DONE sentinel passed through",
			input: "data: [DONE]\n\n",
			want:  []SSEEvent{{Data: "[DONE]"}},
		},
		{
			name:  "space after colon stripping",
			input: "data:hello\n\ndata: world\n\n",
			want:  []SSEEvent{{Data: "hello"}, {Data: "world"}},
		},
		{
			name:  "empty events between blank lines",
			input: "\n\ndata: first\n\n\n\ndata: second\n\n",
			want:  []SSEEvent{{Data: "first"}, {Data: "second"}},
		},
		{
			name:  "id and retry fields ignored",
			input: "id: 123\nretry: 5000\ndata: payload\n\n",
			want:  []SSEEvent{{Data: "payload"}},
		},
		{
			name:  "multiple events",
			input: "data: first\n\ndata: second\n\ndata: third\n\n",
			want:  []SSEEvent{{Data: "first"}, {Data: "second"}, {Data: "third"}},
		},
		{
			name:  "event without trailing blank line at EOF",
			input: "data: last",
			want:  []SSEEvent{{Data: "last"}},
		},
		{
			name:  "event type with data",
			input: "event: content_block_delta\ndata: {\"delta\":\"hi\"}\n\n",
			want:  []SSEEvent{{Event: "content_block_delta", Data: "{\"delta\":\"hi\"}"}},
		},
		{
			name:  "only comments produces no events",
			input: ": comment1\n: comment2\n\n",
			want:  nil,
		},
		{
			name:  "empty data field",
			input: "data:\n\n",
			want:  []SSEEvent{{Data: ""}},
		},
		{
			name:  "data with no space after colon",
			input: "data:no-space\n\n",
			want:  []SSEEvent{{Data: "no-space"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewSSEParser(strings.NewReader(tt.input))
			var got []SSEEvent

			for {
				event, err := parser.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got = append(got, event)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d events, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].Event != tt.want[i].Event {
					t.Errorf("event %d: Event got %q, want %q", i, got[i].Event, tt.want[i].Event)
				}
				if got[i].Data != tt.want[i].Data {
					t.Errorf("event %d: Data got %q, want %q", i, got[i].Data, tt.want[i].Data)
				}
			}
		})
	}
}

func TestSSEParser_EOF(t *testing.T) {
	t.Parallel()

	parser := NewSSEParser(strings.NewReader(""))
	_, err := parser.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF for empty reader, got %v", err)
	}
}
