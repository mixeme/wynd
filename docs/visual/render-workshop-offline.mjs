// Offline HTML/CSS diagnostic. This does not control or navigate a browser.
// Usage: node docs/visual/render-workshop-offline.mjs <prince.exe> <output-dir>
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { execFileSync, spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import { checkWorkshopBaselineFiles } from "./workshop-baseline-guard.mjs";

checkWorkshopBaselineFiles();
const [princeArg, outputArg] = process.argv.slice(2);
assert.ok(
  princeArg && outputArg,
  "Provide Prince executable and output directory",
);
const prince = path.resolve(princeArg);
const output = path.resolve(outputArg);
const repo = path.resolve(import.meta.dirname, "../..");
const sourcePath = "docs/visual/workshop-servers.fragment.html";
const source = fs.readFileSync(path.join(repo, sourcePath), "utf8");
const sha = (value) => crypto.createHash("sha256").update(value).digest("hex");
const frames = JSON.parse(
  source.match(
    /<script type="application\/json" id="workshop-frame-data">([\s\S]*?)<\/script>/,
  )[1],
);
const originalStyles = source.match(
  /<style id="workshop-frame-styles">([\s\S]*?)<\/style>/,
)[1];
// Resolve only the existing light palette: Prince does not implement color-scheme.
const styles = originalStyles.replace(
  /light-dark\(([^,()]+),([^()]+)\)/g,
  "$1",
);
assert.ok(!styles.includes("light-dark("));
const width = 1024;
const height = 700;
const marginTop = 50;
fs.mkdirSync(output, { recursive: true });
const records = [];
for (const [id, name] of [
  ["7в", "baseline-7v"],
  ["c71-build", "c71-build"],
]) {
  const frame = frames.find((item) => item.id === id);
  assert.ok(frame);
  assert.ok(
    !/<(?:img|iframe|link|script)\b|url\s*\(/i.test(frame.win + styles),
    "Unexpected external resource",
  );
  const html = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>${id}</title><style>
@page { size: ${width}px ${height}px; margin: ${marginTop}px 0 0; }
html, body { margin: 0; padding: 0; }
${styles}
</style></head><body><div id="workshop-storyboard">${frame.win}</div></body></html>`;
  const htmlPath = path.join(output, `${name}.html`);
  fs.writeFileSync(htmlPath, html);
  const args = [
    "--no-network",
    "--media=screen",
    `--page-size=${width}px ${height}px`,
    `--page-margin=${marginTop}px 0 0`,
    "--raster-pages=first",
    "--raster-dpi=96",
    `--raster-output=${path.join(output, `${name}-%d.png`)}`,
    htmlPath,
    "-o",
    path.join(output, `${name}.pdf`),
  ];
  const render = spawnSync(prince, args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  fs.writeFileSync(
    path.join(output, `${name}.log`),
    (render.stdout ?? "") + (render.stderr ?? ""),
  );
  if (render.error) throw render.error;
  assert.equal(render.status, 0, render.stderr);
  const pngPath = path.join(output, `${name}-1.png`);
  assert.ok(fs.statSync(pngPath).size > 0);
  records.push({
    id,
    name,
    frameSha256: sha(frame.win),
    htmlSha256: sha(html),
    pngSha256: sha(fs.readFileSync(pngPath)),
    command: args,
  });
}
const manifest = {
  source: sourcePath,
  sourceCommit: execFileSync("git", ["rev-parse", "HEAD"], {
    cwd: repo,
    encoding: "utf8",
  }).trim(),
  sourceSha256: sha(source),
  approvedBaseCommit: "b827aff829e43857700e1eacac833862e3ea1a06",
  candidateCommit: "c5a98dff",
  renderer: execFileSync(prince, ["--version"], { encoding: "utf8" }).trim(),
  page: { width, height, marginTop },
  frameWidth: width,
  theme: "existing light palette",
  originalStylesSha256: sha(originalStyles),
  renderedStylesSha256: sha(styles),
  records,
  limitations:
    "Offline HTML/CSS render, not browser/WebView validation. Native form controls can differ. Prince evaluation mark retained above the UI. Layout approval remains pending.",
};
fs.writeFileSync(
  path.join(output, "source.json"),
  JSON.stringify(manifest, null, 2) + "\n",
);
console.log(
  "Offline source renders saved; browser fidelity and owner layout approval remain pending.",
);
