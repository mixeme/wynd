// C71.2 is a single-state change of the owner-approved C71.1.
import fs from "node:fs";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { checkWorkshopBaselineFiles } from "./workshop-baseline-guard.mjs";
const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
const { JSDOM } = require("jsdom");
checkWorkshopBaselineFiles();
const fragmentPath = new URL(
  "./workshop-servers.fragment.html",
  import.meta.url,
);
const galleryPath = new URL("./installer.html", import.meta.url);
const manifestPath = new URL(
  "./workshop-review-baselines.json",
  import.meta.url,
);
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
assert.equal(manifest.variants["c71-build"].layoutReview, "approved");
let fragment = fs.readFileSync(fragmentPath, "utf8");
const re =
  /(<script type="application\/json" id="workshop-frame-data">)([\s\S]*?)(<\/script>)/;
const frames = JSON.parse(fragment.match(re)[2]);
const base = frames.find((frame) => frame.id === "c71-build");
const frame = frames.find((frame) => frame.id === "c71-quiet");
const doc = new JSDOM(base.win).window.document;
const win = doc.querySelector(".win");
win.setAttribute("aria-label", frame.title);
win.querySelector(".server-entry.selected").dataset.frame = frame.id;
win.querySelector("[data-c71-note]").textContent =
  "Новых сообщений нет 2 минуты. Связь с сервером есть.";
frame.win = win.outerHTML;
fragment = fragment.replace(
  re,
  (_, start, data, end) =>
    start + JSON.stringify(frames).replaceAll("<", "\\u003c") + end,
);
fs.writeFileSync(fragmentPath, fragment.replace(/\r?\n/g, "\r\n"));
let gallery = fs.readFileSync(galleryPath, "utf8");
const galleryDOM = new JSDOM(gallery, { includeNodeLocations: true });
const oldWin = galleryDOM.window.document.querySelector(
  '[data-frame-id="c71-quiet"] .win',
);
const loc = galleryDOM.nodeLocation(oldWin);
assert.ok(loc);
gallery =
  gallery.slice(0, loc.startOffset) + frame.win + gallery.slice(loc.endOffset);
fs.writeFileSync(galleryPath, gallery.replace(/\r?\n/g, "\r\n"));
manifest.variants[frame.id].base = base.id;
manifest.variants[frame.id].layoutReview = "draft";
manifest.variants[frame.id].note =
  "Derived from approved C71.1; only the last-output timestamp changes to the quiet-build message. Owner review pending.";
delete manifest.variants[frame.id].renderEvidence;
fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
checkWorkshopBaselineFiles();
console.log("C71.2 derived from approved C71.1; layout review pending.");
