# Session Context

## User Prompts

### Prompt 1

Working directory: `/Users/jaronswab/go/src/github.com/jrswab/axe`

Stage and commit all current changes with this exact commit message (use `git commit -m` with the message below). Stage all changed files first with `git add -A`.

Commit message (pass as a single string to `git commit -m`):

```
fix: resolve relative artifact dirs against agent workdir, not process CWD

Relative artifacts.dir paths (both from TOML config and --artifact-dir
flag) were resolved by the OS using the process's cu...

