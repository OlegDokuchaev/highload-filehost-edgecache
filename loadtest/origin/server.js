const http = require("node:http");
const crypto = require("node:crypto");

const PORT = parseInt(process.env.ORIGIN_PORT || "8080");

// File size is encoded in file_id prefix: "1kb-v1i42" → 1024 bytes.
const SIZES = {
  "1kb": 1024,
  "10kb": 10 * 1024,
  "100kb": 100 * 1024,
  "500kb": 500 * 1024,
  "1mb": 1024 * 1024,
  "5mb": 5 * 1024 * 1024,
  "10mb": 10 * 1024 * 1024,
};

function parseSize(fileId) {
  const prefix = fileId.split("-")[0];
  return SIZES[prefix] || 64 * 1024;
}

const server = http.createServer((req, res) => {
  const match = req.url.match(/^\/files\/(.+)$/);
  if (!match) {
    res.writeHead(404);
    res.end("not found");
    return;
  }

  const fileId = decodeURIComponent(match[1]);
  const size = parseSize(fileId);
  const etag = `"${crypto.createHash("md5").update(fileId).digest("hex")}"`;

  // Conditional GET
  if (req.headers["if-none-match"] === etag) {
    res.writeHead(304, { "Cache-Control": "public, max-age=3600" });
    res.end();
    return;
  }

  // Generate deterministic content
  const body = Buffer.alloc(size);
  const seed = crypto.createHash("sha256").update(fileId).digest();
  for (let i = 0; i < size; i++) {
    body[i] = seed[i % seed.length];
  }

  res.writeHead(200, {
    "Content-Type": "application/octet-stream",
    "Content-Length": size,
    "ETag": etag,
    "Cache-Control": "public, max-age=3600",
  });
  res.end(body);
});

server.listen(PORT, () => {
  console.log(`Mock origin on :${PORT} (sizes: ${Object.keys(SIZES).join(", ")})`);
});
