// Локальный двойник формата GitHub Releases; доступен только через loopback/adb reverse.
import { createServer } from "node:http";
import { createReadStream, readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import path from "node:path";

const directory = path.resolve(
  process.argv[2] ?? "D:/android-env/test-releases",
);
const port = Number(process.argv[3] ?? 7697);
const origin = `http://127.0.0.1:${port}`;
const scenarios = new Set([
  "none",
  "young",
  "mature",
  "wrong-signature",
  "tampered",
  "bad-checksum",
  "draft",
  "prerelease",
]);
let scenario = "none";
const counts = { catalog: 0, apk: 0, checksum: 0 };
const artifact = () =>
  path.join(
    directory,
    scenario === "wrong-signature"
      ? "wrong-signature.apk"
      : scenario === "tampered"
        ? "tampered.apk"
        : "candidate-0.1.1.apk",
  );
const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, origin);
    if (url.pathname === "/__test/scenario" && request.method === "POST") {
      let raw = "";
      for await (const chunk of request) {
        raw += chunk;
        if (raw.length > 1024) throw new Error("Body too large");
      }
      const next = JSON.parse(raw).scenario;
      if (!scenarios.has(next)) throw new Error("Unknown scenario");
      scenario = next;
      response.end(JSON.stringify({ scenario }));
    } else if (url.pathname === "/__test/state") {
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({ scenario, counts }));
    } else if (url.pathname === "/repos/mixeme/wynd/releases") {
      counts.catalog++;
      const age = scenario === "young" ? 60 * 60 * 1000 : 73 * 60 * 60 * 1000;
      const apk = `${origin}/releases/download/android-v0.1.1/wynd-android.apk`;
      const release = {
        tag_name: "android-v0.1.1",
        draft: scenario === "draft",
        prerelease: scenario === "prerelease",
        published_at: new Date(Date.now() - age)
          .toISOString()
          .replace(/\.\d{3}Z$/, "Z"),
        assets: [
          { name: "wynd-android.apk", browser_download_url: apk },
          {
            name: "wynd-android.apk.sha256",
            browser_download_url: `${apk}.sha256`,
          },
        ],
      };
      response.setHeader("Content-Type", "application/json");
      response.end(
        JSON.stringify(
          scenario === "none" || Number(url.searchParams.get("page") ?? 1) > 1
            ? []
            : [release],
        ),
      );
    } else if (
      url.pathname ===
      "/releases/download/android-v0.1.1/wynd-android.apk.sha256"
    ) {
      counts.checksum++;
      const hash =
        scenario === "bad-checksum"
          ? "0".repeat(64)
          : createHash("sha256").update(readFileSync(artifact())).digest("hex");
      response.end(`${hash}\n`);
    } else if (
      url.pathname === "/releases/download/android-v0.1.1/wynd-android.apk"
    ) {
      counts.apk++;
      response.setHeader(
        "Content-Type",
        "application/vnd.android.package-archive",
      );
      const stream = createReadStream(artifact());
      stream.on("error", () => {
        if (!response.headersSent) response.writeHead(503);
        response.end();
      });
      stream.pipe(response);
    } else {
      response.writeHead(404).end();
    }
  } catch {
    if (!response.headersSent) response.writeHead(400);
    response.end();
  }
});
server.listen(port, "127.0.0.1", () =>
  console.log(`Android update fixture: ${origin}`),
);
process.on("SIGTERM", () => server.close());
