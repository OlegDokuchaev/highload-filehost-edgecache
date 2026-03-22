# Path to the environment file
ENV_FILE ?= .env

# Cargo commands
CARGO := cargo
CARGO_TEST := $(CARGO) test
CARGO_FMT := $(CARGO) fmt
CARGO_CLIPPY := $(CARGO) clippy

.PHONY: fmt-check clippy test unit integration

test: unit integration

unit:
	@bash -c 'set -a; source $(ENV_FILE); set +a; $(CARGO_TEST) --test unit'

integration:
	@bash -c 'set -a; source $(ENV_FILE); set +a; $(CARGO_TEST) --test integration -- --test-threads=1'

fmt-check:
	$(CARGO_FMT) -- --check

clippy:
	$(CARGO_CLIPPY) --all-targets --all-features -- -D warnings
