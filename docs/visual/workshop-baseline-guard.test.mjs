import fs from "node:fs";
import assert from "node:assert/strict";
import test from "node:test";
import { checkWorkshopBaselines } from "./workshop-baseline-guard.mjs";

const source = fs.readFileSync(
  new URL("./workshop-servers.fragment.html", import.meta.url),
  "utf8",
);
const frames = JSON.parse(
  source.match(
    /<script type="application\/json" id="workshop-frame-data">([\s\S]*?)<\/script>/,
  )[1],
);
const styles = source.match(
  /<style id="workshop-frame-styles">([\s\S]*?)<\/style>/,
)[1];
const manifest = JSON.parse(
  fs.readFileSync(
    new URL("./workshop-review-baselines.json", import.meta.url),
    "utf8",
  ),
);
const variantId = "c71-build";

test("current shell passes without accepting unreviewed layouts", () => {
  const result = checkWorkshopBaselines(frames, styles, manifest);
  assert.ok(result.pendingVisualReview.some((item) => item.id === variantId));
});

test("approved frame and common styles cannot drift silently", () => {
  assert.throws(
    () => checkWorkshopBaselines(frames, styles + "\n/* changed */", manifest),
    /styles changed/,
  );
  const changed = structuredClone(frames);
  changed.find((frame) => frame.id === manifest.variants[variantId].base).win +=
    " ";
  assert.throws(
    () => checkWorkshopBaselines(changed, styles, manifest),
    /Approved frame/,
  );
});

test("variant cannot change chrome, local shell style or selected profile", () => {
  for (const [from, to, expected] of [
    ["Wynd Мастерская", "Other app", /\.tb differs/],
    [
      'class="win workshop-v2"',
      'class="win workshop-v2" style="font-size:20px"',
      /style differs/,
    ],
    ['data-profile="family"', 'data-profile="new"', /server-context.*differs/],
  ]) {
    const changed = structuredClone(frames);
    const frame = changed.find((item) => item.id === variantId);
    assert.ok(frame.win.includes(from));
    frame.win = frame.win.replace(from, to);
    assert.throws(
      () => checkWorkshopBaselines(changed, styles, manifest),
      expected,
    );
  }
});

test("approved label needs recorded owner decision and actual-render evidence", () => {
  const changed = structuredClone(manifest);
  changed.variants[variantId].layoutReview = "approved";
  assert.throws(
    () => checkWorkshopBaselines(frames, styles, changed),
    /approval requires/,
  );
});

test("body changes are drafts, not mechanically validated visual design", () => {
  const changed = structuredClone(frames);
  const frame = changed.find((item) => item.id === variantId);
  frame.win = frame.win.replace("Собираем Wynd", "Получаем исходники");
  const result = checkWorkshopBaselines(changed, styles, manifest);
  assert.ok(result.pendingVisualReview.some((item) => item.id === variantId));
});
