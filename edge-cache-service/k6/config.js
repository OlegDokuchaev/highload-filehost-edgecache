// ---------------------------------------------------------------------------
// Environment-driven parameters
// ---------------------------------------------------------------------------
export const BASE_URL = __ENV.BASE_URL || "http://localhost:3000";

export const WARM_VUS = parseInt(__ENV.WARM_VUS || "20");
export const WARM_RPS = parseInt(__ENV.WARM_RPS || "200");
export const WARM_DURATION = __ENV.WARM_DURATION || "30s";
export const WARM_START_TIME = __ENV.WARM_START_TIME || "10s";

export const HOT_VUS = parseInt(__ENV.HOT_VUS || "50");
export const HOT_RPS = parseInt(__ENV.HOT_RPS || "500");
export const HOT_DURATION = __ENV.HOT_DURATION || "30s";
export const HOT_FILE_ID = __ENV.HOT_FILE_ID || "1mb-hot-0";

export const COLD_VUS = parseInt(__ENV.COLD_VUS || "10");
export const COLD_ITERATIONS = parseInt(__ENV.COLD_ITERATIONS || "500");

// ---------------------------------------------------------------------------
// File sizes (prefix → used in file_id, origin parses it)
// ---------------------------------------------------------------------------
export const SIZE_POOL = [
  { prefix: "1kb", weight: 3 },
  { prefix: "10kb", weight: 3 },
  { prefix: "100kb", weight: 2 },
  { prefix: "1mb", weight: 1 },
  { prefix: "5mb", weight: 1 },
];

// Warm cache — fixed pool of file IDs to seed and then re-read.
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
// Summary stats
// ---------------------------------------------------------------------------
export const SUMMARY_TREND_STATS = [
  "avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count",
];
