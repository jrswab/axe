package mcpclient

import (
	"context"
	"fmt"

	"github.com/jrswab/axe/internal/provider"
)

// Router routes MCP tool calls to owning clients.
type Router struct {
	tools   map[string]*Client
	clients []*Client
}

// NewRouter returns a new empty MCP tool router.
func NewRouter() *Router {
	return &Router{tools: map[string]*Client{}}
}

// Register adds tool routes for a client and returns tools not skipped.
func (r *Router) Register(client *Client, tools []provider.Tool, builtinNames map[string]bool) ([]provider.Tool, error) {
	r.clients = append(r.clients, client)
	filtered := make([]provider.Tool, 0, len(tools))
	for _, tool := range tools {
		if builtinNames[tool.Name] {
			continue
		}

		if existing, ok := r.tools[tool.Name]; ok && existing != client {
			return nil, fmt.Errorf("MCP tool %q collision between servers %q and %q", tool.Name, existing.Name(), client.Name())
		}

		r.tools[tool.Name] = client
		filtered = append(filtered, tool)
	}

	return filtered, nil
}

// Dispatch routes a tool call to the correct MCP client.
func (r *Router) Dispatch(ctx context.Context, call provider.ToolCall) (provider.ToolResult, error) {
	client, ok := r.tools[call.Name]
	if !ok {
		return provider.ToolResult{}, fmt.Errorf("unknown MCP tool %q", call.Name)
	}
	return client.CallTool(ctx, call)
}

// Has reports whether the router has a route for the given tool name.
func (r *Router) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// ServerName returns the MCP server name that owns a tool.
func (r *Router) ServerName(name string) (string, bool) {
	client, ok := r.tools[name]
	if !ok {
		return "", false
	}
	return client.Name(), true
}

// Close closes each unique client once.
func (r *Router) Close() error {
	closed := map[*Client]struct{}{}
	var firstErr error

	for _, client := range r.clients {
		if _, seen := closed[client]; seen {
			continue
		}
		closed[client] = struct{}{}
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
