# git-erg — agent-native local ticket system
#
# Usage:
#   make build      Build the erg binary
#   make test       Run shell integration tests
#   make validate   Validate tickets in tickets/
#   make ready      List ready tickets
#   make archive    Dry-run archive (pass EXECUTE=1 to commit)
#   make install DEST=/path/to/project  Install into a project
#   make install-erg-binary              Install erg to ~/.local/bin

.PHONY: build test validate ready archive clean install install-erg-binary

ERG_BIN := tickets/tools/go/erg

build:
	cd tickets/tools/go && go build -o erg .

test: build
	@sh tests/test_validate.sh
	@sh tests/test_ready.sh
	@sh tests/test_archive.sh
	@sh tests/test_update.sh
	@sh tests/test_close.sh
	@sh tests/test_migrate.sh
	@echo "ALL TESTS PASSED"

validate: build
	$(ERG_BIN) validate tickets/

ready: build
	$(ERG_BIN) ready tickets/

DAYS ?= 90
EXECUTE ?=
archive: build
	$(ERG_BIN) archive tickets/ --days=$(DAYS) $(if $(EXECUTE),--execute)

install-erg-binary:
	@mkdir -p $(HOME)/.local/bin
	@if [ "$$(uname -s)" = "Linux" ] && [ "$$(uname -m)" = "x86_64" ] && [ -f $(ERG_BIN) ] \
		&& [ -z "$$(find tickets/tools/go -name '*.go' -newer $(ERG_BIN) -print -quit)" ]; then \
		install -m755 $(ERG_BIN) $(HOME)/.local/bin/erg; \
	elif command -v go >/dev/null 2>&1; then \
		cd tickets/tools/go && go build -o $(HOME)/.local/bin/erg .; \
	else \
		echo "ERROR: committed binary not usable and Go not found — cannot install erg" >&2; exit 1; \
	fi
	@echo "erg installed to $(HOME)/.local/bin/erg"

install:
ifndef DEST
	$(error DEST is required. Usage: make install DEST=/path/to/project)
endif
	sh bin/install.sh "$(DEST)"

clean:
	rm -f $(ERG_BIN)
