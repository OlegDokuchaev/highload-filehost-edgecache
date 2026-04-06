import http from "k6/http";
import { check } from "k6";
import {
  CACHE_URL, WARM_POOL,
  WARM_VUS, WARM_RPS, WARM_DURATION, WARM_START_TIME,
  SUMMARY_TREND_STATS,
} from "../config.js";
import { downloadCache, pickRandom, buildReport } from "../helpers.js";

export const options = {
  scenarios: {
    // Phase 1: seed cache with a fixed pool of files.
    seed: {
      executor: "per-vu-iterations",
      vus: 1,
      iterations: WARM_POOL.length,
      maxDuration: "10m",
      exec: "seed",
    },
    // Phase 2: guaranteed RPS from warm cache.
    warm: {
      executor: "constant-arrival-rate",
      rate: WARM_RPS,
      timeUnit: "1s",
      preAllocatedVUs: WARM_VUS,
      maxVUs: WARM_VUS * 2,
      duration: WARM_DURATION,
      startTime: WARM_START_TIME,
      exec: "warm",
    },
  },
  summaryTrendStats: SUMMARY_TREND_STATS,
  thresholds: {
    http_req_duration: ["p(95)<2000", "p(99)<5000"],
    http_req_failed:   ["rate<0.01"],
  },
};

export function seed() {
  const fileId = WARM_POOL[__ITER % WARM_POOL.length];
  const res = http.get(`${CACHE_URL}/download/${fileId}`);
  check(res, { "seed status 200": (r) => r.status === 200 });
}

export function warm() {
  downloadCache(pickRandom(WARM_POOL), {
    "x-cache is HIT": (r) =>
      (r.headers["X-Cache"] || r.headers["x-cache"]) === "HIT",
  });
}

export function handleSummary(data) {
  const report = buildReport("Cache Warm", data, { showCache: true });
  return {
    stdout: report,
    "results/cache-warm.md":   report,
    "results/cache-warm.json": JSON.stringify(data),
  };
}
