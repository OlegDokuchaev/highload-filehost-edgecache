import {
  HOT_FILE_ID, HOT_VUS, HOT_RPS, HOT_DURATION, SUMMARY_TREND_STATS,
} from "./config.js";
import { download, buildReport, cacheStats } from "./helpers.js";

export const options = {
  scenarios: {
    hot_key: {
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
    http_req_duration: ["p(95)<2000", "p(99)<5000"],
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  download(HOT_FILE_ID);
}

export function handleSummary(data) {
  const s = cacheStats(data);
  const coalescingEffect =
    s.misses <= 1 ? "effective" : `${s.misses} origin fetches (expected 1)`;

  const report = buildReport("Hot Key", data, [
    `| Горячий файл     | ${HOT_FILE_ID}      |`,
    `| Coalescing       | ${coalescingEffect} |`,
  ]);

  return { stdout: report, "k6/results/hot-key.md": report };
}
