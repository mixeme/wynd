import fs from "node:fs";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
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
const hash = (value) => createHash("sha256").update(value).digest("hex");

// Изолированный реестр: состояния draft, rejected и approved проверяются на
// своих кадрах, а не на принятом C71.1. Основа — настоящий кадр, поэтому
// проверки chrome и выбранного профиля остаются теми же.
function syntheticFixture() {
  const base = frames.find((frame) => frame.id === "c71-build").win;
  const frame = (id) => ({ id, win: base });
  return {
    list: [
      frame("synthetic-base"),
      frame("synthetic-draft"),
      frame("synthetic-rejected"),
      frame("synthetic-approved"),
    ],
    registry: {
      version: 1,
      stylesSha256: manifest.stylesSha256,
      frames: { "synthetic-base": { sha256: hash(base) } },
      variants: {
        "synthetic-draft": { base: "synthetic-base", layoutReview: "draft" },
        "synthetic-rejected": {
          base: "synthetic-base",
          layoutReview: "rejected",
        },
        "synthetic-approved": {
          base: "synthetic-base",
          layoutReview: "approved",
          approvalEvidence: "docs/testing/example.md#owner-decision",
          renderEvidence: "docs/testing/example.md#actual-render",
        },
      },
    },
  };
}

function mutateFrame(list, id, change) {
  const changed = structuredClone(list);
  const frame = changed.find((item) => item.id === id);
  const next = change(frame.win);
  assert.notEqual(next, frame.win, `frame ${id} mutation did not apply`);
  frame.win = next;
  return changed;
}

test("real approved baselines pass and nothing is pending", () => {
  const result = checkWorkshopBaselines(frames, styles, manifest);
  assert.equal(result.pinnedFrames, 4);
  assert.deepEqual(result.pendingVisualReview, []);
});

test("synthetic registry covers draft, rejected and approved states", () => {
  const { list, registry } = syntheticFixture();
  const result = checkWorkshopBaselines(list, styles, registry);
  assert.deepEqual(
    result.pendingVisualReview.map((item) => item.state),
    ["draft", "rejected"],
  );

  // Правка тела кадра — черновик, а не механически проверенная вёрстка.
  const withBody = mutateFrame(list, "synthetic-draft", (win) =>
    win.replace("Обновляем", "Получаем исходники"),
  );
  assert.ok(
    checkWorkshopBaselines(withBody, styles, registry).pendingVisualReview.some(
      (item) => item.id === "synthetic-draft",
    ),
  );
});

test("pinned base hash and shared styles cannot drift silently", () => {
  const { list, registry } = syntheticFixture();
  assert.throws(
    () => checkWorkshopBaselines(list, styles + "\n/* changed */", registry),
    /styles changed/,
  );
  const changed = mutateFrame(list, "synthetic-base", (win) => win + " ");
  assert.throws(
    () => checkWorkshopBaselines(changed, styles, registry),
    /Approved frame synthetic-base changed/,
  );
});

test("variant chrome, local shell style and selected profile cannot drift", () => {
  const { list, registry } = syntheticFixture();
  for (const [change, expected] of [
    [(win) => win.replace("Wynd Мастерская", "Other app"), /\.tb differs/],
    [
      (win) =>
        win.replace(
          'class="win workshop-v2"',
          'class="win workshop-v2" style="font-size:20px"',
        ),
      /style differs/,
    ],
    [
      (win) => win.replace('data-profile="family"', 'data-profile="new"'),
      /server-context.*differs/,
    ],
  ]) {
    const changed = mutateFrame(list, "synthetic-draft", change);
    assert.throws(
      () => checkWorkshopBaselines(changed, styles, registry),
      expected,
    );
  }
});

test("approved state needs recorded evidence", () => {
  const { list, registry } = syntheticFixture();
  const noEvidence = structuredClone(registry);
  delete noEvidence.variants["synthetic-approved"].approvalEvidence;
  assert.throws(
    () => checkWorkshopBaselines(list, styles, noEvidence),
    /approval requires/,
  );

  const noRender = structuredClone(registry);
  delete noRender.variants["synthetic-approved"].renderEvidence;
  assert.throws(
    () => checkWorkshopBaselines(list, styles, noRender),
    /approval requires/,
  );
});
