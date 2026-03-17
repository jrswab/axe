package tool

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// truncateCommand truncates a command string to maxLen chars, appending "..." if longer.
func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen] + "..."
}

const maxOutputBytes = 100 * 1024 // 102400 bytes

// runCommandEntry returns the ToolEntry for the run_command tool.
func runCommandEntry() ToolEntry {
	return ToolEntry{
		Definition: runCommandDefinition,
		Execute:    runCommandExecute,
	}
}

func runCommandDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.RunCommand,
		Description: "Execute a shell command in the agent's working directory via sh -c and return the combined stdout/stderr output. Commands are sandboxed to the working directory: absolute paths outside it are rejected, parent traversal (..) escaping it is rejected, and all file operations should use relative paths.",
		Parameters: map[string]provider.ToolParameter{
			"command": {
				Type:        "string",
				Description: "The shell command to execute.",
				Required:    true,
			},
		},
	}
}

func runCommandExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	command := call.Arguments["command"]
	truncCmd := truncateCommand(command, 60)
	var cmdErr error

	defer func() {
		var summary string
		if result.IsError {
			var exitErr *exec.ExitError
			if errors.As(cmdErr, &exitErr) {
				summary = fmt.Sprintf("%q (exit %d)", truncCmd, exitErr.ExitCode())
			} else if cmdErr != nil {
				summary = fmt.Sprintf("%q (%s)", truncCmd, cmdErr.Error())
			} else {
				summary = fmt.Sprintf("%q (%s)", truncCmd, result.Content)
			}
		} else {
			summary = fmt.Sprintf("%q (exit 0)", truncCmd)
		}
		toolVerboseLog(ec, toolname.RunCommand, result, summary)
	}()

	if command == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "command is required",
			IsError: true,
		}
	}

	if err := validateCommand(ec.Workdir, command); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = ec.Workdir
	cmd.Env = sandboxEnv(ec.Workdir)

	output, err := cmd.CombinedOutput()

	// Truncate output if it exceeds 100KB.
	outStr := string(output)
	if len(output) > maxOutputBytes {
		outStr = string(output[:maxOutputBytes]) + "\n... [output truncated, exceeded 100KB]"
	}

	if err == nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: outStr,
			IsError: false,
		}
	}

	cmdErr = err

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("exit code %d\n%s", exitErr.ExitCode(), outStr),
			IsError: true,
		}
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: err.Error(),
		IsError: true,
	}
}
