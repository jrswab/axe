package tool

import (
	"context"
	"fmt"
	"io"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// ExecContext holds the minimal context needed by generic tool executors.
type ExecContext struct {
	Workdir         string
	Stderr          io.Writer
	Verbose         bool
	AllowedHosts    []string
	ArtifactDir     string
	ArtifactTracker *artifact.Tracker
}

// ToolEntry holds a tool's definition and executor functions.
type ToolEntry struct {
	Definition func() provider.Tool
	Execute    func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult
}

// Registry maps tool names to their definitions and executors.
// Registration must happen at startup before any concurrent Dispatch calls.
// Concurrent Dispatch calls (read-only) are safe after registration is complete.
type Registry struct {
	entries map[string]ToolEntry
}

// NewRegistry returns a new Registry with an empty, initialized entries map.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]ToolEntry),
	}
}

// Register adds a tool entry to the registry, keyed by name.
// If a tool with the same name already exists, it is silently replaced.
func (r *Registry) Register(name string, entry ToolEntry) {
	r.entries[name] = entry
}

// Has returns true if a tool with the given name exists in the registry.
func (r *Registry) Has(name string) bool {
	_, ok := r.entries[name]
	return ok
}

// Resolve takes a list of tool names and returns their provider.Tool definitions.
// Each entry's Definition() is called on every Resolve invocation (not cached).
// Returns an error if any name is not found in the registry or has a nil Definition.
// Returns an empty non-nil slice for nil or empty input.
func (r *Registry) Resolve(names []string) ([]provider.Tool, error) {
	if len(names) == 0 {
		return []provider.Tool{}, nil
	}

	tools := make([]provider.Tool, 0, len(names))
	for _, name := range names {
		entry, ok := r.entries[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", name)
		}
		if entry.Definition == nil {
			return nil, fmt.Errorf("tool %q has no definition", name)
		}
		tools = append(tools, entry.Definition())
	}
	return tools, nil
}

// Dispatch looks up the tool by call.Name and executes it.
// Returns (ToolResult, nil) on success, or (zero ToolResult, error) for
// unknown tools or tools with nil executors.
func (r *Registry) Dispatch(ctx context.Context, call provider.ToolCall, ec ExecContext) (provider.ToolResult, error) {
	entry, ok := r.entries[call.Name]
	if !ok {
		return provider.ToolResult{}, fmt.Errorf("unknown tool %q", call.Name)
	}
	if entry.Execute == nil {
		return provider.ToolResult{}, fmt.Errorf("tool %q has no executor", call.Name)
	}
	return entry.Execute(ctx, call, ec), nil
}

// RegisterAll registers all implemented tool entries into the given registry.
// Both cmd/run.go and ExecuteCallAgent call this after NewRegistry().
// Future milestones add lines here — no call-site changes needed.
// Safe to call multiple times (Register silently replaces duplicates).
func RegisterAll(r *Registry) {
	r.Register(toolname.ListDirectory, listDirectoryEntry())
	r.Register(toolname.ReadFile, readFileEntry())
	r.Register(toolname.WriteFile, writeFileEntry())
	r.Register(toolname.EditFile, editFileEntry())
	r.Register(toolname.RunCommand, runCommandEntry())
	r.Register(toolname.URLFetch, urlFetchEntry())
	r.Register(toolname.WebSearch, webSearchEntry())
}
