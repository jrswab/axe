# Installation

Source builds and `pkg/runner` imports require **Go 1.26.11 or later**.

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

> **Note:** For a checkout, Go's default automatic toolchain selection may use an available `go1.26.11` on `PATH` or download it if the installed toolchain is older. A blocked or unverifiable download can fail even with cached dependencies. `GOTOOLCHAIN=local` disables switching and requires an installed Go version at least 1.26.11. Versioned `go install` uses the target module's minimum but not its toolchain suggestion. If installation fails, check the Go version and toolchain availability, upgrade from [go.dev/dl](https://go.dev/dl/), or use a pre-built binary above.

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
