// Только синтетический JPEG из репозитория; личные снимки и текущее положение не читаются.
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import path from "node:path";
import { AndroidDevice, waitFor } from "./android-device.mjs";

const [serial, phase, label = phase] = process.argv.slice(2);
const device = new AndroidDevice(serial);
const folder = "wynd-android-geo-test",
  filename = "wynd-android-exif-gps-fixture.jpg";
const fixture = readFileSync(
  new URL("../web/src/test/fixtures/exif-gps.jpg", import.meta.url),
);
if (phase === "repeat") {
  const cdp = await device.connect();
  try {
    const response = await cdp.send("Runtime.evaluate", {
      userGesture: true,
      awaitPromise: true,
      returnByValue: true,
      expression: `(async()=>{
      if(!window.__wyndGeoInput)throw Error('Start original probe first');
      window.__wyndGeoProbe={state:'waiting',label:${JSON.stringify(label)}};
      __wyndGeoInput.accept=await __wyndPhotoProbeModule.originalPhotoAccept('image/*,video/*');__wyndGeoInput.click();
    })()`,
    });
    assert(!response.exceptionDetails, "Repeat chooser could not start");
  } finally {
    cdp.close();
  }
} else if (phase === "basic" || phase === "original") {
  device.adb("shell", "am", "force-stop", device.packageName);
  device.adb("shell", "input", "keyevent", "224");
  device.adb(
    "shell",
    "am",
    "start",
    "-W",
    "-f",
    "0x14000000",
    "-n",
    device.component,
  );
  const cdp = await device.connect();
  try {
    await cdp.navigate(device.origin + "/");
    const require = createRequire(
      new URL("../web/package.json", import.meta.url),
    );
    const { buildSync } = require("esbuild");
    const compiled = buildSync({
      stdin: {
        contents:
          "export { originalPhotoAccept, applyPhotoLocationChoice } from './src/lib/android/photos'; export { readExif } from './src/lib/media/exif';",
        resolveDir: path.resolve("web"),
        loader: "ts",
      },
      bundle: true,
      write: false,
      format: "iife",
      globalName: "__wyndPhotoProbeModule",
      target: "es2018",
      alias: {
        "$lib/android/photos": path.resolve("web/src/lib/android/photos.ts"),
      },
    }).outputFiles[0].text;
    await cdp.evaluate(compiled);
    const execution = await cdp.send("Runtime.evaluate", {
      userGesture: true,
      awaitPromise: true,
      returnByValue: true,
      expression: `(async()=>{
      const input=document.createElement('input');input.type='file';input.hidden=true;document.body.append(input);window.__wyndGeoInput=input;
      window.__wyndGeoProbe={state:'waiting',label:${JSON.stringify(label)}};
      input.onchange=async()=>{try {const file=input.files[0];if(!file){__wyndGeoProbe.state='cancelled';return;}
        const chosenAccept=input.accept;input.value='';input.accept=accept;
        if(${JSON.stringify(phase)}==='original') await __wyndPhotoProbeModule.applyPhotoLocationChoice([file],chosenAccept);
        const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',await file.arrayBuffer())),b=>b.toString(16).padStart(2,'0')).join('');
        Object.assign(__wyndGeoProbe,{state:'selected',name:file.name,size:file.size,type:file.type,sha256:hash,hints:await __wyndPhotoProbeModule.readExif(file)});
      }catch(error){Object.assign(__wyndGeoProbe,{state:'error',error:String(error)});}};
      input.oncancel=()=>{__wyndGeoProbe.state='cancelled';};
      const accept=${JSON.stringify(label)}.includes('gallery')?'image/*':${JSON.stringify(label)}.includes('file')?'*/*':'image/*,video/*';
      input.accept=${JSON.stringify(phase)}==='original'?await __wyndPhotoProbeModule.originalPhotoAccept(accept):accept;input.click();
    })()`,
    });
    assert(
      !execution.exceptionDetails,
      execution.exceptionDetails?.exception?.description ??
        "Photo chooser could not start",
    );
    console.log(
      JSON.stringify({
        phase,
        label,
        expectedSha256: createHash("sha256").update(fixture).digest("hex"),
      }),
    );
  } finally {
    cdp.close();
  }
} else if (phase === "select" || phase === "gallery") {
  const enter = (name) => {
    const list = device
      .hierarchy()
      .find((n) => /sub_menu_list$/.test(n["resource-id"]));
    if (list) device.tapNode(list);
    for (let i = 0; i < 12; i++) {
      const nodes = device.hierarchy(),
        target = nodes.find(
          (n) => n.text === name && n["resource-id"] === "android:id/title",
        );
      assert(target, "Own test folder/file is not visible");
      const focus = nodes.find(
        (n) => n.focused === "true" && /item_root$/.test(n["resource-id"]),
      );
      const box = (n) =>
        /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/.exec(n.bounds).slice(1).map(Number);
      if (focus) {
        const [x1, y1, x2, y2] = box(focus),
          [a, b, c, d] = box(target);
        if (x1 <= a && y1 <= b && x2 >= c && y2 >= d) {
          device.adb("shell", "input", "keyevent", "66");
          return;
        }
      }
      device.adb("shell", "input", "keyevent", "20");
    }
    throw Error("Could not focus own fixture safely");
  };
  let nodes = device.hierarchy();
  if (phase === "gallery") {
    device.tapNode(
      nodes.find((n) => /корневые папки|Show roots/i.test(n["content-desc"])),
    );
    nodes = device.hierarchy();
    const images = nodes.find(
      (n) =>
        /^(Изображения|Images)$/.test(n.text) &&
        n["resource-id"] === "android:id/title",
    );
    assert(
      images,
      "This DocumentsUI does not expose Images root; gallery-provider path cannot be verified here",
    );
    device.tapNode(images);
    nodes = device.hierarchy();
    device.tapNode(
      nodes.find(
        (n) =>
          /menu_search$/.test(n["resource-id"]) ||
          /^(Поиск|Search)$/.test(n["content-desc"]),
      ),
    );
    nodes = device.hierarchy();
    device.tapNode(
      nodes.find((n) => /search_src_text$/.test(n["resource-id"])),
    );
    device.adb("shell", "input", "text", filename);
    device.adb("shell", "input", "keyevent", "66");
    nodes = device.hierarchy();
  } else if (!nodes.some((n) => n.text === filename)) {
    device.tapNode(
      nodes.find((n) => /корневые папки|Show roots/i.test(n["content-desc"])),
    );
    nodes = device.hierarchy();
    const storage = nodes.find(
      (n) =>
        /CPH1931|Внутренняя память|Internal storage/.test(n.text) &&
        n["resource-id"] === "android:id/title",
    );
    assert(
      storage,
      "Internal-storage root unavailable; show it in DocumentsUI",
    );
    device.tapNode(storage);
    enter("Download");
    enter(folder);
  }
  const ownNodes = device.hierarchy();
  assert(
    ownNodes.some(
      (n) => n.text === filename && n["resource-id"] === "android:id/title",
    ),
    "Own fixture not visible before capture",
  );
  if (phase === "gallery")
    assert.equal(
      ownNodes.filter((n) => n["resource-id"] === "android:id/title").length,
      1,
      "Search still shows unrelated files; not capturing",
    );
  device.screenshot("dev/screens/shots/android-photo-location-picker.png");
  enter(filename);
  console.log("Selected only the synthetic JPEG");
} else if (phase === "allow" || phase === "deny") {
  const nodes = device.hierarchy();
  const button = nodes.find(
    (n) =>
      /permissioncontroller/.test(n.package) &&
      new RegExp(
        phase === "allow"
          ? "permission_allow_button$"
          : "permission_deny_button$",
      ).test(n["resource-id"]),
  );
  assert(button, "Expected photo-location permission question absent");
  device.screenshot(`dev/screens/shots/android-photo-location-${phase}.png`);
  device.tapNode(button);
} else if (phase === "result") {
  const cdp = await device.connect();
  try {
    await waitFor(async () => {
      const s = await cdp.evaluate("window.__wyndGeoProbe?.state");
      assert(s, "Probe page recreated");
      return s !== "waiting";
    }, "Photo File API did not finish");
    const result = await cdp.evaluate("window.__wyndGeoProbe");
    assert.equal(result.state, "selected", JSON.stringify(result));
    assert.equal(result.name, filename);
    if (label.startsWith("granted")) {
      assert.equal(
        result.sha256,
        createHash("sha256").update(fixture).digest("hex"),
      );
      assert(Math.abs(result.hints.geo_lat - 59.9387) < 0.001);
      assert(Math.abs(result.hints.geo_lng - 30.309) < 0.001);
      assert.equal(result.hints.entry_date, "2026-08-14");
      const pid = device.adb("shell", "pidof", "-s", device.packageName);
      const log = device.adb(
        "logcat",
        "-d",
        "--pid",
        pid,
        "-s",
        "WyndAndroid:I",
      );
      assert(
        log.includes("Selected original photo source prepared"),
        "MediaStore original path was not used",
      );
    }
    if (label === "denied") {
      assert.equal(result.hints.geo_lat, undefined);
      assert.equal(result.hints.geo_lng, undefined);
      assert.equal(result.hints.entry_date, "2026-08-14");
    }
    const output = path.join(
      process.env.WYND_ANDROID_ENV ?? "D:/android-env",
      "test-photo-location",
    );
    mkdirSync(output, { recursive: true });
    writeFileSync(
      path.join(output, label + ".json"),
      JSON.stringify(result, null, 2),
    );
    console.log(JSON.stringify(result, null, 2));
  } finally {
    cdp.close();
  }
} else throw Error("Unknown phase");
