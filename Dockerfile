# Build stage
FROM golang:1.25-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /build/axe .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

RUN addgroup -g 10001 axe && \
    adduser -u 10001 -G axe -h /home/axe -D axe && \
    mkdir -p /home/axe/.config/axe /home/axe/.local/share/axe /tmp/axe && \
    chown -R axe:axe /home/axe /tmp/axe

ENV HOME=/home/axe
ENV XDG_CONFIG_HOME=/home/axe/.config
ENV XDG_DATA_HOME=/home/axe/.local/share

COPY --from=build /build/axe /usr/local/bin/axe

LABEL org.opencontainers.image.title="axe"
LABEL org.opencontainers.image.description="Lightweight CLI for running single-purpose LLM agents"
LABEL org.opencontainers.image.source="https://github.com/jrswab/axe"
LABEL org.opencontainers.image.licenses="Apache-2.0"

USER axe

ENTRYPOINT ["/usr/local/bin/axe"]
