# git-erg — agent-native local ticket system
#
# Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
# Last modified: 2026-05-04
# Status: Working draft

# Usage:
#   make build      Build the erg binary
#   make test       Run Go unit tests and shell integration tests
#   make unit-test  Run Go unit tests with coverage report
#   make test-scaling  Empirical 4x-ladder scaling guard (slow; not in `test`)
#   make docs       Generate docs/erg-manual.md from erg --help --all
#   make validate   Validate tickets in tickets/
#   make ready      List ready tickets
#   make install-erg-binary              Install erg to ~/.local/bin

TEST_SUITES := validate check list ready update close migrate nextid log tag new init main archive rm datasafety pipeline help version hook godoc docs contract roundtrip
TEST_TARGETS := $(TEST_SUITES:%=test-%)

.PHONY: build test unit-test test-scaling _test-lint docs $(TEST_TARGETS) validate ready clean install-erg-binary update-bootstrap-binary

ERG_BIN := $(CURDIR)/build/erg
BOOTSTRAP_BIN := $(CURDIR)/tickets/erg

# Build hardening (single source of truth for both build recipes below).
# - CGO_ENABLED=0 + -tags osusergo,netgo: fully static, no libc dependency, so
#   the committed/cloned binary runs on musl (Alpine) and old glibc too.
# - -trimpath: drop absolute build paths.
# - -s -w (in ldflags): strip symbol table + DWARF (~31% smaller).
# The -X build-stamp flags are appended so version/revision are still embedded.
GO_BUILD_ENV   := CGO_ENABLED=0
GO_BUILD_FLAGS := -tags osusergo,netgo -trimpath
GO_LDFLAGS      = -s -w -X main.buildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.vcsRevision=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)

build:
	mkdir -p build
	cd src/go && $(GO_BUILD_ENV) go build \
		$(GO_BUILD_FLAGS) \
		-ldflags "$(GO_LDFLAGS)" \
		-o $(ERG_BIN) .

_test-lint:
	@if grep -R -n 'ERG="$${ERG_BIN:-tickets/' tests/*.sh >/dev/null; then \
		echo "ERROR: tests must default ERG to build/erg (found committed-binary default)" >&2; \
		exit 1; \
	fi
	@for f in tests/test_*.sh; do \
		suite=$$(basename $$f .sh | sed 's/^test_//'); \
		echo " $(TEST_SUITES) " | grep -q " $$suite " || \
			{ echo "ERROR: $$f not in TEST_SUITES" >&2; exit 1; }; \
	done

$(TEST_TARGETS): test-%: build _test-lint
	@ERG_BIN=$(ERG_BIN) sh tests/test_$*.sh

unit-test: build
	cd src/go && go test -cover -coverprofile=$(CURDIR)/build/coverage.out ./... && \
		go tool cover -func=$(CURDIR)/build/coverage.out

test: unit-test $(TEST_TARGETS)
	@echo "ALL TESTS PASSED"

# Empirical scaling regression guard (ticket 0159). Build-tagged out of the
# default suite: slow, and a regression check rather than a per-merge gate.
# No `build` prerequisite — the test drives the commands in-process, never the
# binary. The -run pattern matches the linear test, its negative control, and
# the corpus-validity check (all named TestScaling*). -count=1 disables the
# test result cache so the profiling table is always printed on demand.
test-scaling:
	cd src/go && go test -tags scaling -run TestScaling -count=1 -v .

validate: build
	$(ERG_BIN) check tickets/

ready: build
	$(ERG_BIN) ready tickets/

update-bootstrap-binary:
	cd src/go && $(GO_BUILD_ENV) go build \
		$(GO_BUILD_FLAGS) \
		-ldflags "$(GO_LDFLAGS)" \
		-o $(BOOTSTRAP_BIN) .

install-erg-binary:
	@mkdir -p $(HOME)/.local/bin
	@if [ "$$(uname -s)" = "Linux" ] && [ "$$(uname -m)" = "x86_64" ] && [ -f "$(BOOTSTRAP_BIN)" ] \
		&& [ -z "$$(find src/go -name '*.go' -newer "$(BOOTSTRAP_BIN)" -print -quit)" ]; then \
		install -m755 "$(BOOTSTRAP_BIN)" "$(HOME)/.local/bin/erg"; \
	elif command -v go >/dev/null 2>&1; then \
		cd src/go && go build -o "$(HOME)/.local/bin/erg" .; \
	else \
		echo "ERROR: bootstrap binary not usable and Go not found — cannot install erg" >&2; exit 1; \
	fi
	@echo "erg installed to $(HOME)/.local/bin/erg"

docs: build
	mkdir -p docs
	$(ERG_BIN) --help --all > docs/erg-manual.md

clean:
	rm -rf build
