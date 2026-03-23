package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// readFileEntry returns the ToolEntry for the read_file tool.
func readFileEntry() ToolEntry {
	return ToolEntry{
		Definition: readFileDefinition,
		Execute:    readFileExecute,
	}
}

func readFileDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.ReadFile,
		Description: "Read the contents of a file relative to the working directory. Returns line-numbered output in the format 'N: content' for each line.",
		Parameters: map[string]provider.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Relative path to the file to read.",
				Required:    true,
			},
			"offset": {
				Type:        "string",
				Description: "1-indexed line number to start reading from. Defaults to 1.",
				Required:    false,
			},
			"limit": {
				Type:        "string",
				Description: "Maximum number of lines to return. Defaults to 2000.",
				Required:    false,
			},
			"artifact": {
				Type:        "string",
				Required:    false,
				Description: `When "true", read from the artifact directory instead of the working directory.`,
			},
		},
	}
}

func readFileExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	path := call.Arguments["path"]

	defer func() {
		var summary string
		if result.IsError {
			summary = fmt.Sprintf("path %q: %s", path, result.Content)
		} else {
			lineCount := 0
			if result.Content != "" {
				lineCount = strings.Count(result.Content, "\n") + 1
			}
			summary = fmt.Sprintf("path %q (%d lines)", path, lineCount)
		}
		toolVerboseLog(ec, toolname.ReadFile, result, summary)
	}()

	// Determine which directory to resolve against.
	baseDir := ec.Workdir
	if strings.EqualFold(call.Arguments["artifact"], "true") {
		if ec.ArtifactDir == "" {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: "artifact directory not configured for this agent",
				IsError: true,
			}
		}
		baseDir = ec.ArtifactDir
	}
	resolved, err := validatePath(baseDir, path)
	if err != nil {
		msg := err.Error()
		if strings.EqualFold(call.Arguments["artifact"], "true") {
			msg = rewriteArtifactError(msg)
		}
		return provider.ToolResult{
			CallID:  call.ID,
			Content: msg,
			IsError: true,
		}
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	if info.IsDir() {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "path is a directory, not a file",
			IsError: true,
		}
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	// Binary detection: scan first 512 bytes for NUL byte.
	peek := content
	if len(peek) > 512 {
		peek = peek[:512]
	}
	if bytes.ContainsRune(peek, '\x00') {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "binary file detected",
			IsError: true,
		}
	}

	// Parse offset (default 1).
	offset := 1
	if v := call.Arguments["offset"]; v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("invalid offset %q: %s", v, err.Error()),
				IsError: true,
			}
		}
		if offset < 1 {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: "offset must be >= 1",
				IsError: true,
			}
		}
	}

	// Parse limit (default 2000).
	limit := 2000
	if v := call.Arguments["limit"]; v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("invalid limit %q: %s", v, err.Error()),
				IsError: true,
			}
		}
		if limit < 1 {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: "limit must be >= 1",
				IsError: true,
			}
		}
	}

	// Empty file early return.
	if len(content) == 0 {
		if offset > 1 {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("offset %d exceeds file length of 0 lines", offset),
				IsError: true,
			}
		}
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "",
			IsError: false,
		}
	}

	// Split into lines and handle trailing newline.
	lines := strings.Split(string(content), "\n")
	if len(content) > 0 && content[len(content)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}

	totalLines := len(lines)

	// Offset bounds check.
	if offset > totalLines {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("offset %d exceeds file length of %d lines", offset, totalLines),
			IsError: true,
		}
	}

	// Line selection.
	start := offset - 1
	end := start + limit
	if end > totalLines {
		end = totalLines
	}
	selected := lines[start:end]

	// Output formatting.
	var b strings.Builder
	for i, line := range selected {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d: %s", start+i+1, line)
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: b.String(),
		IsError: false,
	}
}
