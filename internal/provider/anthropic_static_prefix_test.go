package provider

import (
	"encoding/json"
	"reflect"
	"testing"
)

type anthropicStaticPrefixRow struct {
	name   string
	system string
	tools  bool
}

type anthropicStaticPrefixTurn struct {
	name     string
	messages []Message
	want     string
}

type anthropicStaticPrefixMode struct {
	name    string
	enabled bool
}

func anthropicStaticPrefixModes() []anthropicStaticPrefixMode {
	return []anthropicStaticPrefixMode{{name: "enabled", enabled: true}, {name: "disabled"}}
}

func anthropicStaticPrefixRows() []anthropicStaticPrefixRow {
	return []anthropicStaticPrefixRow{
		{name: "system_and_tools", system: "Static instructions only.", tools: true},
		{name: "system_only", system: "Static instructions only."},
		{name: "tools_only", tools: true},
		{name: "neither"},
	}
}

func anthropicStaticPrefixTurns() []anthropicStaticPrefixTurn {
	return []anthropicStaticPrefixTurn{
		{
			name: "first",
			messages: []Message{
				{Role: "user", Content: "History: first record"},
				{Role: "assistant", Content: "First history acknowledged"},
				{Role: "assistant", Content: "Looking up first", ToolCalls: []ToolCall{{ID: "call_first", Name: "z_lookup", Arguments: map[string]string{"key": "first"}}}},
				{Role: "tool", ToolResults: []ToolResult{{CallID: "call_first", Content: "First lookup result"}}},
				{Role: "user", Content: "Task: report first\nContext: first private context"},
			},
			want: `[
				{"role":"user","content":"History: first record"},
				{"role":"assistant","content":"First history acknowledged"},
				{"role":"assistant","content":[{"type":"text","text":"Looking up first"},{"type":"tool_use","id":"call_first","name":"z_lookup","input":{"key":"first"}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_first","content":"First lookup result","is_error":false}]},
				{"role":"user","content":"Task: report first\nContext: first private context"}
			]`,
		},
		{
			name: "second",
			messages: []Message{
				{Role: "user", Content: "History: second record"},
				{Role: "assistant", Content: "Second history acknowledged"},
				{Role: "assistant", Content: "Reporting second", ToolCalls: []ToolCall{{ID: "call_second", Name: "a_report", Arguments: map[string]string{"record": "second"}}}},
				{Role: "tool", ToolResults: []ToolResult{{CallID: "call_second", Content: "Second report failed", IsError: true}}},
				{Role: "user", Content: "Task: retry second\nContext: second private context"},
			},
			want: `[
				{"role":"user","content":"History: second record"},
				{"role":"assistant","content":"Second history acknowledged"},
				{"role":"assistant","content":[{"type":"text","text":"Reporting second"},{"type":"tool_use","id":"call_second","name":"a_report","input":{"record":"second"}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_second","content":"Second report failed","is_error":true}]},
				{"role":"user","content":"Task: retry second\nContext: second private context"}
			]`,
		},
	}
}

func anthropicStaticPrefixDecode() func(*testing.T, []byte) interface{} {
	decode := func(t *testing.T, raw []byte) interface{} {
		t.Helper()
		var value interface{}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("invalid JSON %s: %v", raw, err)
		}
		return value
	}
	return decode
}

func anthropicStaticPrefixRequest(row anthropicStaticPrefixRow) *Request {
	// Deliberately nonalphabetical caller order; one required property per
	// tool keeps schema-map ordering (T3) outside this breakpoint slice.
	req := &Request{Model: "claude-sonnet-4-20250514", MaxTokens: 128, System: row.system}
	if row.tools {
		req.Tools = []Tool{
			{Name: "z_lookup", Description: "Look up a record", Parameters: map[string]ToolParameter{
				"key":     {Type: "string", Description: "Record key", Required: true},
				"verbose": {Type: "boolean", Description: "Include details"},
			}},
			{Name: "a_report", Description: "Report a record", Parameters: map[string]ToolParameter{
				"record": {Type: "string", Description: "Record to report", Required: true},
			}},
		}
	}
	return req
}

func assertAnthropicStaticPrefixWire(t *testing.T, got map[string]json.RawMessage, row anthropicStaticPrefixRow, mode anthropicStaticPrefixMode, turn anthropicStaticPrefixTurn, decode func(*testing.T, []byte) interface{}) {
	t.Helper()
	// R2/R5, docs/plans/053_corrected_caching_001_spec.md: the approved
	// https://platform.claude.com/docs/en/build-with-claude/prompt-caching
	// contract orders the prefix tools -> system -> messages. These literal
	// wire expectations do not use production serialization types or helpers.
	const plainTools = `[
		{"name":"z_lookup","description":"Look up a record","input_schema":{"type":"object","properties":{"key":{"type":"string","description":"Record key"},"verbose":{"type":"boolean","description":"Include details"}},"required":["key"]}},
		{"name":"a_report","description":"Report a record","input_schema":{"type":"object","properties":{"record":{"type":"string","description":"Record to report"}},"required":["record"]}}
	]`
	const cachedTools = `[
		{"name":"z_lookup","description":"Look up a record","input_schema":{"type":"object","properties":{"key":{"type":"string","description":"Record key"},"verbose":{"type":"boolean","description":"Include details"}},"required":["key"]}},
		{"name":"a_report","description":"Report a record","input_schema":{"type":"object","properties":{"record":{"type":"string","description":"Record to report"}},"required":["record"]},"cache_control":{"type":"ephemeral"}}
	]`
	wantSystem, wantTools := "", ""
	if row.system != "" {
		wantSystem = `"Static instructions only."`
		if mode.enabled {
			wantSystem = `[{"type":"text","text":"Static instructions only.","cache_control":{"type":"ephemeral"}}]`
		}
	}
	if row.tools {
		wantTools = plainTools
		if mode.enabled && row.system == "" {
			wantTools = cachedTools
		}
	}
	for _, field := range []struct{ name, want string }{
		{"system", wantSystem}, {"tools", wantTools}, {"messages", turn.want},
	} {
		raw, present := got[field.name]
		if field.want == "" {
			if present {
				t.Errorf("%s must be omitted, got %s", field.name, raw)
			}
		} else if !present || !reflect.DeepEqual(decode(t, raw), decode(t, []byte(field.want))) {
			t.Errorf("%s wire mismatch (only the last caller-ordered tool gets a hint for enabled tools-only):\ngot  %s\nwant %s", field.name, raw, field.want)
		}
		delete(got, field.name)
	}
}
