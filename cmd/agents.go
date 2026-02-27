package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jrswab/axe/internal/agent"
	"github.com/jrswab/axe/internal/xdg"
	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent configurations",
	Long:  "Subcommands for managing agent TOML configuration files. Use these commands to list, inspect, create, and edit agent configurations.",
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agent configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, err := agent.List()
		if err != nil {
			return err
		}

		sort.Slice(agents, func(i, j int) bool {
			return agents[i].Name < agents[j].Name
		})

		for _, a := range agents {
			if a.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s - %s\n", a.Name, a.Description)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), a.Name)
			}
		}

		return nil
	},
}

var agentsShowCmd = &cobra.Command{
	Use:   "show <agent>",
	Short: "Show agent configuration details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := agent.Load(args[0])
		if err != nil {
			return err
		}

		w := cmd.OutOrStdout()

		// Always print required fields
		fmt.Fprintf(w, "%-16s%s\n", "Name:", cfg.Name)
		if cfg.Description != "" {
			fmt.Fprintf(w, "%-16s%s\n", "Description:", cfg.Description)
		}
		fmt.Fprintf(w, "%-16s%s\n", "Model:", cfg.Model)
		if cfg.SystemPrompt != "" {
			fmt.Fprintf(w, "%-16s%s\n", "System Prompt:", cfg.SystemPrompt)
		}
		if cfg.Skill != "" {
			fmt.Fprintf(w, "%-16s%s\n", "Skill:", cfg.Skill)
		}
		if len(cfg.Files) > 0 {
			fmt.Fprintf(w, "%-16s%s\n", "Files:", strings.Join(cfg.Files, ", "))
		}
		if cfg.Workdir != "" {
			fmt.Fprintf(w, "%-16s%s\n", "Workdir:", cfg.Workdir)
		}
		if len(cfg.SubAgents) > 0 {
			fmt.Fprintf(w, "%-16s%s\n", "Sub-Agents:", strings.Join(cfg.SubAgents, ", "))
		}
		if cfg.Memory.Enabled {
			fmt.Fprintf(w, "%-16s%v\n", "Memory Enabled:", cfg.Memory.Enabled)
		}
		if cfg.Memory.Path != "" {
			fmt.Fprintf(w, "%-16s%s\n", "Memory Path:", cfg.Memory.Path)
		}
		if cfg.Params.Temperature != 0 {
			fmt.Fprintf(w, "%-16s%g\n", "Temperature:", cfg.Params.Temperature)
		}
		if cfg.Params.MaxTokens != 0 {
			fmt.Fprintf(w, "%-16s%d\n", "Max Tokens:", cfg.Params.MaxTokens)
		}

		return nil
	},
}

var agentsInitCmd = &cobra.Command{
	Use:   "init <agent>",
	Short: "Create a new agent configuration file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		agentsDir := filepath.Join(configDir, "agents")
		path := filepath.Join(agentsDir, name+".toml")

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("agent config already exists: %s", path)
		}

		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory: %w", err)
		}

		content, err := agent.Scaffold(name)
		if err != nil {
			return err
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write agent config: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), path)
		return nil
	},
}

var agentsEditCmd = &cobra.Command{
	Use:   "edit <agent>",
	Short: "Edit an agent configuration file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			return errors.New("$EDITOR environment variable is not set")
		}

		name := args[0]

		configDir, err := xdg.GetConfigDir()
		if err != nil {
			return err
		}

		path := filepath.Join(configDir, "agents", name+".toml")

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("agent config not found: %s", name)
		}

		// Execute the editor as a child process with connected stdio
		editorCmd := exec.Command(editor, path)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		return editorCmd.Run()
	},
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsShowCmd)
	agentsCmd.AddCommand(agentsInitCmd)
	agentsCmd.AddCommand(agentsEditCmd)
	rootCmd.AddCommand(agentsCmd)
}
