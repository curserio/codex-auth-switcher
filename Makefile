GOCACHE ?= /tmp/codex-auth-switcher-go-build

.PHONY: test install check status-demo

test:
	GOCACHE=$(GOCACHE) go test ./...

install:
	GOCACHE=$(GOCACHE) go install ./cmd/codex-switch

check: test
	git diff --check

status-demo:
	GOCACHE=$(GOCACHE) go run ./cmd/codex-switch status
