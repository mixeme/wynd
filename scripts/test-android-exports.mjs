// Сохраняет только собственный бинарный образец/общедоступный manifest тестового сервера.
import assert from "node:assert/strict";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { createHash } from "node:crypto";
import { AndroidDevice, waitFor } from "./android-device.mjs";

const [serial, phase] = process.argv.slice(2);
assert(["blob", "url", "cancel", "save", "result"].includes(phase), "Specify phase");
const device = new AndroidDevice(serial);
const folder = "wynd-android-export-test";
if (["blob", "url", "cancel"].includes(phase)) {
  device.adb("shell", "mkdir", "-p", `/sdcard/Download/${folder}`);
  device.adb("shell", "am", "force-stop", device.packageName);
  device.start();
  const cdp = await device.connect();
  try {
    await cdp.navigate(device.origin + "/");
    const require = createRequire(new URL("../web/package.json", import.meta.url));
    const { transformSync } = require("esbuild");
    const source = readFileSync(new URL("../web/src/lib/android/files.ts", import.meta.url), "utf8");
    const compiled = transformSync(source, { loader: "ts", format: "iife", globalName: "__wyndExportProbeModule", target: "es2018" }).code;
    await cdp.evaluate(compiled);
    const kind = phase === "url" ? "url" : "blob";
    const filename = `wynd-export-test-${phase}.${phase === "url" ? "zip" : "bin"}`;
    await cdp.evaluate(`(() => {
      window.__wyndExportProbe={phase:${JSON.stringify(phase)},filename:${JSON.stringify(filename)},state:'pending'};
      const bytes=Uint8Array.from({length:1048583},(_,i)=>i%251);
      const pending=${JSON.stringify(kind)}==='blob'
        ? __wyndExportProbeModule.saveAndroidBlob(new Blob([bytes],{type:'application/octet-stream'}),${JSON.stringify(filename)})
        : __wyndExportProbeModule.saveAndroidDownload('/manifest.webmanifest',${JSON.stringify(filename)});
      pending.then(handled=>Object.assign(__wyndExportProbe,{state:'done',handled}),error=>Object.assign(__wyndExportProbe,{state:'error',error:String(error)}));
    })()`);
    console.log(JSON.stringify({ phase, filename, sourceSha256: createHash("sha256").update(source).digest("hex") }));
  } finally { cdp.close(); }
} else if (phase === "save") {
  let nodes = device.hierarchy();
  const inOwnFolder = () => nodes.some(n => n.text === folder && /breadcrumb_text/.test(n["resource-id"]));
  if (!inOwnFolder()) {
    const root = nodes.find(n => /breadcrumb_text/.test(n["resource-id"]) && /Загрузить|Загрузки|Downloads/i.test(n.text));
    if (root && root.enabled !== "false") device.tapNode(root);
    else {
      device.tapNode(nodes.find(n => /корневые папки|Show roots/i.test(n["content-desc"])));
      nodes = device.hierarchy();
      device.tapNode(nodes.find(n => /^(Загрузить|Загрузки|Downloads)$/.test(n.text) && n["resource-id"] === "android:id/title"));
    }
    nodes = device.hierarchy();
    const target = nodes.find(n => n.text === folder && n["resource-id"] === "android:id/title");
    assert(target, "Own folder not visible in Downloads");
    const box = node => /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/.exec(node.bounds).slice(1).map(Number);
    for (let attempt = 0; attempt < 12; attempt++) {
      nodes = device.hierarchy();
      const focus = nodes.find(n => n.focused === "true" && /item_root$/.test(n["resource-id"]));
      if (focus) {
        const [x1,y1,x2,y2] = box(focus), [tx1,ty1,tx2,ty2] = box(target);
        if (x1 <= tx1 && y1 <= ty1 && x2 >= tx2 && y2 >= ty2) { device.adb("shell", "input", "keyevent", "66"); break; }
      }
      device.adb("shell", "input", "keyevent", "20");
    }
    nodes = device.hierarchy();
  }
  assert(inOwnFolder(), "Select the own test folder before saving");
  device.screenshot("dev/screens/shots/android-export-save-picker.png");
  const save = nodes.find((node) => /сохранить|save/i.test(node.text) && node.enabled === "true");
  assert(save, "Save action unavailable");
  device.tapNode(save);
  console.log("Selected own document destination");
} else {
  device.adb("shell", "cmd", "statusbar", "collapse");
  device.start();
  const cdp = await device.connect();
  try {
    await waitFor(async () => (await cdp.evaluate("window.__wyndExportProbe?.state")) !== "pending", "Export did not finish", 60000);
    const result = await cdp.evaluate("window.__wyndExportProbe");
    assert.equal(result?.state, "done", JSON.stringify(result));
    assert(["blob", "url", "cancel"].includes(result.phase));
    assert.equal(result.filename, `wynd-export-test-${result.phase}.${result.phase === "url" ? "zip" : "bin"}`);
    console.log(JSON.stringify(result));
    if (result.phase !== "cancel") {
      assert(device.hierarchy().some(n => n.package === device.packageName), "Saving unexpectedly opened another application");
      const file = `/sdcard/Download/${folder}/${result.filename}`;
      let expected;
      if (result.phase === "url") {
        const response = await fetch(device.origin + "/manifest.webmanifest");
        assert(response.ok);
        expected = createHash("sha256").update(Buffer.from(await response.arrayBuffer())).digest("hex");
      }
      else expected = createHash("sha256").update(Uint8Array.from({ length: 1048583 }, (_, i) => i % 251)).digest("hex");
      const actual = device.adb("shell", "sha256sum", file).split(/\s+/)[0];
      assert.equal(actual, expected, "Saved file differs; not removing it");
      device.adb("shell", "input", "keyevent", "224");
      device.adb("shell", "cmd", "statusbar", "expand-notifications");
      let notification;
      for (let attempt = 0; attempt < 6; attempt++) {
        const nodes = device.hierarchy();
        notification = nodes.find(n => n.text === result.filename);
        if (notification) break;
        const scroller = nodes.find(n => /notification_stack_scroller$/.test(n["resource-id"]));
        assert(scroller, "System notification list unavailable");
        const [,x1,y1,x2,y2] = /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/.exec(scroller.bounds).map(Number);
        const x = Math.round((x1+x2)/2);
        device.adb("shell", "input", "swipe", String(x), String(Math.round(y1+(y2-y1)*.85)), String(x), String(Math.round(y1+(y2-y1)*.3)), "400");
      }
      assert(notification, "Own saved-file notification absent");
      device.tapNode(notification);
      let handler;
      await waitFor(() => {
        const activities = device.adb("shell", "dumpsys", "activity", "activities");
        handler = /mResumedActivity:.*?\s(?:u\d+\s+)?([\w.]+\/[\w.$]+)/.exec(activities)?.[1];
        return handler && handler !== device.component && !/^(com\.oppo\.launcher|com\.android\.systemui)\//.test(handler);
      }, "Notification action did not open a handler/chooser", 10000);
      device.start();
      device.adb("shell", "rm", file); // Только свой файл после точного совпадения SHA.
      const output = path.join(process.env.WYND_ANDROID_ENV ?? "D:/android-env", "test-exports");
      mkdirSync(output, { recursive: true });
      writeFileSync(path.join(output, result.phase + ".json"), JSON.stringify({ ...result, sha256: actual, handler, coldNotification: false, at: new Date().toISOString() }, null, 2));
      console.log(JSON.stringify({ sha256: actual, handler, cleaned: file }));
    }
  } finally { cdp.close(); }
}
