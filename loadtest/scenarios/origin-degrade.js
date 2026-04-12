import {
  DEGRADE_FILE_ID, DEGRADE_START_RPS, DEGRADE_PRE_VUS, DEGRADE_MAX_VUS,
  DEGRADE_P99_LIMIT, DEGRADE_ERR_LIMIT, DEGRADE_ABORT_DELAY,
  buildDegradeStages, SUMMARY_TREND_STATS,
} from "../config.js";
import { downloadOrigin, buildReport } from "../helpers.js";

const STAGES = buildDegradeStages();

export const options = {
  scenarios: {
    find_degradation_point: {
      executor: "ramping-arrival-rate",
      startRate: DEGRADE_START_RPS,
      timeUnit: "1s",
      preAllocatedVUs: DEGRADE_PRE_VUS,
      maxVUs: DEGRADE_MAX_VUS,
      stages: STAGES,
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

// Single file, RPS ramps up until origin degrades.
export default function () {
  downloadOrigin(DEGRADE_FILE_ID);
}

export function handleSummary(data) {
  const stagesList = STAGES
    .map((s) => `${s.target} RPS (${s.duration})`)
    .join(" → ");

  const report = buildReport("Origin — точка деградации", data, {
    extraRows: [
      `| Файл             | ${DEGRADE_FILE_ID}  |`,
      `| Ступени          | ${stagesList} |`,
      `| Abort p99 >=     | ${DEGRADE_P99_LIMIT} ms |`,
      `| Abort error >=   | ${(Number(DEGRADE_ERR_LIMIT) * 100).toFixed(0)}% |`,
    ],
  });

  return {
    stdout: report,
    "results/origin-degrade.md":   report,
    "results/origin-degrade.json": JSON.stringify(data),
  };
}
