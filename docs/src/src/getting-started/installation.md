# Installation

Requires **Go 1.25+**.

## Install via Go

```bash
go install github.com/jrswab/axe@latest
```

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
