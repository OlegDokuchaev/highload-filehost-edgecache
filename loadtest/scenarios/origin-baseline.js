import {
  HOT_FILE_ID, HOT_VUS, HOT_RPS, HOT_DURATION,
  SUMMARY_TREND_STATS,
} from "../config.js";
import { downloadOrigin, buildReport } from "../helpers.js";

export const options = {
  scenarios: {
    origin_baseline: {
      executor: "constant-arrival-rate",
      rate: HOT_RPS,
      timeUnit: "1s",
      preAllocatedVUs: HOT_VUS,
      maxVUs: HOT_VUS * 2,
      duration: HOT_DURATION,
    },
  },
  summaryTrendStats: SUMMARY_TREND_STATS,
  thresholds: {
    http_req_duration: ["p(95)<5000", "p(99)<10000"],
    http_req_failed:   ["rate<0.01"],
  },
};

// All VUs hit the same file on origin directly — baseline without cache.
export default function () {
  downloadOrigin(HOT_FILE_ID);
}

export function handleSummary(data) {
  const report = buildReport("Origin Baseline", data, {
    extraRows: [
      `| Файл             | ${HOT_FILE_ID}      |`,
    ],
  });
  return {
    stdout: report,
    "results/origin-baseline.md":   report,
    "results/origin-baseline.json": JSON.stringify(data),
  };
}
