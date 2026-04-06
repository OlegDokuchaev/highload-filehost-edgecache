// ---------------------------------------------------------------------------
// Environment-driven parameters
// ---------------------------------------------------------------------------
export const ORIGIN_URL = __ENV.ORIGIN_URL || "http://localhost:8080";
export const CACHE_URL  = __ENV.CACHE_URL  || "http://localhost:3000";

// ---------------------------------------------------------------------------
// Cache cold scenario
// ---------------------------------------------------------------------------
export const COLD_VUS        = parseInt(__ENV.COLD_VUS || "20");
export const COLD_ITERATIONS = parseInt(__ENV.COLD_ITERATIONS || "1000");

// ---------------------------------------------------------------------------
// Cache warm scenario
// ---------------------------------------------------------------------------
export const WARM_VUS        = parseInt(__ENV.WARM_VUS || "30");
export const WARM_RPS        = parseInt(__ENV.WARM_RPS || "300");
export const WARM_DURATION   = __ENV.WARM_DURATION || "60s";
export const WARM_START_TIME = __ENV.WARM_START_TIME || "10s";

// ---------------------------------------------------------------------------
// Hot key scenario (both origin and cache)
// ---------------------------------------------------------------------------
export const HOT_VUS      = parseInt(__ENV.HOT_VUS || "50");
export const HOT_RPS      = parseInt(__ENV.HOT_RPS || "500");
export const HOT_DURATION = __ENV.HOT_DURATION || "60s";
export const HOT_FILE_ID  = __ENV.HOT_FILE_ID || "1mb-hot-0";

// ---------------------------------------------------------------------------
// File sizes (prefix → used in file_id, mock origin parses it)
// ---------------------------------------------------------------------------
export const SIZE_POOL = [
  { prefix: "1kb",  weight: 3 },
  { prefix: "10kb", weight: 3 },
  { prefix: "100kb",weight: 2 },
  { prefix: "1mb",  weight: 1 },
  { prefix: "5mb",  weight: 1 },
];

// ---------------------------------------------------------------------------
// Warm pool — fixed set of file IDs to seed and re-read
// ---------------------------------------------------------------------------
export const WARM_POOL_SIZE = parseInt(__ENV.WARM_POOL_SIZE || "200");
export const WARM_POOL = buildWarmPool(WARM_POOL_SIZE);

function buildWarmPool(count) {
  const pool = [];
  for (let i = 0; i < count; i++) {
    const size = pickWeightedPrefix(i);
    pool.push(`${size}-warm-${i}`);
  }
  return pool;
}

function pickWeightedPrefix(seed) {
  const totalWeight = SIZE_POOL.reduce((s, e) => s + e.weight, 0);
  let r = seed % totalWeight;
  for (const entry of SIZE_POOL) {
    r -= entry.weight;
    if (r < 0) return entry.prefix;
  }
  return SIZE_POOL[0].prefix;
}

// ---------------------------------------------------------------------------
// Degradation (breakpoint) scenarios
// ---------------------------------------------------------------------------
export const DEGRADE_FILE_ID  = __ENV.DEGRADE_FILE_ID || "1mb-degrade-0";
export const DEGRADE_START_RPS = parseInt(__ENV.DEGRADE_START_RPS || "50");
export const DEGRADE_STEP_RPS  = parseInt(__ENV.DEGRADE_STEP_RPS  || "100");
export const DEGRADE_STEPS     = parseInt(__ENV.DEGRADE_STEPS     || "20");
export const DEGRADE_STAGE_SEC = parseInt(__ENV.DEGRADE_STAGE_SEC || "30");
export const DEGRADE_PRE_VUS   = parseInt(__ENV.DEGRADE_PRE_VUS  || String(Math.max(DEGRADE_START_RPS, 200)));
export const DEGRADE_MAX_VUS   = parseInt(__ENV.DEGRADE_MAX_VUS  || String((DEGRADE_START_RPS + DEGRADE_STEP_RPS * DEGRADE_STEPS) * 2));
export const DEGRADE_P99_LIMIT = __ENV.DEGRADE_P99_LIMIT || "500";    // ms
export const DEGRADE_ERR_LIMIT = __ENV.DEGRADE_ERR_LIMIT || "0.01";   // rate
export const DEGRADE_ABORT_DELAY = __ENV.DEGRADE_ABORT_DELAY || "10s";

export function buildDegradeStages() {
  const arr = [];
  let r = DEGRADE_START_RPS;
  for (let i = 0; i < DEGRADE_STEPS; i++) {
    arr.push({ target: r, duration: `${DEGRADE_STAGE_SEC}s` });
    r += DEGRADE_STEP_RPS;
  }
  return arr;
}

// ---------------------------------------------------------------------------
// Summary stats
// ---------------------------------------------------------------------------
export const SUMMARY_TREND_STATS = [
  "avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count",
];
