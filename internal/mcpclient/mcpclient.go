package mcpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/envinterp"
	"github.com/jrswab/axe/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client wraps an MCP session for one configured server.
type Client struct {
	name    string
	session *mcp.ClientSession
	closeFn func() error
}

// Connect initializes an MCP client session for one server config.
func Connect(ctx context.Context, cfg agent.MCPServerConfig) (*Client, error) {
	headers, err := envinterp.ExpandHeaders(cfg.Headers)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Transport: &headerRoundTripper{base: http.DefaultTransport, headers: headers}}

	var transport mcp.Transport
	switch cfg.Transport {
	case "sse":
		transport = &mcp.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient}
	case "streamable-http":
		transport = &mcp.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient}
	default:
		return nil, fmt.Errorf("unsupported MCP transport %q", cfg.Transport)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "axe", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect MCP server %q: %w", cfg.Name, err)
	}

	return &Client{name: cfg.Name, session: session}, nil
}

// Name returns the configured server name.
func (c *Client) Name() string {
	return c.name
}

// Close closes the underlying MCP session.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.closeFn != nil {
		return c.closeFn()
	}
	if c.session == nil {
		return nil
	}
	return c.session.Close()
}

// ListTools returns MCP tools converted to provider tools.
func (c *Client) ListTools(ctx context.Context) ([]provider.Tool, error) {
	res, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	tools := make([]provider.Tool, 0, len(res.Tools))
	for _, tool := range res.Tools {
		tools = append(tools, convertTool(tool))
	}
	return tools, nil
}

// CallTool calls an MCP tool and converts the response into provider format.
func (c *Client) CallTool(ctx context.Context, call provider.ToolCall) (provider.ToolResult, error) {
	args := make(map[string]any, len(call.Arguments))
	for key, value := range call.Arguments {
		args[key] = value
	}

	res, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: call.Name, Arguments: args})
	if err != nil {
		return provider.ToolResult{}, err
	}

	textParts := make([]string, 0, len(res.Content))
	for _, content := range res.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		textParts = append(textParts, text.Text)
	}

	if len(textParts) == 0 {
		return provider.ToolResult{CallID: call.ID, IsError: true, Content: "MCP tool returned no text content"}, nil
	}

	return provider.ToolResult{CallID: call.ID, IsError: res.IsError, Content: strings.Join(textParts, "\n")}, nil
}

func convertTool(tool *mcp.Tool) provider.Tool {
	out := provider.Tool{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  map[string]provider.ToolParameter{},
	}

	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		return out
	}

	required := map[string]bool{}
	if reqList, ok := schema["required"].([]any); ok {
		for _, item := range reqList {
			if name, ok := item.(string); ok {
				required[name] = true
			}
		}
	} else if reqList, ok := schema["required"].([]string); ok {
		for _, name := range reqList {
			required[name] = true
		}
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return out
	}

	for name, raw := range properties {
		param := provider.ToolParameter{Type: "string", Required: required[name]}
		property, ok := raw.(map[string]any)
		if !ok {
			param.Description = "Type unknown"
			out.Parameters[name] = param
			continue
		}

		desc, _ := property["description"].(string)
		if typeName, ok := property["type"].(string); ok {
			switch typeName {
			case "string", "integer", "number", "boolean":
				param.Type = typeName
				param.Description = desc
			default:
				param.Description = prependTypeDescription(typeName, desc)
			}
		} else {
			param.Description = prependTypeDescription("unknown", desc)
		}

		out.Parameters[name] = param
	}

	return out
}

func prependTypeDescription(typeName, desc string) string {
	if desc == "" {
		return fmt.Sprintf("Type %s", typeName)
	}
	return fmt.Sprintf("Type %s: %s", typeName, desc)
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, value := range h.headers {
		clone.Header.Set(key, value)
	}

	base := h.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}
