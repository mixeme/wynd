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
let fragment = fs.readFileSync(fragmentPath, "utf8");
const dataRe =
  /(<script type="application\/json" id="workshop-frame-data">)([\s\S]*?)(<\/script>)/;
const frames = JSON.parse(fragment.match(dataRe)[2]);
const base = frames.find((frame) => frame.id === "7в");
const frame = frames.find((frame) => frame.id === "c71-build");
assert.ok(base && frame);
const doc = new JSDOM(base.win).window.document;
const win = doc.querySelector(".win");
win.setAttribute("aria-label", frame.title);
const selected = win.querySelector(".server-entry.selected");
selected.dataset.frame = frame.id;
selected.querySelector("small:last-child").textContent = "Обновление · сборка";
// Keep the approved chrome, title, row markup, spacing and footer unchanged.
// A build has no known percentage; omit the existing determinate progress bar.
win.querySelector(".bar").remove();
const rows = [...win.querySelectorAll(".body > .row")];
assert.equal(rows.length, 5);
const steps = [
  ["✓", "ok", "Получили исходники Wynd 0.48.2"],
  ["…", "w", "Собираем образ Wynd"],
  ["○", "dim", "Копия данных"],
  ["○", "dim", "Запуск 0.48.2"],
  ["○", "dim", "Проверка"],
];
rows.forEach((row, index) => {
  const [mark, tone, title] = steps[index];
  const icon = row.querySelector(".ic");
  icon.className = `ic ${tone}`;
  icon.textContent = mark;
  const content = row.querySelector("div");
  content.textContent = title;
  if (tone === "dim") content.className = "dim";
  else content.removeAttribute("class");
});
const active = rows[1].querySelector("div");
const lastLine = doc.createElement("div");
lastLine.className = "mono";
lastLine.dataset.c71Latest = "";
lastLine.setAttribute("aria-current", "step");
const dockerStep = doc.createElement("strong");
dockerStep.className = "w";
dockerStep.textContent = "#8 [build 3/5] RUN go build ./cmd/wynd";
lastLine.append(dockerStep);
const updated = doc.createElement("div");
updated.className = "dim";
updated.dataset.c71Note = "";
updated.textContent = "Последнее сообщение — сейчас";
active.append(lastLine, updated);
const toggle = win.querySelector(".footer-actions button");
toggle.textContent = "Полный вывод сборки";
toggle.dataset.c71Toggle = "";
toggle.setAttribute("aria-expanded", "false");
const logSource = new JSDOM(frames.find((item) => item.id === "2е").win).window
  .document;
const log = doc.importNode(logSource.querySelector("pre"), true);
log.replaceChildren(
  doc.createTextNode(
    "#1 Получаем исходники v0.48.2\n#2 [build 1/5] FROM golang\n#3 [build 2/5] COPY . .\n#8 [build 3/5] RUN go build ./cmd/wynd",
  ),
);
log.dataset.c71Log = "";
log.hidden = true;
win.querySelector(".body").append(log);
frame.win = win.outerHTML;
fragment = fragment.replace(
  dataRe,
  (_, start, data, end) =>
    start + JSON.stringify(frames).replaceAll("<", "\\u003c") + end,
);
if (!fragment.includes("closest('[data-c71-toggle]')")) {
  const old = "root.addEventListener('click',event=>{";
  assert.equal(fragment.split(old).length, 2);
  fragment = fragment.replace(
    old,
    old +
      `
    const logToggle=event.target.closest('[data-c71-toggle]');
    if(logToggle&&stage.contains(logToggle)){
      const log=stage.querySelector('[data-c71-log]');
      log.hidden=!log.hidden;
      logToggle.setAttribute('aria-expanded',String(!log.hidden));
      return;
    }`,
  );
}
fs.writeFileSync(fragmentPath, fragment.replace(/\r?\n/g, "\r\n"));
let gallery = fs.readFileSync(galleryPath, "utf8");
const galleryDOM = new JSDOM(gallery, { includeNodeLocations: true });
const oldWin = galleryDOM.window.document.querySelector(
  '[data-frame-id="c71-build"] .win',
);
const location = galleryDOM.nodeLocation(oldWin);
assert.ok(location);
gallery =
  gallery.slice(0, location.startOffset) +
  frame.win +
  gallery.slice(location.endOffset);
fs.writeFileSync(galleryPath, gallery.replace(/\r?\n/g, "\r\n"));
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
if (manifest.variants[frame.id].layoutReview !== "approved") {
  manifest.variants[frame.id].layoutReview = "draft";
  manifest.variants[frame.id].note =
    "Owner clarified that emphasis belongs to the current Docker step, not the Wynd stage title. Docker output uses existing mono/strong/w styles; earlier incorrect variants remain unapproved. Layout review pending.";
  delete manifest.variants[frame.id].renderEvidence;
}
fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
checkWorkshopBaselineFiles();
console.log(
  `C71.1 derived from approved 7в without new visual styles; review: ${manifest.variants[frame.id].layoutReview}.`,
);
