# git-erg — agent-native local ticket system
#
# Author: Minh Ha-Duong <minh.ha-duong@cnrs.fr>
# Last modified: 2026-05-04
# Status: Working draft

# Usage:
#   make build      Build the erg binary
#   make test       Run shell integration tests
#   make validate   Validate tickets in tickets/
#   make ready      List ready tickets
#   make install-erg-binary              Install erg to ~/.local/bin

.PHONY: build test validate ready clean install-erg-binary update-bootstrap-binary

ERG_BIN := $(CURDIR)/build/erg
BOOTSTRAP_BIN := $(CURDIR)/tickets/tools/go/erg

build:
	mkdir -p build
	cd tickets/tools/go && go build -o $(ERG_BIN) .

test: build
	@if grep -R -n 'ERG="$${ERG_BIN:-tickets/tools/go/erg}"' tests/*.sh >/dev/null; then \
		echo "ERROR: tests must default ERG to build/erg (found legacy tickets/tools/go/erg default)" >&2; \
		exit 1; \
	fi
	@ERG_BIN=$(ERG_BIN) sh tests/test_validate.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_ready.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_update.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_close.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_migrate.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_nextid.sh
	@ERG_BIN=$(ERG_BIN) sh tests/test_init_uninstall.sh
	@echo "ALL TESTS PASSED"

validate: build
	$(ERG_BIN) validate tickets/

ready: build
	$(ERG_BIN) ready tickets/

update-bootstrap-binary:
	cd tickets/tools/go && go build -o erg .

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

clean:
	rm -rf build
