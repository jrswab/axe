package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the hardcoded version constant for the CLI.
const Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "axe",
	Short: "Axe is a CLI tool for managing AI agents and skills",
	Long: `Axe is a command-line interface for managing AI agents, skills,
and configuration. It provides tools for setting up and organizing
your agent workspace.`,
	Example: `  axe version          Show the current version
  axe config path      Print the configuration directory path
  axe config init      Initialize the configuration directory`,
}

// Execute runs the root command and exits with code 1 on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
