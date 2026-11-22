package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	bedrockWirePath       = "/model/wire-model/converse"
	bedrockWireReply      = `{"output":{"message":{"role":"assistant","content":[{"text":"Done"}]}},"stopReason":"end_turn"`
	bedrockWireSystem     = `{"text":"Static instructions"}`
	bedrockWireCheckpoint = `{"cachePoint":{"type":"default"}}`
	bedrockWireTools      = `{"toolSpec":{"name":"z_lookup","description":"Look up a record","inputSchema":{"json":{
		"type":"object","properties":{"group":{"type":"string","description":"Record group"},"id":{"type":"string","description":"Record ID"},"limit":{"type":"integer","description":"Optional limit"}},"required":["group","id"]}}}},
		{"toolSpec":{"name":"a_notify","description":"Send a notification","inputSchema":{"json":{
		"type":"object","properties":{"quiet":{"type":"boolean","description":"Suppress sound"}}}}}}`
	bedrockWireInitial = `[{"role":"user","content":[{"text":"First task"}]}]`
)

// Wire oracles: milestone 002 R1/R3 and the AWS Converse contracts:
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_SystemContentBlock.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_Tool.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolConfiguration.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolSpecification.html
// https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolInputSchema.html
// This local fixture shows wire intent, not live acceptance or cache eligibility.
func TestBedrock_StaticWireContract(t *testing.T) {
	for _, tt := range []struct {
		name          string
		system, tools bool
	}{
		{name: "system-only", system: true},
		{name: "tools-only", tools: true},
		{name: "system-and-tools", system: true, tools: true},
		{name: "empty-static"},
	} {
		for _, enabled := range []bool{true, false} {
			name := fmt.Sprintf("%s/cache=%t", tt.name, enabled)
			if t.Run(name, func(t *testing.T) {
				b, capture, _ := newBedrockWireFixture(t, bedrockWireResponse(http.StatusOK, bedrockWireReply+`}`))
				req := bedrockWireRequest(tt.system, tt.tools, enabled, false)
				if _, err := b.Send(bedrockWireContext(t), req); err != nil {
					t.Fatalf("Send: %v", err)
				}

				want := map[string]string{"messages": bedrockWireInitial, "inferenceConfig": `{"temperature":0.25,"maxTokens":64}`}
				checkpoint := ""
				if enabled {
					checkpoint = "," + bedrockWireCheckpoint
				}
				if tt.system {
					want["system"] = "[" + bedrockWireSystem + checkpoint + "]"
				}
				if tt.tools {
					want["toolConfig"] = `{"tools":[` + bedrockWireTools + checkpoint + `]}`
				}
				assertBedrockWireFields(t, capture.requests(t, 1)[0], want)
			}) {
				t.Logf("wire matrix passed: %s", name)
			}
		}
	}
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("stable-static/cache=%t", enabled), func(t *testing.T) {
			checkBedrockWireStability(t, enabled)
		})
	}
	if t.Run("usage", checkBedrockWireUsage) {
		t.Log("all usage preservation cases passed")
	}
	if t.Run("failures", checkBedrockWireFailures) {
		t.Log("all failure and no-replay preservation cases passed")
	}
}

func bedrockWireRequest(system, tools, enabled, reverse bool) *Request {
	req := &Request{Model: "wire-model", Messages: []Message{{Role: "user", Content: "First task"}},
		CacheConfig: enabled, Temperature: 0.25, MaxTokens: 64}
	if system {
		req.System = "Static instructions"
	}
	if tools {
		params := make(map[string]ToolParameter)
		group := ToolParameter{Type: "string", Description: "Record group", Required: true}
		id := ToolParameter{Type: "string", Description: "Record ID", Required: true}
		limit := ToolParameter{Type: "integer", Description: "Optional limit"}
		if reverse {
			params["id"], params["limit"], params["group"] = id, limit, group
		} else {
			params["group"], params["limit"], params["id"] = group, limit, id
		}
		req.Tools = []Tool{
			{Name: "z_lookup", Description: "Look up a record", Parameters: params},
			{Name: "a_notify", Description: "Send a notification", Parameters: map[string]ToolParameter{
				"quiet": {Type: "boolean", Description: "Suppress sound"},
			}},
		}
	}
	return req
}

func checkBedrockWireStability(t *testing.T, enabled bool) {
	t.Helper()
	b, capture, _ := newBedrockWireFixture(t, bedrockWireResponse(http.StatusOK, bedrockWireReply+`}`))
	turns := []struct {
		more []Message
		want string
	}{
		{want: bedrockWireInitial},
		{
			more: []Message{
				{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Name: "z_lookup", Arguments: map[string]string{"group": "A", "id": "7"}}}},
				{Role: "user", ToolResults: []ToolResult{{CallID: "call_1", Content: "Found seven"}}},
				{Role: "user", Content: "Second task\nContext: notify only if found"},
				{Role: "assistant", Content: "Notifying", ToolCalls: []ToolCall{{ID: "call_2", Name: "a_notify", Arguments: map[string]string{}}}},
				{Role: "user", ToolResults: []ToolResult{{CallID: "call_2", Content: "Already notified", IsError: true}}},
			},
			want: `[{"role":"user","content":[{"text":"First task"}]},
			{"role":"assistant","content":[{"toolUse":{"toolUseId":"call_1","name":"z_lookup","input":{"group":"A","id":"7"}}}]},
			{"role":"user","content":[{"toolResult":{"toolUseId":"call_1","content":[{"text":"Found seven"}],"status":"success"}}]},
			{"role":"user","content":[{"text":"Second task\nContext: notify only if found"}]},
			{"role":"assistant","content":[{"text":"Notifying"},{"toolUse":{"toolUseId":"call_2","name":"a_notify","input":{}}}]},
			{"role":"user","content":[{"toolResult":{"toolUseId":"call_2","content":[{"text":"Already notified"}],"status":"error"}}]}]`,
		},
	}
	const repeats = 8
	ctx := bedrockWireContext(t)
	for _, turn := range turns {
		for i := range repeats {
			req := bedrockWireRequest(true, true, enabled, i%2 == 1)
			req.Messages = append(req.Messages, turn.more...)
			if _, err := b.Send(ctx, req); err != nil {
				t.Fatalf("Send: %v", err)
			}
		}
	}

	count := repeats * len(turns)
	bodies := capture.requests(t, count)[:count]
	first := decodeBedrockWire(t, bodies[0])
	changed := make(map[string]int)
	for i, body := range bodies {
		wire := decodeBedrockWire(t, body)
		assertBedrockWireJSON(t, fmt.Sprintf("messages on request %d", i+1), wire["messages"], turns[i/repeats].want)
		for _, field := range []string{"system", "toolConfig"} {
			if !bytes.Equal(wire[field], first[field]) && changed[field] == 0 {
				changed[field] = i + 1
			}
		}
	}
	for _, field := range []string{"system", "toolConfig"} {
		if attempt := changed[field]; attempt != 0 {
			t.Errorf("serialized %s differs between request 1 and request %d with identical static input", field, attempt)
		}
	}
}

func checkBedrockWireUsage(t *testing.T) {
	for _, tt := range []struct {
		name, tail string
		want       [4]int // input, output, cache read, cache write
	}{
		{"reported", `,"usage":{"inputTokens":101,"outputTokens":17,"cacheReadInputTokens":29,"cacheWriteInputTokens":53}}`, [4]int{101, 17, 29, 53}},
		{"zero-read", `,"usage":{"inputTokens":31,"outputTokens":7,"cacheReadInputTokens":0,"cacheWriteInputTokens":19}}`, [4]int{31, 7, 0, 19}},
		{"zero-write", `,"usage":{"inputTokens":43,"outputTokens":11,"cacheReadInputTokens":13,"cacheWriteInputTokens":0}}`, [4]int{43, 11, 13, 0}},
		{"omitted-cache-counts", `,"usage":{"inputTokens":23,"outputTokens":3}}`, [4]int{23, 3, 0, 0}},
		{"omitted-usage", `}`, [4]int{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, capture, _ := newBedrockWireFixture(t, bedrockWireResponse(http.StatusOK, bedrockWireReply+tt.tail))
			got, err := b.Send(bedrockWireContext(t), bedrockWireRequest(false, false, true, false))

			if err != nil || got == nil {
				t.Fatalf("Send = %+v, %v; want a successful response", got, err)
			}
			if got.Model != "wire-model" || got.Content != "Done" || got.StopReason != "end_turn" {
				t.Errorf("response = %+v; want wire-model, Done, end_turn", got)
			}
			counts := [4]int{got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens}
			if counts != tt.want {
				t.Errorf("usage [input output read write] = %v, want %v", counts, tt.want)
			}
			capture.requests(t, 1)
		})
	}
}

func checkBedrockWireFailures(t *testing.T) {
	for _, tt := range []struct {
		name, body, message string
		status              int
		category            ErrorCategory
		cause               error
	}{
		{name: "401", status: 401, body: `{"message":"Missing signature"}`, category: ErrCategoryAuth, message: "Missing signature"},
		{name: "403", status: 403, body: `{"message":"Access denied"}`, category: ErrCategoryAuth, message: "Access denied"},
		{name: "400-validation", status: 400, body: `{"type":"ValidationException","message":"Invalid request: check cache settings or model arguments"}`, category: ErrCategoryBadRequest, message: "Invalid request: check cache settings or model arguments"},
		{name: "429", status: 429, body: `{"message":"Slow down"}`, category: ErrCategoryRateLimit, message: "Slow down"},
		{name: "500", status: 500, body: `{"message":"Internal inference error"}`, category: ErrCategoryServer, message: "Internal inference error"},
		{name: "cancelled-after-request", category: ErrCategoryTimeout, message: "context canceled", cause: context.Canceled},
		{name: "accepted-then-lost", category: ErrCategoryServer, cause: io.EOF},
		{name: "response-read", message: "failed to read response body: unexpected EOF", cause: io.ErrUnexpectedEOF},
		{name: "malformed-response", body: `{"output":`, category: ErrCategoryServer, message: "failed to parse response: unexpected end of JSON input"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(bedrockWireContext(t))
			defer cancel()
			handler := bedrockWireResponse(tt.status, tt.body)
			switch tt.name {
			case "cancelled-after-request":
				handler = func(_ http.ResponseWriter, r *http.Request) {
					cancel() // Only after the fixture has read the original request.
					select {
					case <-r.Context().Done():
					case <-time.After(5 * time.Second):
						t.Error("local HTTP request did not observe cancellation")
					}
				}
			case "accepted-then-lost":
				handler = func(w http.ResponseWriter, _ *http.Request) {
					conn, _, err := http.NewResponseController(w).Hijack()
					if err != nil {
						t.Errorf("drop local connection: %v", err)
						return
					}
					_ = conn.Close()
				}
			case "response-read":
				handler = func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Length", "1000")
					_, _ = io.WriteString(w, `{"output":`)
				}
			case "malformed-response":
				handler = bedrockWireResponse(http.StatusOK, tt.body)
			}
			b, capture, endpoint := newBedrockWireFixture(t, handler)
			message := tt.message
			if tt.name == "accepted-then-lost" {
				message = `Post ` + strconv.Quote(endpoint+bedrockWirePath) + `: EOF`
			}
			resp, err := b.Send(ctx, bedrockWireRequest(true, true, true, false))

			assertBedrockWireNoReplay(t, capture.requests(t, 1))
			if resp != nil || err == nil {
				t.Fatalf("Send = %+v, %v; want nil response and an error", resp, err)
			}
			var pe *ProviderError
			if tt.category == "" {
				if errors.As(err, &pe) || err.Error() != message || errors.Unwrap(err) != tt.cause {
					t.Errorf("read error = %T %v (cause %T %v); want plain %q wrapping %v", err, err, errors.Unwrap(err), errors.Unwrap(err), message, tt.cause)
				}
				return
			}
			if !errors.As(err, &pe) {
				t.Fatalf("error = %T %v, want ProviderError", err, err)
			}
			if pe.Category != tt.category || pe.Status != tt.status || pe.Message != message || err.Error() != string(tt.category)+": "+message {
				t.Errorf("error = %q (status %d, category %q, message %q); want status %d, category %q, message %q", err, pe.Status, pe.Category, pe.Message, tt.status, tt.category, message)
			}
			cause := errors.Unwrap(pe)
			switch tt.name {
			case "accepted-then-lost":
				var transport *url.Error
				if !errors.As(cause, &transport) || !errors.Is(err, tt.cause) {
					t.Errorf("cause = %T %v, want transport error wrapping %v", cause, cause, tt.cause)
				}
			case "malformed-response":
				var syntax *json.SyntaxError
				if !errors.As(cause, &syntax) {
					t.Errorf("cause = %T %v, want JSON syntax error", cause, cause)
				}
			default: // Cancellation retains its cause; HTTP status errors have none.
				if cause != tt.cause {
					t.Errorf("cause = %T %v, want %v", cause, cause, tt.cause)
				}
			}
		})
	}
}

// The matrix checks exact placement. Here, count the original default hints on
// every attempt, even if a regression replays with hints removed or changed.
func assertBedrockWireNoReplay(t *testing.T, bodies [][]byte) {
	t.Helper()
	for i, body := range bodies {
		wire := decodeBedrockWire(t, body)
		for _, field := range []string{"system", "toolConfig"} {
			var compact bytes.Buffer
			if err := json.Compact(&compact, wire[field]); err != nil {
				t.Errorf("attempt %d %s is missing or invalid: %v", i+1, field, err)
				continue
			}
			if found := strings.Count(compact.String(), `"cachePoint":{"type":"default"}`); found != 1 {
				t.Errorf("attempt %d %s has %d default cache hints, want one", i+1, field, found)
			}
		}
		if i > 0 && !bytes.Equal(body, bodies[0]) {
			t.Errorf("unexpected replay %d changed the request body", i+1)
		}
	}
}

func assertBedrockWireFields(t *testing.T, body []byte, want map[string]string) {
	t.Helper()
	wire := decodeBedrockWire(t, body)
	for _, field := range []string{"system", "toolConfig", "messages", "inferenceConfig"} {
		if expected, ok := want[field]; ok {
			assertBedrockWireJSON(t, field, wire[field], expected)
		}
	}
	for field, value := range wire {
		if _, ok := want[field]; !ok {
			t.Errorf("unexpected %s field: %s", field, value)
		}
	}
}

func assertBedrockWireJSON(t *testing.T, field string, actual []byte, expected string) {
	t.Helper()
	var got, want any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("invalid fixture JSON for %s: %v", field, err)
	}
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Errorf("missing or invalid %s JSON: %q (%v)", field, actual, err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		var compact bytes.Buffer
		_ = json.Compact(&compact, []byte(expected))
		t.Errorf("%s = %s; want %s", field, actual, compact.String())
	}
}

func decodeBedrockWire(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("invalid request JSON: %v", err)
	}
	return wire
}

func bedrockWireContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func bedrockWireResponse(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

type bedrockWireCapture struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (c *bedrockWireCapture) requests(t *testing.T, count int) [][]byte {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bodies) != count {
		t.Errorf("request attempts = %d, want %d (no replay)", len(c.bodies), count)
		if len(c.bodies) < count {
			t.FailNow()
		}
	}
	return append([][]byte(nil), c.bodies...)
}

func newBedrockWireFixture(t *testing.T, reply http.HandlerFunc) (*Bedrock, *bedrockWireCapture, string) {
	t.Helper()
	capture := &bedrockWireCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != bedrockWirePath {
			t.Errorf("endpoint = %s %s, want POST %s", r.Method, r.URL.Path, bedrockWirePath)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read local request: %v", err)
			http.Error(w, "fixture could not read request", http.StatusInternalServerError)
			return
		}
		capture.mu.Lock()
		capture.bodies = append(capture.bodies, body)
		capture.mu.Unlock()
		reply(w, r)
	}))
	t.Cleanup(server.Close)
	b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	if err != nil {
		t.Fatalf("NewBedrock: %v", err)
	}
	return b, capture, server.URL
}
