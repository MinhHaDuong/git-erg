# git-erg — agent-native local ticket system
#
# Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
# Last modified: 2026-05-04
# Status: Working draft

# Usage:
#   make build      Build the erg binary
#   make test       Run Go unit tests and shell integration tests
#   make unit-test  Run Go unit tests with coverage report
#   make validate   Validate tickets in tickets/
#   make ready      List ready tickets
#   make install-erg-binary              Install erg to ~/.local/bin
#   make install-scripts                 Install scripts/ to ~/.local/bin

TEST_SUITES := validate check ready update close migrate nextid log new init main archive pipeline help
TEST_TARGETS := $(TEST_SUITES:%=test-%)

.PHONY: build test unit-test _test-lint $(TEST_TARGETS) validate ready clean install-erg-binary install-scripts update-bootstrap-binary

ERG_BIN := $(CURDIR)/build/erg
BOOTSTRAP_BIN := $(CURDIR)/tickets/tools/go/erg

build:
	mkdir -p build
	cd tickets/tools/go && go build \
		-ldflags "-X main.buildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.vcsRevision=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
		-o $(ERG_BIN) .

_test-lint:
	@if grep -R -n 'ERG="$${ERG_BIN:-tickets/tools/go/erg}"' tests/*.sh >/dev/null; then \
		echo "ERROR: tests must default ERG to build/erg (found legacy tickets/tools/go/erg default)" >&2; \
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
	cd tickets/tools/go && go test -cover -coverprofile=$(CURDIR)/build/coverage.out ./... && \
		go tool cover -func=$(CURDIR)/build/coverage.out

test: unit-test $(TEST_TARGETS)
	@echo "ALL TESTS PASSED"

validate: build
	$(ERG_BIN) check tickets/

ready: build
	$(ERG_BIN) ready tickets/

update-bootstrap-binary:
	cd tickets/tools/go && go build \
		-ldflags "-X main.buildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.vcsRevision=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
		-o $(BOOTSTRAP_BIN) .

install-erg-binary:
	@mkdir -p $(HOME)/.local/bin
	@if [ "$$(uname -s)" = "Linux" ] && [ "$$(uname -m)" = "x86_64" ] && [ -f "$(BOOTSTRAP_BIN)" ] \
		&& [ -z "$$(find tickets/tools/go -name '*.go' -newer "$(BOOTSTRAP_BIN)" -print -quit)" ]; then \
		install -m755 "$(BOOTSTRAP_BIN)" "$(HOME)/.local/bin/erg"; \
	elif command -v go >/dev/null 2>&1; then \
		cd tickets/tools/go && go build -o "$(HOME)/.local/bin/erg" .; \
	else \
		echo "ERROR: bootstrap binary not usable and Go not found — cannot install erg" >&2; exit 1; \
	fi
	@echo "erg installed to $(HOME)/.local/bin/erg"

install-scripts:
	@mkdir -p $(HOME)/.local/bin
	@for f in scripts/*; do \
		install -m755 "$$f" "$(HOME)/.local/bin/$$(basename $$f)"; \
		echo "installed $$f → $(HOME)/.local/bin/$$(basename $$f)"; \
	done

clean:
	rm -rf build
