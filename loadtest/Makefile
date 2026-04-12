K6         := k6
SCRIPT_DIR := scenarios
ORIGIN_DIR := origin

# ---------------------------------------------------------------------------
# Mock origin (Node.js — generates files by file_id prefix)
# ---------------------------------------------------------------------------
.PHONY: mock-origin
mock-origin:
	@bash $(ORIGIN_DIR)/start.sh

# ---------------------------------------------------------------------------
# Origin scenarios (hit mock origin directly, no cache layer)
# ---------------------------------------------------------------------------
.PHONY: origin origin-degrade

origin:
	$(K6) run $(SCRIPT_DIR)/origin-baseline.js

origin-degrade:
	$(K6) run $(SCRIPT_DIR)/origin-degrade.js

# ---------------------------------------------------------------------------
# Cache scenarios (hit edge-cache → mock origin)
# ---------------------------------------------------------------------------
.PHONY: cache-cold cache-warm cache-hot cache-degrade

cache-cold:
	$(K6) run $(SCRIPT_DIR)/cache-cold.js

cache-warm:
	$(K6) run $(SCRIPT_DIR)/cache-warm.js

cache-hot:
	$(K6) run $(SCRIPT_DIR)/cache-hot-key.js

cache-degrade:
	$(K6) run $(SCRIPT_DIR)/cache-degrade.js

# ---------------------------------------------------------------------------
# Clean results
# ---------------------------------------------------------------------------
.PHONY: clean
clean:
	rm -f results/*.json results/*.md
