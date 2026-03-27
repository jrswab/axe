# Session Context

## User Prompts

### Prompt 1

You are working in the axe project at `/Users/jaronswab/go/src/github.com/jrswab/axe`.

**Task:** Add a new table-driven test function `TestRun_ArtifactDirWorkdirResolution` to `cmd/run_test.go`. Place it right after the existing `TestRun_ArtifactEnvVar` test (which ends around line 3122).

**Context:** We're fixing a bug where relative `artifacts.dir` paths in TOML and relative `--artifact-dir` flag values are resolved against the process CWD instead of the agent's resolved `--workdir`. The ...

