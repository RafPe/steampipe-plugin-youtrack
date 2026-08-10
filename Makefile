GO ?= go
GOLANGCI_LINT_VERSION := v2.12.2
GOSEC_VERSION := v2.22.8
GOVULNCHECK_VERSION := v1.6.0

.PHONY: build test test-race test-contract test-integration coverage fmt lint e2e vet docs security check

build:
	$(GO) build ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-contract:
	$(GO) test ./tests/contract

test-integration:
	$(GO) test ./tests/integration

coverage:
	./scripts/coverage.sh

fmt:
	test -z "$$(gofmt -l .)"

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

e2e:
	./tests/e2e/run.sh

vet:
	$(GO) vet ./...

docs:
	$(GO) list -f '{{if .GoFiles}}{{.ImportPath}}{{end}}' ./... | sed '/^$$/d' | xargs -n 1 $(GO) doc >/dev/null

security:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) ./...

check: fmt test test-contract test-integration coverage test-race vet lint build docs security
