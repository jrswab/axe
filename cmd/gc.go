package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/config"
	"github.com/jrswab/axe/internal/memory"
	"github.com/jrswab/axe/internal/provider"
	"github.com/spf13/cobra"
)

// gcPatternPrompt is the hard-coded system prompt for pattern detection (Req 4.1, 4.2).
const gcPatternPrompt = `You are a memory analyst for an AI agent. You will receive a log of the agent's past tasks and results. Analyze the entries and provide a structured report.

Your report MUST contain exactly these three sections with these exact headings:

## Patterns Found
Identify recurring themes, common task types, or behavioral patterns across the entries. If no patterns exist, state "No clear patterns detected."

## Repeated Work
Identify any tasks that appear to be duplicated or that the agent has done multiple times with the same or similar inputs. If no repetition is found, state "No repeated work detected."

## Recommendations
Based on the patterns and repetitions found, suggest concrete actions the user could take to improve the agent's configuration, skill, or workflow. If no recommendations apply, state "No specific recommendations."

Be concise. Reference specific entries by their timestamps when relevant.`

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Analyze and trim agent memory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGC,
}

func init() {
	gcCmd.Flags().Bool("dry-run", false, "Analyze and print suggestions without trimming the memory file")
	gcCmd.Flags().Bool("all", false, "Run GC on all agents that have memory.enabled = true")
	gcCmd.Flags().String("model", "", "Override the model used for pattern detection (provider/model-name format)")
	rootCmd.AddCommand(gcCmd)
}

func runGC(cmd *cobra.Command, args []string) error {
	allFlag, _ := cmd.Flags().GetBool("all")

	// Argument validation
	if allFlag && len(args) > 0 {
		return &ExitError{Code: 1, Err: fmt.Errorf("cannot specify both --all and an agent name")}
	}
	if !allFlag && len(args) == 0 {
		return &ExitError{Code: 1, Err: fmt.Errorf("agent name is required (or use --all)")}
	}

	if allFlag {
		// TODO: implement --all flow in Phase 6
		return nil
	}

	// Single-agent GC flow
	agentName := args[0]
	return runSingleAgentGC(cmd, agentName)
}

func runSingleAgentGC(cmd *cobra.Command, agentName string) error {
	// Step 1: Load agent config (Req 3.1)
	cfg, err := agent.Load(agentName)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 2: Check if memory is enabled (Req 3.2)
	if !cfg.Memory.Enabled {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: agent %q does not have memory enabled. Skipping.\n", agentName)
		return nil
	}

	// Step 3: Resolve memory file path (Req 3.4)
	memPath, err := memory.FilePath(agentName, cfg.Memory.Path)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	// Step 4: Load all entries (Req 3.5)
	entries, err := memory.LoadEntries(memPath, 0)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	if entries == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "No memory entries for agent %q. Nothing to do.\n", agentName)
		return nil
	}

	// Step 5: Count and display entries (Req 3.6)
	count, err := memory.CountEntries(memPath)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Agent: %s\nEntries: %d\n", agentName, count)

	// Step 6: Determine model (Req 3.3)
	modelFlag, _ := cmd.Flags().GetString("model")
	modelStr := cfg.Model
	if modelFlag != "" {
		modelStr = modelFlag
	}

	provName, modelName, err := parseModel(modelStr)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// Step 7: Load global config and resolve API key (Req 3.8)
	globalCfg, err := config.Load()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}

	apiKey := globalCfg.ResolveAPIKey(provName)
	baseURL := globalCfg.ResolveBaseURL(provName)

	if provider.Supported(provName) && provName != "ollama" && apiKey == "" {
		envVar := config.APIKeyEnvVar(provName)
		return &ExitError{Code: 3, Err: fmt.Errorf("API key for provider %q is not configured (set %s or add to config.toml)", provName, envVar)}
	}

	// Step 8: Create provider (Req 3.7)
	prov, err := provider.New(provName, apiKey, baseURL)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	// Step 9: Build and send LLM request (Req 3.7)
	req := &provider.Request{
		Model:       modelName,
		System:      gcPatternPrompt,
		Messages:    []provider.Message{{Role: "user", Content: entries}},
		Temperature: 0.3,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := prov.Send(ctx, req)
	if err != nil {
		return mapProviderError(err)
	}

	// Step 10: Print analysis (Req 3.9)
	fmt.Fprintf(cmd.OutOrStdout(), "--- Analysis ---\n%s\n", resp.Content)

	// TODO: dry-run and trim (Phase 5)
	return nil
}
