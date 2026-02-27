package cmd

import (
	"errors"
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
  axe config init      Initialize the configuration directory
  axe run pr-reviewer   Run the pr-reviewer agent`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// exitCodeFromError extracts the exit code from an ExitError, defaulting to 1.
func exitCodeFromError(err error) int {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return 1
}

// Execute runs the root command and exits with the appropriate exit code on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFromError(err))
	}
}
