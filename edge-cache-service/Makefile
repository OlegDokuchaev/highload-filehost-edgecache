# Path to the environment file
ENV_FILE ?= .env

# Cargo commands
CARGO := cargo
CARGO_TEST := $(CARGO) test
CARGO_FMT := $(CARGO) fmt
CARGO_CLIPPY := $(CARGO) clippy

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

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

# ---------------------------------------------------------------------------
# k6 load testing
# ---------------------------------------------------------------------------

K6          := k6
K6_DIR      := k6
ORIGIN_DIR  := $(K6_DIR)/origin

.PHONY: k6-origin k6-cold k6-warm k6-hot

k6-origin:
	@bash $(ORIGIN_DIR)/start.sh

k6-cold:
	$(K6) run $(K6_DIR)/cold-cache.js

k6-warm:
	$(K6) run $(K6_DIR)/warm-cache.js

k6-hot:
	$(K6) run $(K6_DIR)/hot-key.js
