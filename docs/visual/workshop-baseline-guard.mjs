import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import fs from "node:fs";

const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
const { JSDOM } = require("jsdom");
const hash = (value) => createHash("sha256").update(value).digest("hex");

export function checkWorkshopBaselineFiles() {
  const source = fs.readFileSync(
    new URL("./workshop-servers.fragment.html", import.meta.url),
    "utf8",
  );
  const frames = source.match(
    /<script type="application\/json" id="workshop-frame-data">([\s\S]*?)<\/script>/,
  );
  const styles = source.match(
    /<style id="workshop-frame-styles">([\s\S]*?)<\/style>/,
  );
  assert.ok(
    frames && styles,
    "Workshop preview must expose its source frames and styles",
  );
  const manifest = JSON.parse(
    fs.readFileSync(
      new URL("./workshop-review-baselines.json", import.meta.url),
      "utf8",
    ),
  );
  return checkWorkshopBaselines(JSON.parse(frames[1]), styles[1], manifest);
}

// A structural baseline guard, not visual acceptance or PNG provenance verification.
export function checkWorkshopBaselines(frames, styles, manifest) {
  assert.equal(manifest.version, 1, "Unsupported workshop baseline manifest");
  assert.equal(
    hash(styles.replaceAll("\r\n", "\n")),
    manifest.stylesSha256,
    "Approved workshop styles changed: review the design decision before updating the baseline",
  );
  const byId = new Map(frames.map((frame) => [frame.id, frame]));
  for (const [id, expected] of Object.entries(manifest.frames)) {
    assert.ok(byId.has(id), `Missing approved frame ${id}`);
    assert.equal(
      hash(byId.get(id).win),
      expected.sha256,
      `Approved frame ${id} changed`,
    );
  }
  const pending = [];
  for (const [id, spec] of Object.entries(manifest.variants)) {
    assert.ok(byId.has(id), `Missing variant ${id}`);
    assert.ok(
      manifest.frames[spec.base],
      `Variant ${id} needs a pinned approved base`,
    );
    assert.ok(
      ["draft", "rejected", "approved"].includes(spec.layoutReview),
      `Invalid review state for ${id}`,
    );
    if (spec.layoutReview === "approved") {
      assert.ok(
        spec.approvalEvidence && spec.renderEvidence,
        `${id}: approval requires recorded owner decision and actual-render evidence`,
      );
    } else pending.push({ id, state: spec.layoutReview });
    const base = new JSDOM(byId.get(spec.base).win).window.document;
    const candidate = new JSDOM(byId.get(id).win).window.document;
    for (const selector of [".tb", ".wtabs", ".server-context"]) {
      assert.equal(
        candidate.querySelector(selector)?.outerHTML,
        base.querySelector(selector)?.outerHTML,
        `${id}: ${selector} differs from approved ${spec.base}`,
      );
    }
    for (const selector of [".win", ".main", ".body"]) {
      for (const attr of ["class", "style"]) {
        assert.equal(
          candidate.querySelector(selector)?.getAttribute(attr),
          base.querySelector(selector)?.getAttribute(attr),
          `${id}: ${selector} ${attr} differs from approved ${spec.base}`,
        );
      }
    }
    const selected = candidate.querySelector(".server-entry.selected");
    assert.equal(
      selected?.dataset.server,
      candidate.querySelector(".server-context")?.dataset.profile,
      `${id}: selected server and operation profile disagree`,
    );
    // Only the selected entry's operation destination, status and status tone may change.
    for (const doc of [base, candidate]) {
      const entry = doc.querySelector(".server-entry.selected");
      assert.ok(entry, `${id}: missing selected server`);
      entry.dataset.frame = "[operation]";
      const status = entry.querySelector("small:last-child");
      status.textContent = "[operation status]";
      status.removeAttribute("class");
    }
    assert.equal(
      candidate.querySelector(".server-rail")?.outerHTML,
      base.querySelector(".server-rail")?.outerHTML,
      `${id}: server list differs from approved ${spec.base}`,
    );
  }
  return {
    pinnedFrames: Object.keys(manifest.frames).length,
    pendingVisualReview: pending,
  };
}
