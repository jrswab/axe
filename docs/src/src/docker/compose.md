# Docker Compose

A `docker-compose.yml` is included for running axe alongside a local Ollama instance.

## Cloud Provider Only (no Ollama)

```bash
docker compose run --rm axe run my-agent
```

## With Ollama Sidecar

```bash
docker compose --profile ollama up -d ollama
docker compose --profile cli run --rm axe run my-agent
```

## Pull an Ollama Model

```bash
docker compose --profile ollama exec ollama ollama pull llama3
```

> **Note:** The compose `axe` service declares `depends_on: ollama`. Docker Compose will attempt to start the Ollama service whenever axe is started via compose, even for cloud-only runs. For cloud-only usage without Ollama, use `docker run` directly instead of `docker compose run`.

## Ollama on the Host

If Ollama runs directly on the host (not via compose), point to it with:

- **Linux:** `--add-host=host.docker.internal:host-gateway -e AXE_OLLAMA_BASE_URL=http://host.docker.internal:11434`
- **macOS / Windows (Docker Desktop):** `-e AXE_OLLAMA_BASE_URL=http://host.docker.internal:11434`
