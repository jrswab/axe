# Building the Image

```bash
docker build -t axe .
```

Multi-architecture builds (linux/amd64, linux/arm64) are supported via buildx:

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t axe:latest .
```
