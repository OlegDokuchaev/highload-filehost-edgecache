# Cargo commands
CARGO := cargo
CARGO_FMT := $(CARGO) fmt
CARGO_CLIPPY := $(CARGO) clippy

.PHONY: fmt-check clippy

fmt-check:
	$(CARGO_FMT) -- --check

clippy:
	$(CARGO_CLIPPY) --all-targets --all-features -- -D warnings
