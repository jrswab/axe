package tool

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/resolve"
	"github.com/jrswab/axe/internal/xdg"
)

// CallAgentToolName is the constant name for the sub-agent invocation tool.
const CallAgentToolName = "call_agent"

// maxConversationTurns is the safety limit for the conversation loop.
const maxConversationTurns = 50

// ExecuteOptions holds configuration for executing a call_agent tool call.
type ExecuteOptions struct {
	AllowedAgents []string
	ParentModel   string
	Depth         int
	MaxDepth      int
	Timeout       int
	GlobalConfig  *config.GlobalConfig
	Verbose       bool
	Stderr        io.Writer
}

// CallAgentTool returns the call_agent tool definition for LLM tool calling.
// The allowedAgents list is included in the description and parameter descriptions
// so the LLM knows which agent names are valid.
func CallAgentTool(allowedAgents []string) provider.Tool {
	agentList := strings.Join(allowedAgents, ", ")

	return provider.Tool{
		Name:        CallAgentToolName,
		Description: "Delegate a task to a sub-agent. The sub-agent runs independently with its own context and returns only its final result. Available agents: " + agentList,
		Parameters: map[string]provider.ToolParameter{
			"agent": {
				Type:        "string",
				Description: "Name of the sub-agent to invoke (must be one of: " + agentList + ")",
				Required:    true,
			},
			"task": {
				Type:        "string",
				Description: "What you need the sub-agent to do",
				Required:    true,
			},
			"context": {
				Type:        "string",
				Description: "Additional context from your conversation to pass along",
				Required:    false,
			},
		},
	}
}

// ExecuteCallAgent executes a call_agent tool call by loading and running a sub-agent.
// It always returns a ToolResult (never an error). Errors are communicated via
// ToolResult.Content and ToolResult.IsError fields.
func ExecuteCallAgent(ctx context.Context, call provider.ToolCall, opts ExecuteOptions) provider.ToolResult {
	// Step 1: Extract arguments
	agentName := call.Arguments["agent"]
	task := call.Arguments["task"]
	taskContext := call.Arguments["context"]

	// Step 2: Validate agent name
	if agentName == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: `call_agent error: "agent" argument is required`,
			IsError: true,
		}
	}

	// Step 3: Validate task
	if task == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: `call_agent error: "task" argument is required`,
			IsError: true,
		}
	}

	// Step 4: Validate agent is allowed
	allowed := false
	for _, a := range opts.AllowedAgents {
		if a == agentName {
			allowed = true
			break
		}
	}
	if !allowed {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("call_agent error: agent %q is not in this agent's sub_agents list", agentName),
			IsError: true,
		}
	}

	// Step 5: Check depth limit
	if opts.Depth >= opts.MaxDepth {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("call_agent error: maximum sub-agent depth (%d) reached", opts.MaxDepth),
			IsError: true,
		}
	}

	// Verbose: log before sub-agent call
	if opts.Verbose && opts.Stderr != nil {
		taskPreview := task
		if len(taskPreview) > 80 {
			taskPreview = taskPreview[:80] + "..."
		}
		fmt.Fprintf(opts.Stderr, "[sub-agent] Calling %q (depth %d) with task: %s\n", agentName, opts.Depth+1, taskPreview)
	}

	start := time.Now()

	// Step 6: Load sub-agent config
	cfg, err := agent.Load(agentName)
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("failed to load agent %q: %s", agentName, err), opts)
	}

	// Step 7: Parse sub-agent's model
	provName, modelName, err := parseModel(cfg.Model)
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("invalid model for agent %q: %s", agentName, err), opts)
	}

	// Step 8: Resolve sub-agent's working directory, files, skill, system prompt
	workdir := resolve.Workdir("", cfg.Workdir)

	files, err := resolve.Files(cfg.Files, workdir)
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("failed to resolve files for agent %q: %s", agentName, err), opts)
	}

	configDir, err := xdg.GetConfigDir()
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("failed to get config dir: %s", err), opts)
	}

	skillContent, err := resolve.Skill(cfg.Skill, configDir)
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("failed to load skill for agent %q: %s", agentName, err), opts)
	}

	systemPrompt := resolve.BuildSystemPrompt(cfg.SystemPrompt, skillContent, files)

	// Step 8b: Memory — load entries into system prompt
	if cfg.Memory.Enabled {
		memPath, memErr := memory.FilePath(agentName, cfg.Memory.Path)
		if memErr != nil {
			if opts.Verbose && opts.Stderr != nil {
				fmt.Fprintf(opts.Stderr, "[sub-agent] Warning: failed to load memory for %q: %v\n", agentName, memErr)
			}
		} else {
			entries, memErr := memory.LoadEntries(memPath, cfg.Memory.LastN)
			if memErr != nil {
				if opts.Verbose && opts.Stderr != nil {
					fmt.Fprintf(opts.Stderr, "[sub-agent] Warning: failed to load memory for %q: %v\n", agentName, memErr)
				}
			} else if entries != "" {
				systemPrompt += "\n\n---\n\n## Memory\n\n" + entries
			}
		}
	}

	// Step 9: Resolve API key and base URL
	globalCfg := opts.GlobalConfig
	if globalCfg == nil {
		globalCfg = &config.GlobalConfig{}
	}

	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)

	if provider.Supported(provName) && provName != "ollama" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return errorResult(call.ID, agentName, fmt.Sprintf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar), opts)
	}

	// Step 10: Create provider
	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return errorResult(call.ID, agentName, fmt.Sprintf("failed to create provider for agent %q: %s", agentName, err), opts)
	}

	// Step 11: Build user message
	var userMessage string
	if strings.TrimSpace(taskContext) != "" {
		userMessage = fmt.Sprintf("Task: %s\n\nContext:\n%s", task, taskContext)
	} else {
		userMessage = fmt.Sprintf("Task: %s", task)
	}

	// Step 12: Build request
	req := &provider.Request{
		Model:       modelName,
		System:      systemPrompt,
		Messages:    []provider.Message{{Role: "user", Content: userMessage}},
		Temperature: cfg.Params.Temperature,
		MaxTokens:   cfg.Params.MaxTokens,
	}

	// Inject tools if sub-agent has sub_agents and depth allows
	newDepth := opts.Depth + 1
	if len(cfg.SubAgents) > 0 && newDepth < opts.MaxDepth {
		req.Tools = []provider.Tool{CallAgentTool(cfg.SubAgents)}
	}

	// Step 13: Create timeout context
	var callCtx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
	} else {
		callCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Step 14: Run conversation loop (or single-shot if no tools)
	resp, err := runConversationLoop(callCtx, prov, req, cfg, newDepth, opts)
	if err != nil {
		durationMs := time.Since(start).Milliseconds()
		if opts.Verbose && opts.Stderr != nil {
			fmt.Fprintf(opts.Stderr, "[sub-agent] %q failed: %s\n", agentName, err)
		}
		_ = durationMs
		return errorResult(call.ID, agentName, err.Error(), opts)
	}

	// Step 14b: Memory — append entry after successful response
	if cfg.Memory.Enabled {
		appendPath, appendErr := memory.FilePath(agentName, cfg.Memory.Path)
		if appendErr != nil {
			if opts.Verbose && opts.Stderr != nil {
				fmt.Fprintf(opts.Stderr, "[sub-agent] Warning: failed to save memory for %q: %v\n", agentName, appendErr)
			}
		} else {
			if appendErr = memory.AppendEntry(appendPath, userMessage, resp.Content); appendErr != nil {
				if opts.Verbose && opts.Stderr != nil {
					fmt.Fprintf(opts.Stderr, "[sub-agent] Warning: failed to save memory for %q: %v\n", agentName, appendErr)
				}
			}
		}
	}

	// Step 15: Return result
	durationMs := time.Since(start).Milliseconds()
	if opts.Verbose && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "[sub-agent] %q completed in %dms (%d chars returned)\n", agentName, durationMs, len(resp.Content))
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: resp.Content,
		IsError: false,
	}
}

// runConversationLoop runs the multi-turn conversation loop for a sub-agent.
// If the sub-agent has no tools, this is a single-shot call.
func runConversationLoop(ctx context.Context, prov provider.Provider, req *provider.Request, cfg *agent.AgentConfig, depth int, opts ExecuteOptions) (*provider.Response, error) {
	for turn := 0; turn < maxConversationTurns; turn++ {
		resp, err := prov.Send(ctx, req)
		if err != nil {
			return nil, err
		}

		// No tool calls: we're done
		if len(resp.ToolCalls) == 0 {
			return resp, nil
		}

		// If no tools were sent, treat as done even if response has tool calls
		if len(req.Tools) == 0 {
			return resp, nil
		}

		// Append assistant message with tool calls
		assistantMsg := provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		req.Messages = append(req.Messages, assistantMsg)

		// Execute tool calls and collect results
		results := make([]provider.ToolResult, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			if tc.Name == CallAgentToolName {
				subOpts := ExecuteOptions{
					AllowedAgents: cfg.SubAgents,
					ParentModel:   cfg.Model,
					Depth:         depth,
					MaxDepth:      opts.MaxDepth,
					Timeout:       opts.Timeout,
					GlobalConfig:  opts.GlobalConfig,
					Verbose:       opts.Verbose,
					Stderr:        opts.Stderr,
				}
				results[i] = ExecuteCallAgent(ctx, tc, subOpts)
			} else {
				results[i] = provider.ToolResult{
					CallID:  tc.ID,
					Content: fmt.Sprintf("Unknown tool: %q", tc.Name),
					IsError: true,
				}
			}
		}

		// Append tool result message
		toolMsg := provider.Message{
			Role:        "tool",
			ToolResults: results,
		}
		req.Messages = append(req.Messages, toolMsg)
	}

	return nil, fmt.Errorf("sub-agent exceeded maximum conversation turns (%d)", maxConversationTurns)
}

// errorResult creates an error ToolResult for a sub-agent failure.
func errorResult(callID, agentName, errMsg string, opts ExecuteOptions) provider.ToolResult {
	if opts.Verbose && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "[sub-agent] %q failed: %s\n", agentName, errMsg)
	}
	return provider.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Error: sub-agent %q failed - %s. You may retry or proceed without this result.", agentName, errMsg),
		IsError: true,
	}
}

// parseModel splits a "provider/model-name" string into provider and model parts.
func parseModel(model string) (providerName, modelName string, err error) {
	idx := strings.Index(model, "/")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid model format %q: expected provider/model-name", model)
	}

	providerName = model[:idx]
	modelName = model[idx+1:]

	if providerName == "" {
		return "", "", fmt.Errorf("invalid model format %q: empty provider", model)
	}
	if modelName == "" {
		return "", "", fmt.Errorf("invalid model format %q: empty model name", model)
	}

	return providerName, modelName, nil
}
