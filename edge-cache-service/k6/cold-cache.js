import { COLD_VUS, COLD_ITERATIONS, SUMMARY_TREND_STATS } from "./config.js";
import { download, randomFileId, buildReport } from "./helpers.js";

export const options = {
  scenarios: {
    cold_cache: {
      executor: "per-vu-iterations",
      vus: COLD_VUS,
      iterations: COLD_ITERATIONS,
      maxDuration: "10m",
    },
  },
  summaryTrendStats: SUMMARY_TREND_STATS,
  thresholds: {
    http_req_duration: ["p(95)<5000", "p(99)<10000"],
    http_req_failed: ["rate<0.01"],
  },
};

// Each file_id is unique per VU+ITER — no cache hits, pure cold.
export default function () {
  download(randomFileId(__VU, __ITER));
}

export function handleSummary(data) {
  const report = buildReport("Cold Cache", data);
  return { stdout: report, "k6/results/cold-cache.md": report };
}
