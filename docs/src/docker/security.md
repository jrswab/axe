# Security

The container runs with the following hardening by default (via compose):

- **Non-root user** — UID 10001
- **Read-only root filesystem** — writable locations are the config mount, data mount, and `/tmp/axe` tmpfs
- **All capabilities dropped** — `cap_drop: ALL`
- **No privilege escalation** — `no-new-privileges:true`

These settings do not restrict outbound network access. To isolate an agent that only talks to a local Ollama instance, add `--network=none` and connect it to the shared Docker network manually.
