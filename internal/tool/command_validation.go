package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Regex to match absolute paths: / followed by one or more path characters
var absPathRe = regexp.MustCompile(`(/[\w./_-]*)`)

// Regex to match .. as a path component (not inside a filename like file..bak)
// Matches: standalone "..", "../something", "something/..", "something/../other"
var dotDotRe = regexp.MustCompile(`(?:^|/)\.\.(?:/|$)`)

// Regex to match tilde in shell expansion position
// ~ at start of token (after whitespace, =, :, or start of string) followed by / or end of token
var tildeRe = regexp.MustCompile(`(?:^|[\s=:])~(?:/|$|\s)`)

// validateCommand performs heuristic validation of a shell command string to detect
// attempts to access files outside the given workdir. It scans the raw command string
// for: (1) absolute paths not within workdir, (2) ".." path components that resolve
// outside workdir, and (3) tilde "~" in shell expansion position.
//
// This is a heuristic guard — not airtight. Shell is Turing-complete, and sufficiently
// creative constructs (variable expansion, command substitution, encoding tricks, eval,
// aliases) can bypass this check. For full sandboxing, use Docker/container isolation.
//
// Returns nil if the command passes validation, or an error with an actionable message
// including the pattern type, offending token, and workdir restriction note.
func validateCommand(workdir, command string) error {
	cleanWorkdir := filepath.Clean(workdir)

	// Special case: if workdir is root, allow all absolute paths
	// since everything is within root
	if cleanWorkdir == string(filepath.Separator) {
		return nil
	}

	// Check for tilde expansion FIRST (before absolute path check)
	// This catches cases like "~/secrets" which would otherwise be
	// caught as absolute paths after shell expansion
	if tildeRe.MatchString(command) {
		// Find the offending token
		loc := tildeRe.FindStringIndex(command)
		// Extract context around the match
		start := loc[0]
		end := loc[1]
		offending := strings.TrimSpace(command[start:end])
		// Strip leading non-tilde prefix character (e.g., '=' from '=~/')
		if idx := strings.Index(offending, "~"); idx > 0 {
			offending = offending[idx:]
		}
		return fmt.Errorf("command rejected: home directory expansion %q is not allowed; use relative paths from the working directory instead", offending)
	}

	// Check for .. traversal escaping workdir
	// Split into tokens and check each one that contains ..
	tokens := extractTokens(command)
	for _, token := range tokens {
		if dotDotRe.MatchString(token) {
			resolved := filepath.Clean(filepath.Join(cleanWorkdir, token))
			if !isWithinDir(resolved, cleanWorkdir) {
				return fmt.Errorf("command rejected: parent traversal in %q escapes the working directory %q; use relative paths that stay within the workdir", token, workdir)
			}
		}
	}

	// Check for absolute paths outside workdir
	// Skip tokens that contain ".." as they were already handled above
	// Allow /dev/ paths (device files are system resources, not file system traversal)
	matches := absPathRe.FindAllString(command, -1)
	for _, match := range matches {
		// Skip if this match is part of a parent traversal
		if strings.Contains(match, "..") {
			continue
		}
		// Allow /dev/ device paths
		if strings.HasPrefix(match, "/dev/") {
			continue
		}
		cleanPath := filepath.Clean(match)
		if !isWithinDir(cleanPath, cleanWorkdir) {
			return fmt.Errorf("command rejected: absolute path %q is outside the working directory %q; use relative paths instead", match, workdir)
		}
	}

	return nil
}

// extractTokens splits a command string into tokens by whitespace and shell metacharacters.
func extractTokens(command string) []string {
	// Split by whitespace, then further split by shell operators
	fields := strings.Fields(command)
	var tokens []string
	for _, f := range fields {
		// Split on = for env var assignments like FOO=/etc/passwd
		parts := strings.SplitN(f, "=", 2)
		for _, p := range parts {
			if p != "" {
				tokens = append(tokens, p)
			}
		}
	}
	return tokens
}

// sandboxEnv builds an explicit environment variable list for sandboxed command execution.
// It sets HOME and TMPDIR to the workdir, inherits PATH and locale/identity/terminal
// variables from the parent process (if set), and strips all other variables.
func sandboxEnv(workdir string) []string {
	env := []string{
		"HOME=" + workdir,
		"TMPDIR=" + workdir,
	}

	// Inherit these from parent process if they are set and non-empty
	inherit := []string{"PATH", "LANG", "LC_ALL", "USER", "LOGNAME", "TERM"}
	for _, key := range inherit {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}

	return env
}
