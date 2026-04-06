import http from "k6/http";
import { check } from "k6";
import {
  CACHE_URL, DEGRADE_FILE_ID, DEGRADE_START_RPS,
  DEGRADE_PRE_VUS, DEGRADE_MAX_VUS,
  DEGRADE_P99_LIMIT, DEGRADE_ERR_LIMIT, DEGRADE_ABORT_DELAY,
  buildDegradeStages, SUMMARY_TREND_STATS,
} from "../config.js";
import { downloadCache, buildReport, cacheStats } from "../helpers.js";

const STAGES = buildDegradeStages();

export const options = {
  scenarios: {
    // Phase 1: seed the file into cache (1 request → MISS → cached).
    seed: {
      executor: "per-vu-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "30s",
      exec: "seed",
    },
    // Phase 2: ramp RPS on the cached file until edge-cache degrades.
    find_degradation_point: {
      executor: "ramping-arrival-rate",
      startRate: DEGRADE_START_RPS,
      timeUnit: "1s",
      preAllocatedVUs: DEGRADE_PRE_VUS,
      maxVUs: DEGRADE_MAX_VUS,
      stages: STAGES,
      startTime: "5s",
      exec: "degrade",
      tags: { scenario: "degradation" },
    },
  },
  summaryTrendStats: SUMMARY_TREND_STATS,
  thresholds: {
    "http_req_duration{scenario:find_degradation_point}": [
      { threshold: `p(99)<${DEGRADE_P99_LIMIT}`, abortOnFail: true, delayAbortEval: DEGRADE_ABORT_DELAY },
    ],
    "http_req_failed{scenario:find_degradation_point}": [
      { threshold: `rate<${DEGRADE_ERR_LIMIT}`, abortOnFail: true, delayAbortEval: DEGRADE_ABORT_DELAY },
    ],
  },
};

export function seed() {
  const res = http.get(`${CACHE_URL}/download/${DEGRADE_FILE_ID}`);
  check(res, { "seed status 200": (r) => r.status === 200 });
}

// All requests hit the same cached file — pure cache performance.
export function degrade() {
  downloadCache(DEGRADE_FILE_ID);
}

export function handleSummary(data) {
  const s = cacheStats(data);
  const stagesList = STAGES
    .map((st) => `${st.target} RPS (${st.duration})`)
    .join(" → ");

  const report = buildReport("Cache — точка деградации", data, {
    showCache: true,
    extraRows: [
      `| Файл             | ${DEGRADE_FILE_ID}  |`,
      `| Ступени          | ${stagesList} |`,
      `| Abort p99 >=     | ${DEGRADE_P99_LIMIT} ms |`,
      `| Abort error >=   | ${(Number(DEGRADE_ERR_LIMIT) * 100).toFixed(0)}% |`,
    ],
  });

  return {
    stdout: report,
    "results/cache-degrade.md":   report,
    "results/cache-degrade.json": JSON.stringify(data),
  };
}
