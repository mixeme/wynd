// C71.3 reuses approved C71.1 and existing controls for the build failure state.
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
const frame = frames.find((frame) => frame.id === "c71-error");
const previous = new JSDOM(frame.win).window.document;
const doc = new JSDOM(base.win).window.document;
const win = doc.querySelector(".win");
win.setAttribute("aria-label", frame.title);
const selected = win.querySelector(".server-entry.selected");
selected.dataset.frame = frame.id;
selected.querySelector("small:last-child").textContent =
  "Обновление · ошибка сборки";
win.querySelector(".body > h2").textContent = "Не удалось собрать Wynd";
const current = win.querySelector("[data-c71-latest]");
const row = current.closest(".row");
row.querySelector(".ic").className = "ic no";
row.querySelector(".ic").textContent = "!";
current.removeAttribute("aria-current");
current.querySelector("strong").className = "no";
current.querySelector("strong").textContent =
  "#8 ERROR: no space left on device";
win.querySelector("[data-c71-note]").textContent =
  "На сервере закончилось место. Работающий Wynd не меняли.";
const recovery = doc.createElement("div");
recovery.className = "dim";
recovery.textContent =
  "Освободите место. Перед повтором проверим состояние сборки.";
current.parentElement.append(recovery);
const returnAction = [...previous.querySelectorAll("button")].find(
  (button) => button.textContent.trim() === "К плану обновления",
);
assert.ok(returnAction);
win
  .querySelector(".footer-actions")
  .prepend(doc.importNode(returnAction, true));
win.querySelector("[data-c71-log]").textContent +=
  "\n#8 ERROR: no space left on device\nСборка завершилась с кодом 1";
frame.win = win.outerHTML;
fragment = fragment.replace(
  re,
  (_, start, data, end) =>
    start + JSON.stringify(frames).replaceAll("<", "\\u003c") + end,
);
fs.writeFileSync(fragmentPath, fragment.replace(/\r?\n/g, "\r\n"));
let gallery = fs.readFileSync(galleryPath, "utf8");
const galleryDOM = new JSDOM(gallery, { includeNodeLocations: true });
const loc = galleryDOM.nodeLocation(
  galleryDOM.window.document.querySelector('[data-frame-id="c71-error"] .win'),
);
assert.ok(loc);
gallery =
  gallery.slice(0, loc.startOffset) + frame.win + gallery.slice(loc.endOffset);
fs.writeFileSync(galleryPath, gallery.replace(/\r?\n/g, "\r\n"));
manifest.variants[frame.id].base = base.id;
if (manifest.variants[frame.id].layoutReview !== "approved") {
  manifest.variants[frame.id].layoutReview = "draft";
  manifest.variants[frame.id].note =
    "Build error state derived from approved C71.1; owner review pending.";
  delete manifest.variants[frame.id].renderEvidence;
}
fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
checkWorkshopBaselineFiles();
console.log(
  `C71.3 derived from approved C71.1; review: ${manifest.variants[frame.id].layoutReview}.`,
);
