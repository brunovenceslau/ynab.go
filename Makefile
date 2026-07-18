# Tool pins — never @latest. Bump deliberately, one place only.
GOLANGCI_LINT_VERSION := v2.12.2
# CGO_ENABLED=0: the tool needs no cgo and the host gcc cannot build runtime/cgo.
GOLANGCI_LINT := CGO_ENABLED=0 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: test lint contract coverage update-spec smoke

test:
	go test -race -shuffle=on ./...

lint:
	go vet ./...
	$(GOLANGCI_LINT) run

contract:
	go test -run 'TestContract' ./...

coverage:
	go test -race -coverprofile=cover.out ./... && go tool cover -func=cover.out

# update-spec overwrites the pinned vendored spec (1.86.0) with today's live
# spec. If run before a release: restore with `git checkout -- openapi.yaml`
# and re-assert `version: 1.86.0` — re-vendoring is an ask-first change.
update-spec:
	curl --proto '=https' --tlsv1.2 -fsSL https://api.ynab.com/papi/open_api_spec.yaml -o openapi.yaml
	git diff --stat openapi.yaml

# -count=1: live-API runs must never be served from the test cache.
smoke:
	go test -tags=smoke -count=1 -run 'TestLiveSmoke' ./...
