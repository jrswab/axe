package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// resolveWritePath validates and resolves a write path relative to baseDir.
// It creates parent directories and checks for symlink escapes.
func resolveWritePath(baseDir, path string) (resolved string, err error) {
	if path == "" {
		return "", errors.New("path is required")
	}

	if filepath.IsAbs(path) {
		return "", errors.New("absolute paths are not allowed")
	}

	cleanBase := filepath.Clean(baseDir)
	resolved = filepath.Clean(filepath.Join(cleanBase, path))

	if !isWithinDir(resolved, cleanBase) {
		return "", errors.New("path escapes workdir")
	}

	parent := filepath.Dir(resolved)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}

	evalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}

	evalBase, err := filepath.EvalSymlinks(cleanBase)
	if err != nil {
		return "", err
	}

	if !isWithinDir(evalParent, evalBase) {
		return "", errors.New("path escapes workdir")
	}

	return resolved, nil
}

// writeFileEntry returns the ToolEntry for the write_file tool.
func writeFileEntry() ToolEntry {
	return ToolEntry{
		Definition: writeFileDefinition,
		Execute:    writeFileExecute,
	}
}

func writeFileDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.WriteFile,
		Description: "Create or overwrite a file relative to the working directory. Creates parent directories as needed. Overwrites the file if it already exists.",
		Parameters: map[string]provider.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Relative path to the file to write.",
				Required:    true,
			},
			"content": {
				Type:        "string",
				Description: "The content to write to the file.",
				Required:    false,
			},
			"artifact": {
				Type:        "string",
				Required:    false,
				Description: `When "true", write to the artifact directory instead of the working directory.`,
			},
		},
	}
}

func writeFileExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	path := call.Arguments["path"]

	defer func() {
		var summary string
		if result.IsError {
			summary = fmt.Sprintf("path %q: %s", path, result.Content)
		} else {
			summary = fmt.Sprintf("path %q (%d bytes)", path, len(call.Arguments["content"]))
		}
		toolVerboseLog(ec, toolname.WriteFile, result, summary)
	}()

	// Check if artifact mode is requested.
	if strings.EqualFold(call.Arguments["artifact"], "true") {
		return writeFileArtifact(ctx, call, ec, path)
	}

	// Validate and resolve the path.
	resolved, err := resolveWritePath(ec.Workdir, path)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	// Extract content (missing or empty key writes 0-byte file).
	content := call.Arguments["content"]

	// Write file.
	data := []byte(content)
	if err := os.WriteFile(resolved, data, 0o644); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("wrote %d bytes to %s", len(data), path),
		IsError: false,
	}
}

func writeFileArtifact(ctx context.Context, call provider.ToolCall, ec ExecContext, path string) provider.ToolResult {
	if ec.ArtifactDir == "" {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "artifact directory not configured for this agent",
			IsError: true,
		}
	}

	// Validate and resolve the path.
	resolved, err := resolveWritePath(ec.ArtifactDir, path)
	if err != nil {
		msg := rewriteArtifactError(err.Error())
		return provider.ToolResult{
			CallID:  call.ID,
			Content: msg,
			IsError: true,
		}
	}

	// Extract content.
	content := call.Arguments["content"]
	data := []byte(content)

	// Write file.
	if err := os.WriteFile(resolved, data, 0o644); err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	// Record in tracker if available.
	if ec.ArtifactTracker != nil {
		ec.ArtifactTracker.Record(artifact.Entry{
			Path:  path,
			Agent: "",
			Size:  int64(len(data)),
		})
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("wrote %d bytes to %s (artifact)", len(data), path),
		IsError: false,
	}
}
