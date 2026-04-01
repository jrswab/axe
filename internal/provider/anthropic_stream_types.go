package provider

type anthropicStreamEvent struct {
	Type         string                       `json:"type"`
	Message      *anthropicStreamMessage       `json:"message,omitempty"`
	Index        int                          `json:"index,omitempty"`
	ContentBlock *anthropicStreamContentBlock  `json:"content_block,omitempty"`
	Delta        *anthropicStreamDelta         `json:"delta,omitempty"`
	Usage        *anthropicStreamUsage         `json:"usage,omitempty"`
	Error        *anthropicStreamError         `json:"error,omitempty"`
}

type anthropicStreamMessage struct {
	ID    string                `json:"id,omitempty"`
	Usage *anthropicStreamUsage `json:"usage,omitempty"`
}

type anthropicStreamUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type anthropicStreamContentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

type anthropicStreamDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type anthropicStreamError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
