GO ?= go
GOLANGCI_LINT_VERSION := v2.12.2
GOSEC_VERSION := v2.22.8
GOVULNCHECK_VERSION := v1.6.0
GORELEASER_VERSION := v2.17.1

.PHONY: build test test-race test-contract test-integration coverage fmt lint e2e vet docs security check \
	release-validate changelog-check release-check release-snapshot release-dry-run release-contract-check

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

# releasectl self-validation: build the CLI and run internal/release's unit
# tests. Not a no-op — it's the same package release.yml and prepare-release
# rely on to gate every release. -o /dev/null: `go build ./cmd/releasectl`
# names a single, unambiguous main package, so (unlike `go build ./...`,
# which discards output for multi-package targets) it would otherwise leave
# a `releasectl` binary at the repo root.
release-validate:
	$(GO) build -o /dev/null ./cmd/releasectl
	$(GO) test ./internal/release/...

changelog-check:
	./scripts/changelog-check.sh

release-check:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check
	$(MAKE) changelog-check

# Offline guard against the workflow<->tool contract seams a whole-branch
# review caught (fragment layout, gh api HTTP method, changie's actual
# output filename) — see scripts/release-contract-check.sh. Fast; runs on
# every pull request as ci.yml's release-config job, right after
# release-check.
release-contract-check:
	./scripts/release-contract-check.sh

# Full multi-arch GoReleaser snapshot build + artifact contract check. Slow
# (real cross-compiles + SBOM generation) — CI job + documented local
# target, deliberately NOT part of `check`.
release-snapshot:
	./scripts/release-snapshot-check.sh

# release-snapshot proves the artifacts build; this also prints how the next
# release version would be computed, without creating anything. Try it
# yourself, e.g.:
#   echo '{"previous_tag":"","prs":[{"number":1,"labels":["release/minor"],"head_branch":"feat/x","head_repo":"RafPe/steampipe-plugin-youtrack"}]}' \
#     | go run ./cmd/releasectl next-version --input - --trusted-repo RafPe/steampipe-plugin-youtrack
release-dry-run: release-snapshot
	@echo "release-dry-run: snapshot build ok."
	@echo "release-dry-run: next-version reads {\"previous_tag\":\"vX.Y.Z\"|\"\",\"prs\":[...]} on stdin and"
	@echo "  prints one line of JSON: either {\"release\":true,\"version\":...,\"bump\":...} or"
	@echo "  {\"release\":false,\"reason\":...}. Bootstrap (empty previous_tag) always computes"
	@echo "  v0.1.0/minor regardless of PR labels — see internal/release's CLI contract."
	@echo "  Example:"
	@echo "    echo '{\"previous_tag\":\"\",\"prs\":[{\"number\":1,\"labels\":[\"release/minor\"],\"head_branch\":\"feat/x\",\"head_repo\":\"RafPe/steampipe-plugin-youtrack\"}]}' \\"
	@echo "      | go run ./cmd/releasectl next-version --input - --trusted-repo RafPe/steampipe-plugin-youtrack"

check: fmt test test-contract test-integration coverage test-race vet lint build docs security release-validate release-check
