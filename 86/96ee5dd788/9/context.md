# Session Context

## User Prompts

### Prompt 1

I need to understand the current artifact directory resolution code in cmd/run.go. 

Please:
1. Read cmd/run.go lines 140-230 (the area around workdir resolution and artifact directory resolution)
2. Read the full runAgent function signature and the first few lines to understand the function parameters
3. Identify the exact current code for:
   - The `--artifact-dir` flag branch
   - The TOML `cfg.Artifacts.Dir` branch
   - The auto-generated XDG cache path branch
   - Where `workdir` is reso...

