# Installation

Requires **Go 1.25+**.

## Pre-built Binaries (No Go Required)

Download a pre-built binary for your platform from the [GitHub Releases page](https://github.com/jrswab/axe/releases/latest).

Binaries are available for:

| OS      | Architectures    |
|---------|------------------|
| Linux   | amd64, arm64     |
| macOS   | amd64, arm64     |
| Windows | amd64, arm64     |

Download the archive for your platform, extract it, and place the `axe` binary somewhere on your `$PATH`.

**Example (Linux amd64):**

```bash
curl -L https://github.com/jrswab/axe/releases/latest/download/axe_linux_amd64.tar.gz | tar xz
sudo mv axe /usr/local/bin/
```

## Install via Go

```bash
go install github.com/jrswab/axe@latest
```

> **Note:** If this fails with `invalid go version`, your Go toolchain is older than 1.25. Either upgrade Go from [go.dev/dl](https://go.dev/dl/) or use a pre-built binary above.

## Docker

```bash
docker pull ghcr.io/jrswab/axe:latest
```

See the [Docker section in the README](https://github.com/jrswab/axe#docker) for full usage instructions including volume mounts and environment variables.

## Build from Source

```bash
git clone https://github.com/jrswab/axe.git
cd axe
go build .
```

## Initialize Configuration

After installing, initialize the configuration directory:

```bash
axe config init
```

This creates the directory structure at `$XDG_CONFIG_HOME/axe/` with a sample skill and a default `config.toml` for provider credentials.
