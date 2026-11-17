.PHONY: test lint check apidiff

APIDIFF_MODULE = golang.org/x/exp/cmd/apidiff@v0.0.0-20261106192419-47ce03d25ef0
APIDIFF_PKG ?= github.com/jrswab/axe/pkg/runner

test:
	go test ./...

lint:
	golangci-lint run

check: lint test apidiff

apidiff:
	@git rev-parse --verify 'v1.11.0^{commit}' >/dev/null; status=$$?; \
	if [ $$status -ne 0 ]; then echo "apidiff: tag v1.11.0 not found; run 'git fetch --tags'" >&2; exit $$status; fi; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/axe-apidiff.XXXXXX"); \
	trap 'rm -rf "$$tmp"' EXIT; \
	mkdir "$$tmp/base"; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	git archive -o "$$tmp/base.tar" v1.11.0; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	tar -x -f "$$tmp/base.tar" -C "$$tmp/base"; status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	(cd "$$tmp/base" && go run $(APIDIFF_MODULE) -w "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	report=$$(go run $(APIDIFF_MODULE) -incompatible "$$tmp/old.export" $(APIDIFF_PKG)); status=$$?; if [ $$status -ne 0 ]; then exit $$status; fi; \
	if [ -n "$$report" ]; then printf '%s\n' "$$report"; echo "apidiff: pkg/runner has incompatible changes against v1.11.0" >&2; exit 1; fi
