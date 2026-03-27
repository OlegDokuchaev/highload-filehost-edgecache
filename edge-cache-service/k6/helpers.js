import http from "k6/http";
import { check } from "k6";
import { Counter, Trend } from "k6/metrics";
import { BASE_URL, SIZE_POOL } from "./config.js";

// ---------------------------------------------------------------------------
// Shared metrics
// ---------------------------------------------------------------------------
export const cacheHits = new Counter("cache_hits");
export const cacheMisses = new Counter("cache_misses");
export const cacheRevalidated = new Counter("cache_revalidated");
export const downloadLatency = new Trend("download_latency", true);

// ---------------------------------------------------------------------------
// Request + tracking
// ---------------------------------------------------------------------------
export function download(fileId, checks) {
  const res = http.get(`${BASE_URL}/download/${fileId}`);

  downloadLatency.add(res.timings.duration);

  const xCache = res.headers["X-Cache"] || res.headers["x-cache"] || "";
  if (xCache === "HIT") cacheHits.add(1);
  else if (xCache === "MISS") cacheMisses.add(1);
  else if (xCache === "REVALIDATED") cacheRevalidated.add(1);

  check(res, {
    "status is 200": (r) => r.status === 200,
    "has body": (r) => r.body && r.body.length > 0,
    ...checks,
  });

  return res;
}

// ---------------------------------------------------------------------------
// File ID generation
// ---------------------------------------------------------------------------
export function randomFileId(vu, iter) {
  const prefix = pickWeightedRandom();
  return `${prefix}-v${vu}i${iter}`;
}

function pickWeightedRandom() {
  const totalWeight = SIZE_POOL.reduce((s, e) => s + e.weight, 0);
  const r = Math.random() * totalWeight;
  let acc = 0;
  for (const entry of SIZE_POOL) {
    acc += entry.weight;
    if (r < acc) return entry.prefix;
  }
  return SIZE_POOL[0].prefix;
}

export function pickRandom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// ---------------------------------------------------------------------------
// Summary helpers
// ---------------------------------------------------------------------------
function countMetric(data, name) {
  return data.metrics[name] ? data.metrics[name].values.count || 0 : 0;
}

function percentile(data, name, pct) {
  const m = data.metrics[name];
  if (!m || !m.values) return "N/A";
  const v = m.values[pct];
  return v != null ? v.toFixed(1) : "N/A";
}

function calcRps(data) {
  const m = data.metrics["http_reqs"];
  if (!m) return "N/A";
  return (m.values.rate || 0).toFixed(1);
}

export function cacheStats(data) {
  const hits = countMetric(data, "cache_hits");
  const misses = countMetric(data, "cache_misses");
  const revalidated = countMetric(data, "cache_revalidated");
  const total = hits + misses + revalidated;
  const hitRatio = total > 0 ? ((hits / total) * 100).toFixed(1) : "0.0";
  // Only HIT avoids origin. MISS and REVALIDATED both hit origin.
  const originOffload =
    total > 0 ? ((hits / total) * 100).toFixed(1) : "0.0";

  return { hits, misses, revalidated, total, hitRatio, originOffload };
}

export function buildReport(title, data, extraRows) {
  const s = cacheStats(data);
  const p95 = percentile(data, "download_latency", "p(95)");
  const p99 = percentile(data, "download_latency", "p(99)");
  const rps = calcRps(data);

  const rows = [
    ...(extraRows || []),
    `| Всего запросов   | ${s.total}          |`,
    `| RPS              | ${rps}              |`,
    `| p95 задержка     | ${p95} ms           |`,
    `| p99 задержка     | ${p99} ms           |`,
    `| Cache HIT        | ${s.hits}           |`,
    `| Cache MISS       | ${s.misses}         |`,
    `| REVALIDATED      | ${s.revalidated}    |`,
    `| Hit ratio        | ${s.hitRatio}%      |`,
    `| Origin offload   | ${s.originOffload}% |`,
  ];

  return [
    `# ${title}`,
    "",
    `| Метрика          | Значение            |`,
    `|------------------|---------------------|`,
    ...rows,
    "",
  ].join("\n");
}
