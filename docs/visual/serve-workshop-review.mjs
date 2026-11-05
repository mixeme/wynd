// Narrow loopback preview: four in-memory frames, no filesystem browsing.
// node docs/visual/serve-workshop-review.mjs [port]
import http from "node:http";
import fs from "node:fs";
import crypto from "node:crypto";
import { checkWorkshopBaselineFiles } from "./workshop-baseline-guard.mjs";

checkWorkshopBaselineFiles();
const port = Number(process.argv[2] ?? 5196);
if (!Number.isInteger(port) || port < 1024 || port > 65535)
  throw new Error("Invalid port");
const source = fs.readFileSync(
  new URL("./workshop-servers.fragment.html", import.meta.url),
  "utf8",
);
const styles = source.match(
  /<style id="workshop-frame-styles">([\s\S]*?)<\/style>/,
)[1];
const frames = JSON.parse(
  source.match(
    /<script type="application\/json" id="workshop-frame-data">([\s\S]*?)<\/script>/,
  )[1],
);
const sha = (text) => crypto.createHash("sha256").update(text).digest("hex");
const pages = new Map();
for (const [route, id] of [
  ["/", "c71-build"],
  ["/baseline", "7в"],
  ["/quiet", "c71-quiet"],
  ["/error", "c71-error"],
]) {
  const frame = frames.find((item) => item.id === id);
  if (/<(?:img|iframe|link|script)\b|url\s*\(/i.test(frame.win + styles))
    throw new Error("Unexpected resource");
  pages.set(
    route,
    `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${frame.title}</title><style>html,body{margin:0;padding:0} ${styles}\n#workshop-storyboard{width:1024px;max-width:100%;color-scheme:light}</style></head><body><div id="workshop-storyboard" data-source-sha256="${sha(source)}" data-frame-id="${id}" data-frame-sha256="${sha(frame.win)}">${frame.win}</div><script>document.addEventListener('click',event=>{const toggle=event.target.closest('[data-c71-toggle]');if(!toggle)return;const log=document.querySelector('[data-c71-log]');log.hidden=!log.hidden;toggle.setAttribute('aria-expanded',String(!log.hidden));});</script></body></html>`,
  );
}
http
  .createServer((req, res) => {
    if (
      !["GET", "HEAD"].includes(req.method) ||
      req.headers.host !== `127.0.0.1:${port}` ||
      !pages.has(req.url)
    ) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
      "Content-Security-Policy":
        "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'none'; connect-src 'none'; frame-ancestors 'none'; form-action 'none'; base-uri 'none'",
    });
    res.end(req.method === "HEAD" ? undefined : pages.get(req.url));
  })
  .listen(port, "127.0.0.1", () =>
    console.log(`Review: http://127.0.0.1:${port}/; approved base: /baseline`),
  );
