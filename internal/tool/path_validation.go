package tool

import (
	"errors"
	"path/filepath"
	"strings"
)

// isWithinDir reports whether child is equal to or a subdirectory of parent.
// Both paths must be cleaned (filepath.Clean) before calling.
// Unlike a plain strings.HasPrefix check, this handles the case where
// parent="/tmp/work" and child="/tmp/worker" correctly (returns false).
func isWithinDir(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// validatePath resolves a relative path against a workdir and rejects absolute
// paths, parent-traversal escapes, and symlinks that leave the workdir boundary.
// It is reused by all file-oriented tool executors.
func validatePath(workdir, relPath string) (string, error) {
	if relPath == "" {
		return "", errors.New("path is required")
	}

	if filepath.IsAbs(relPath) {
		return "", errors.New("absolute paths are not allowed")
	}

	cleanWorkdir := filepath.Clean(workdir)
	resolved := filepath.Clean(filepath.Join(cleanWorkdir, relPath))

	// Fast-path: catch .. traversal before EvalSymlinks so nonexistent
	// escaped paths get a clear error instead of an ENOENT from EvalSymlinks.
	if !isWithinDir(resolved, cleanWorkdir) {
		return "", errors.New("path escapes workdir")
	}

	evalResolved, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", err
	}

	evalWorkdir, err := filepath.EvalSymlinks(cleanWorkdir)
	if err != nil {
		return "", err
	}

	if !isWithinDir(evalResolved, evalWorkdir) {
		return "", errors.New("path escapes workdir")
	}

	return evalResolved, nil
}

// rewriteArtifactError rewrites "workdir" to "artifact directory" in error
// messages returned from validatePath when operating in artifact mode.
func rewriteArtifactError(msg string) string {
	return strings.ReplaceAll(msg, "workdir", "artifact directory")
}
