# Tool pins — never @latest. Bump deliberately, one place only.
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.6.0
ACTIONLINT_VERSION := v1.7.12
XEXP_VERSION := v0.0.0-20260718201538-764159d718ef
# CGO_ENABLED=0: the tools need no cgo and the host gcc cannot build runtime/cgo.
GOLANGCI_LINT := CGO_ENABLED=0 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOVULNCHECK := CGO_ENABLED=0 go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
ACTIONLINT := CGO_ENABLED=0 go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
GORELEASE := CGO_ENABLED=0 go run golang.org/x/exp/cmd/gorelease@$(XEXP_VERSION)

.PHONY: test vet lint contract coverage update-spec smoke integration vulncheck tidy-check actionlint \
	check-version-selftest local-ci go-latest-check apidiff

test:
	go test -race -shuffle=on ./...

# The tags only add files — untagged files are always still vetted.
vet:
	go vet -tags='smoke integration' ./...

lint: vet
	$(GOLANGCI_LINT) run

contract:
	go test -run 'TestContract' ./...

# -coverpkg=./... merges cross-package coverage: without it, code covered
# only from another package's tests (transport.DoRaw via root tests, e.g.)
# reads 0% and the gate lies.
coverage:
	go test -race -coverpkg=./... -coverprofile=cover.out ./... && go tool cover -func=cover.out

vulncheck:
	$(GOVULNCHECK) ./...

# update-spec overwrites the pinned vendored spec (1.86.0) with today's live
# spec. If run before a release: restore with `git checkout -- openapi.yaml`
# and re-assert `version: 1.86.0` — re-vendoring is an ask-first change.
update-spec:
	curl --proto '=https' --tlsv1.2 -fsSL https://api.ynab.com/papi/open_api_spec.yaml -o openapi.yaml
	git diff --stat openapi.yaml

# -count=1: live-API runs must never be served from the test cache.
# CGO_ENABLED=0: no -race here, so cgo buys nothing and blocks nothing.
smoke:
	CGO_ENABLED=0 go test -tags=smoke -count=1 -run 'TestLiveSmoke' ./...

# -p 1: the live suite is never concurrent with itself.
# -v: the suite's t.Logf telemetry (per-case request counts, header
# discovery, SK sequence) is the audit trail — without -v a passing run
# swallows it and CI logs cannot prove what actually ran.
integration:
	CGO_ENABLED=0 go test -tags=integration -count=1 -p 1 -run 'TestLive' -v ./...

# Fails if go.mod/go.sum are untidy; running it locally is also the fix.
tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

actionlint:
	$(ACTIONLINT) .github/workflows/*.yaml

# The network-free half of the release gate, plus its expected failures.
check-version-selftest:
	CHECK_VERSION_SKIP_NET=1 scripts/check-version.sh "v$$(sed -n 's/^const Version = "\(.*\)"$$/\1/p' client.go)"
	@if CHECK_VERSION_SKIP_NET=1 scripts/check-version.sh v1.0.0-rc1 >/dev/null 2>&1; then \
		echo 'error: the gate accepted a prerelease tag'; exit 1; fi
	@if CHECK_VERSION_SKIP_NET=1 scripts/check-version.sh v0.0.0 >/dev/null 2>&1; then \
		echo 'error: the gate accepted a tag that mismatches Version'; exit 1; fi
	@echo 'ok: expected failures failed'

# Our own contract with users, gated: gorelease (x/exp; its engine is
# x/exp/apidiff) diffs the public surface against the latest released
# version and fails on undeclared breaking changes. Before the first
# release there is no base to diff — the target says so and passes;
# after v1.0.0 it bites on every run.
apidiff:
	@if [ -z "$$(git tag -l 'v[0-9]*' 2>/dev/null)" ]; then \
		echo 'apidiff: no released tag yet — activates after the first release'; \
	else \
		$(GORELEASE); \
	fi

# Everything the CI's full leg runs, locally — burn zero Actions minutes.
local-ci: lint test contract vulncheck tidy-check actionlint check-version-selftest apidiff coverage

# The toolchain twin of the spec-drift watch: fails when go.dev lists a
# newer stable Go than the newest one pinned in the workflows, so the
# matrix never silently goes stale. Bumping stays a deliberate act.
go-latest-check:
	@latest=$$(curl --proto '=https' --tlsv1.2 --max-time 30 -fsSL 'https://go.dev/dl/?mode=json' \
		| sed -n 's/^ *"version": *"go\([0-9]*\.[0-9]*\)[.0-9]*",$$/\1/p' | head -1); \
	pinned=$$(grep -rho '1\.[0-9]*\.x' .github/workflows/*.yaml | sort -t. -k2 -n | tail -1 | sed 's/\.x//'); \
	if [ -z "$$latest" ] || [ -z "$$pinned" ]; then \
		echo 'error: could not resolve latest ('"$$latest"') or pinned ('"$$pinned"') Go version'; exit 1; fi; \
	echo "latest stable: go$$latest — newest pinned: go$$pinned.x"; \
	if [ "$$latest" != "$$pinned" ]; then \
		echo "error: Go $$latest is out — bump the matrices in ci.yaml/smoke.yaml and go-version in release.yaml"; \
		exit 1; fi; \
	floor=$$(sed -n 's/^go \([0-9]*\.[0-9]*\)$$/\1/p' go.mod); \
	for v in $$(grep -rho '1\.[0-9]*\.x' .github/workflows/*.yaml | sed 's/\.x//' | sort -u); do \
		if [ "$$v" != "$$latest" ] && [ "$$v" != "$$floor" ]; then \
			echo "error: workflow pin go$$v.x is neither the go.mod floor ($$floor) nor latest ($$latest)"; \
			exit 1; fi; \
	done
