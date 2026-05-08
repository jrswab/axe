package runner

import "encoding/json"

// ToolCallDetail captures metadata about a single tool invocation.
type ToolCallDetail struct {
	Name       string            `json:"name"`
	Input      map[string]string `json:"input"`
	Output     string            `json:"output"`
	IsError    bool              `json:"is_error"`
	Turn       int               `json:"turn"`
	DurationMs int64             `json:"duration_ms"`
}

// ArtifactInfo tracks a file written during the run.
type ArtifactInfo struct {
	Path  string `json:"path"`
	Agent string `json:"agent"`
	Size  int64  `json:"size"`
}

// BudgetState captures the token budget status.
type BudgetState struct {
	Max       int  `json:"max_tokens"`
	Used      int  `json:"used_tokens"`
	Exceeded  bool `json:"exceeded"`
}

// DryRunInfo holds resolved context for dry-run display.
type DryRunInfo struct {
	Model             string
	Workdir           string
	Timeout           int
	Params            string
	Budget            int
	Stream            bool
	SystemPrompt      string
	Skill             string
	Files             []string
	UserMessage       string
	Memory            string
	MemoryEnabled     bool
	Tools             []string
	MCPServers        []string
	SubAgents         []string
	MaxDepth          int
	Parallel          bool
	SubAgentTimeout   int
	MessageCount      int
}

// Result captures all outputs of a completed agent run.
type Result struct {
	Content          string           `json:"content"`
	Model            string           `json:"model"`
	InputTokens      int              `json:"input_tokens"`
	OutputTokens     int              `json:"output_tokens"`
	CacheReadTokens  int              `json:"cache_read_tokens"`
	CacheWriteTokens int              `json:"cache_write_tokens"`
	Cost             float64          `json:"cost"`
	StopReason       string           `json:"stop_reason"`
	CacheStatus      string           `json:"cache_status"`
	ToolCalls        int              `json:"tool_calls"`
	ToolCallDetails  []ToolCallDetail `json:"tool_call_details"`
	DurationMs       int64            `json:"duration_ms"`
	Refused          bool             `json:"refused"`
	RetryAttempts    int              `json:"retry_attempts"`
	Budget           BudgetState      `json:"budget"`
	Artifacts        []ArtifactInfo   `json:"artifacts"`
	DryRun           bool             `json:"dry_run"`
	DryRunInfo       *DryRunInfo      `json:"-"`
	Messages         []Message        `json:"messages"`
}

// MarshalJSON produces JSON output that matches the legacy CLI format:
// fields are alphabetically ordered (via map serialization), budget fields
// are flattened and only included when a budget is configured, artifacts are
// only included when non-empty, and dry_run is omitted.
// serializeMessages converts a slice of Message to JSON-compatible maps,
// ensuring empty tool_calls and tool_results are present as [] rather than null.
func serializeMessages(msgs []Message) []map[string]interface{} {
	out := make([]map[string]interface{}, len(msgs))
	for i, m := range msgs {
		entry := map[string]interface{}{
			"role":         m.Role,
			"content":      m.Content,
			"tool_calls":   make([]map[string]interface{}, 0),
			"tool_results": make([]map[string]interface{}, 0),
		}
		if len(m.ToolCalls) > 0 {
			tcs := make([]map[string]interface{}, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				tcs[j] = map[string]interface{}{
					"id":        tc.ID,
					"name":      tc.Name,
					"arguments": tc.Arguments,
				}
			}
			entry["tool_calls"] = tcs
		}
		if len(m.ToolResults) > 0 {
			trs := make([]map[string]interface{}, len(m.ToolResults))
			for j, tr := range m.ToolResults {
				trs[j] = map[string]interface{}{
					"call_id":  tr.CallID,
					"content":  tr.Content,
					"is_error": tr.IsError,
				}
			}
			entry["tool_results"] = trs
		}
		out[i] = entry
	}
	return out
}

func (r Result) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})

	m["content"] = r.Content
	if r.Cost != 0 {
		m["cost"] = r.Cost
	}
	if r.CacheStatus != "" {
		m["cache_status"] = r.CacheStatus
	}
	if r.CacheReadTokens != 0 {
		m["cache_read_tokens"] = r.CacheReadTokens
	}
	if r.CacheWriteTokens != 0 {
		m["cache_write_tokens"] = r.CacheWriteTokens
	}
	m["duration_ms"] = r.DurationMs
	m["input_tokens"] = r.InputTokens
	m["model"] = r.Model
	m["output_tokens"] = r.OutputTokens
	m["refused"] = r.Refused
	m["retry_attempts"] = r.RetryAttempts
	m["stop_reason"] = r.StopReason
	if r.ToolCallDetails == nil {
		m["tool_call_details"] = make([]ToolCallDetail, 0)
	} else {
		m["tool_call_details"] = r.ToolCallDetails
	}
	m["tool_calls"] = r.ToolCalls

	if r.Budget.Max > 0 {
		m["budget_exceeded"] = r.Budget.Exceeded
		m["budget_max_tokens"] = r.Budget.Max
		m["budget_used_tokens"] = r.Budget.Used
	}

	if len(r.Artifacts) > 0 {
		m["artifacts"] = r.Artifacts
	}

	if len(r.Messages) > 0 {
		m["messages"] = serializeMessages(r.Messages)
	}

	return json.Marshal(m)
}
