package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestLiteLLMRegression_CostHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   float64
	}{
		{name: "missing"},
		{name: "invalid", header: "n/a"},
		{name: "negative", header: "-0.25"},
		{name: "nan", header: "NaN"},
		{name: "positive infinity", header: "+Inf"},
		{name: "negative infinity", header: "-Inf"},
		{name: "overflow", header: "1e1000"},
		{name: "zero", header: "0"},
		{name: "positive", header: "0.125", want: 0.125},
		{name: "whitespace positive", header: " \t0.25 \t", want: 0.25},
		{name: "whitespace zero", header: " \t0 \t"},
	}
	for _, tt := range tests {
		for _, streaming := range []bool{false, true} {
			mode := "Send"
			if streaming {
				mode = "SendStream"
			}
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tt.header != "" {
						w.Header().Set("x-litellm-response-cost", tt.header)
					}
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, "data: [DONE]\n\n")
						return
					}
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
				}))
				defer server.Close()
				p, err := NewLiteLLM("key", server.URL)
				if err != nil {
					t.Fatal(err)
				}
				req := &Request{Model: "model"}
				if !streaming {
					resp, err := p.Send(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					if resp.Cost != tt.want {
						t.Errorf("cost = %v, want %v", resp.Cost, tt.want)
					}
					return
				}
				stream, err := p.SendStream(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = stream.Close() }()
				want := []StreamEvent{{Type: StreamEventDone, Cost: tt.want}}
				if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
					t.Errorf("events = %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestLiteLLMRegression_StreamUsageCost(t *testing.T) {
	tests := []struct {
		name   string
		header string
		cost   string
		want   float64
	}{
		{name: "missing header", cost: `,"cost":0.125`, want: 0.125},
		{name: "zero header", header: "0", cost: `,"cost":0.125`, want: 0.125},
		{name: "invalid header", header: "n/a", cost: `,"cost":0.125`, want: 0.125},
		{name: "negative header", header: "-1", cost: `,"cost":0.125`, want: 0.125},
		{name: "nan header", header: "NaN", cost: `,"cost":0.125`, want: 0.125},
		{name: "positive infinity header", header: "+Inf", cost: `,"cost":0.125`, want: 0.125},
		{name: "negative infinity header", header: "-Inf", cost: `,"cost":0.125`, want: 0.125},
		{name: "overflow header", header: "1e1000", cost: `,"cost":0.125`, want: 0.125},
		{name: "positive header wins", header: "0.5", cost: `,"cost":0.125`, want: 0.5},
		{name: "positive header and malformed usage", header: "0.5", cost: `,"cost":"n/a"`, want: 0.5},
		{name: "absent usage cost"},
		{name: "null usage cost", cost: `,"cost":null`},
		{name: "zero usage cost", cost: `,"cost":0`},
		{name: "negative usage cost", cost: `,"cost":-0.125`},
		{name: "invalid string", cost: `,"cost":"n/a"`},
		{name: "numeric string", cost: `,"cost":"0.125"`},
		{name: "nan string", cost: `,"cost":"NaN"`},
		{name: "infinity string", cost: `,"cost":"+Inf"`},
		{name: "overflow number", cost: `,"cost":1e1000`},
		{name: "overflow string", cost: `,"cost":"1e1000"`},
		{name: "object", cost: `,"cost":{"amount":0.125}`},
	}
	for _, tt := range tests {
		for _, withChoices := range []bool{false, true} {
			layout := "usage only"
			choices := `[]`
			if withChoices {
				layout = "usage with choices"
				choices = `[{"delta":{}}]`
			}
			t.Run(tt.name+"/"+layout, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					if tt.header != "" {
						w.Header().Set("x-litellm-response-cost", tt.header)
					}
					_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"+
						"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
						`data: {"choices":`+choices+`,"usage":{"prompt_tokens":12,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":3}`+tt.cost+"}}\n\ndata: [DONE]\n\n")
				}))
				defer server.Close()
				p, err := NewLiteLLM("key", server.URL)
				if err != nil {
					t.Fatal(err)
				}
				stream, err := p.SendStream(context.Background(), &Request{Model: "model"})
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = stream.Close() }()
				want := []StreamEvent{
					{Type: StreamEventText, Text: "hello"},
					{Type: StreamEventDone, InputTokens: 12, OutputTokens: 5, CacheReadTokens: 3, StopReason: "stop", Cost: tt.want},
				}
				if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
					t.Errorf("events = %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestOpenAIRegression_StreamIgnoresMalformedUsageCost(t *testing.T) {
	for _, tt := range []struct {
		name string
		cost string
	}{
		{name: "invalid string", cost: `"n/a"`},
		{name: "numeric string", cost: `"0.125"`},
		{name: "overflow", cost: `1e1000`},
		{name: "object", cost: `{"amount":0.125}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"+
					"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
					`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":5,"cost":`+tt.cost+"}}\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			p, err := NewOpenAI("key", WithOpenAIBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := p.SendStream(context.Background(), &Request{Model: "model"})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = stream.Close() }()
			want := []StreamEvent{
				{Type: StreamEventText, Text: "ok"},
				{Type: StreamEventDone, InputTokens: 12, OutputTokens: 5, StopReason: "stop"},
			}
			if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
				t.Errorf("events = %+v, want %+v", got, want)
			}
		})
	}
}

func TestOpenAIRegression_StreamMixedContentAndFinish(t *testing.T) {
	providers := []struct {
		name string
		new  func(string) (StreamProvider, error)
		cost float64
	}{
		{name: "OpenAI", new: func(url string) (StreamProvider, error) { return NewOpenAI("key", WithOpenAIBaseURL(url)) }},
		{name: "LiteLLM", new: func(url string) (StreamProvider, error) { return NewLiteLLM("key", url) }, cost: 0.125},
	}
	cases := []struct {
		name   string
		chunks string
		want   []StreamEvent
		stop   string
	}{
		{
			name:   "text and stop",
			chunks: `data: {"choices":[{"delta":{"content":"hello"},"finish_reason":"stop"}]}` + "\n\n",
			want:   []StreamEvent{{Type: StreamEventText, Text: "hello"}},
			stop:   "stop",
		},
		{
			name:   "text and whole single tool",
			chunks: `data: {"choices":[{"delta":{"content":"checking","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n",
			want: []StreamEvent{
				{Type: StreamEventText, Text: "checking"},
				{Type: StreamEventToolStart, ToolCallID: "call_a", ToolName: "read_file"},
				{Type: StreamEventToolDelta, ToolCallID: "call_a", ToolInput: `{"path":"a"}`},
				{Type: StreamEventToolEnd, ToolCallID: "call_a"},
			},
			stop: "tool_calls",
		},
		{
			name: "text and whole parallel tools",
			chunks: `data: {"choices":[{"delta":{"content":"checking","tool_calls":[` +
				`{"index":0,"id":"call_a","function":{"name":"read_file","arguments":"{\"path\":\"a\"}"}},` +
				`{"index":1,"id":"call_b","function":{"name":"write_file","arguments":"{\"path\":\"b\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n",
			want: []StreamEvent{
				{Type: StreamEventText, Text: "checking"},
				{Type: StreamEventToolStart, ToolCallID: "call_a", ToolName: "read_file"},
				{Type: StreamEventToolDelta, ToolCallID: "call_a", ToolInput: `{"path":"a"}`},
				{Type: StreamEventToolStart, ToolCallID: "call_b", ToolName: "write_file"},
				{Type: StreamEventToolDelta, ToolCallID: "call_b", ToolInput: `{"path":"b"}`},
				{Type: StreamEventToolEnd, ToolCallID: "call_a"},
				{Type: StreamEventToolEnd, ToolCallID: "call_b"},
			},
			stop: "tool_calls",
		},
		{
			name: "text and chunked parallel tools",
			chunks: `data: {"choices":[{"delta":{"content":"checking","tool_calls":[` +
				`{"index":0,"id":"call_a","function":{"name":"read_file","arguments":"{\"path\":"}},` +
				`{"index":1,"id":"call_b","function":{"name":"write_file","arguments":"{\"path\":"}}]}}]}` + "\n\n" +
				`data: {"choices":[{"delta":{"content":" now","tool_calls":[` +
				`{"index":1,"function":{"arguments":"\"b\"}"}},` +
				`{"index":0,"function":{"arguments":"\"a\"}"}}]},"finish_reason":"tool_calls"}]}` + "\n\n",
			want: []StreamEvent{
				{Type: StreamEventText, Text: "checking"},
				{Type: StreamEventToolStart, ToolCallID: "call_a", ToolName: "read_file"},
				{Type: StreamEventToolDelta, ToolCallID: "call_a", ToolInput: `{"path":`},
				{Type: StreamEventToolStart, ToolCallID: "call_b", ToolName: "write_file"},
				{Type: StreamEventToolDelta, ToolCallID: "call_b", ToolInput: `{"path":`},
				{Type: StreamEventText, Text: " now"},
				{Type: StreamEventToolDelta, ToolCallID: "call_b", ToolInput: `"b"}`},
				{Type: StreamEventToolDelta, ToolCallID: "call_a", ToolInput: `"a"}`},
				{Type: StreamEventToolEnd, ToolCallID: "call_a"},
				{Type: StreamEventToolEnd, ToolCallID: "call_b"},
			},
			stop: "tool_calls",
		},
	}
	for _, prov := range providers {
		for _, tt := range cases {
			for _, layout := range []string{"usage only", "usage with choices", "usage on final choice"} {
				t.Run(prov.name+"/"+tt.name+"/"+layout, func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "text/event-stream")
						const usage = `"usage":{"prompt_tokens":12,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":3},"cost":0.125}`
						body := tt.chunks
						switch layout {
						case "usage only":
							body += `data: {"choices":[],` + usage + "}\n\n"
						case "usage with choices":
							body += `data: {"choices":[{"delta":{}}],` + usage + "}\n\n"
						case "usage on final choice":
							body = strings.TrimSuffix(body, "}\n\n") + "," + usage + "}\n\n"
						}
						_, _ = io.WriteString(w, body+"data: [DONE]\n\n")
					}))
					defer server.Close()
					p, err := prov.new(server.URL)
					if err != nil {
						t.Fatal(err)
					}
					stream, err := p.SendStream(context.Background(), &Request{Model: "model"})
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = stream.Close() }()
					want := append(append([]StreamEvent(nil), tt.want...), StreamEvent{
						Type: StreamEventDone, InputTokens: 12, OutputTokens: 5, CacheReadTokens: 3, StopReason: tt.stop, Cost: prov.cost,
					})
					if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
						t.Errorf("events =\n%+v\nwant\n%+v", got, want)
					}
				})
			}
		}
	}
}
