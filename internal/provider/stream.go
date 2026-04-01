package provider

import (
	"context"
	"io"
	"sync"
)

const (
	StreamEventText      = "text"
	StreamEventToolStart = "tool_start"
	StreamEventToolDelta = "tool_delta"
	StreamEventToolEnd   = "tool_end"
	StreamEventDone      = "done"
)

type StreamEvent struct {
	Type         string
	Text         string
	ToolCallID   string
	ToolName     string
	ToolInput    string
	InputTokens  int
	OutputTokens int
	StopReason   string
}

type EventStream struct {
	nextFunc func() (StreamEvent, error)
	body     io.Closer
	closed   bool
	mu       sync.Mutex
}

func NewEventStream(body io.Closer, nextFunc func() (StreamEvent, error)) *EventStream {
	return &EventStream{
		body:     body,
		nextFunc: nextFunc,
	}
}

func (s *EventStream) Next() (StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return StreamEvent{}, io.EOF
	}

	return s.nextFunc()
}

func (s *EventStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	return s.body.Close()
}

type StreamProvider interface {
	Provider
	SendStream(ctx context.Context, req *Request) (*EventStream, error)
}
