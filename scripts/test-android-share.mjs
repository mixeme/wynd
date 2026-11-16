// Только debug Wynd и отдельный отправитель собственных синтетических файлов.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, waitFor } from "./android-device.mjs";
const [serial, phase, expected = phase] = process.argv.slice(2);
const device = new AndroidDevice(serial);
const envRoot = process.env.WYND_ANDROID_ENV ?? "D:/android-env";
const assets = path.join(envRoot, "share-fixture-assets");
const output = path.join(envRoot, "test-shares");
mkdirSync(output, { recursive: true });
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");
async function module(cdp) {
  const require = createRequire(
    new URL("../web/package.json", import.meta.url),
  );
  const { buildSync } = require("esbuild");
  const js = buildSync({
    stdin: {
      contents: "export * from './src/lib/android/shares';",
      resolveDir: path.resolve("web"),
      loader: "ts",
    },
    alias: {
      "$lib/journal/mentions": path.resolve("web/src/lib/journal/mentions.ts"),
    },
    bundle: true,
    write: false,
    format: "iife",
    globalName: "__wyndShareProbe",
    target: "es2018",
  }).outputFiles[0].text;
  await cdp.evaluate(js);
}
if (phase === "assets") {
  device.start();
  const cdp = await device.connect();
  try {
    const video = await cdp.evaluate(
      `(async()=>{
      const canvas=document.createElement('canvas');canvas.width=96;canvas.height=96;
      const ctx=canvas.getContext('2d'),stream=canvas.captureStream(5),chunks=[];
      const recorder=new MediaRecorder(stream,{mimeType:'video/webm;codecs=vp8'});
      recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};
      const done=new Promise(resolve=>recorder.onstop=resolve);
      let frame=0;const draw=()=>{ctx.fillStyle=frame++%2?'#884422':'#228888';ctx.fillRect(0,0,96,96)};draw();
      const timer=setInterval(draw,180);recorder.start(100);await new Promise(r=>setTimeout(r,1300));recorder.stop();await done;clearInterval(timer);stream.getTracks().forEach(t=>t.stop());
      const blob=new Blob(chunks,{type:'video/webm'});
      return await new Promise(resolve=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.readAsDataURL(blob)});
    })()`,
      true,
    );
    mkdirSync(assets, { recursive: true });
    copyFileSync(
      "web/src/test/fixtures/exif-gps.jpg",
      path.join(assets, "wynd-share-fixture.jpg"),
    );
    const bytes = Buffer.from(video, "base64");
    assert(bytes.length > 100);
    writeFileSync(path.join(assets, "wynd-share-fixture.webm"), bytes);
    console.log(
      JSON.stringify({ syntheticVideoBytes: bytes.length, sha256: sha(bytes) }),
    );
  } finally {
    cdp.close();
  }
} else if (phase === "send") {
  assert(
    ["text", "photo", "multi", "large-text", "no-grant"].includes(expected),
  );
  device.adb("shell", "input", "keyevent", "224");
  console.log(
    device.adb(
      "shell",
      "am",
      "start",
      "-W",
      "-n",
      "mixeme.wynd.sharefixture/.Sender",
      "--es",
      "phase",
      expected,
    ),
  );
} else if (phase === "probe" || phase === "finish" || phase === "empty") {
  const cdp = await device.connect();
  try {
    await module(cdp);
    if (phase === "empty") {
      assert.equal(
        await cdp.evaluate("__wyndShareProbe.peekIncomingShare()", true),
        null,
      );
      console.log("No pending share");
    } else {
      let share;
      await waitFor(async () => {
        share = await cdp.evaluate(
          "__wyndShareProbe.peekIncomingShare()",
          true,
        );
        return !!share;
      }, "Incoming share did not appear");
      assert(
        share.files.every((file) =>
          ["wynd-share-fixture.jpg", "wynd-share-fixture.webm"].includes(
            file.name,
          ),
        ),
        "Only own fixture may be read or removed",
      );
      assert(
        !share.text ||
          share.text ===
            "Тест «Поделиться» <b>без HTML</b>\nhttps://example.org/fixture",
        "Only own fixture text may be read or removed",
      );
      if (phase === "finish") {
        await cdp.evaluate(
          `__wyndShareProbe.finishIncomingShare(${JSON.stringify(share.token)})`,
          true,
        );
        assert.equal(
          await cdp.evaluate("__wyndShareProbe.peekIncomingShare()", true),
          null,
        );
        console.log("Finished own incoming share");
      } else {
        const result = await cdp.evaluate(
          `(async()=>{
          const share=await __wyndShareProbe.peekIncomingShare(),loaded=await __wyndShareProbe.loadIncomingShare(share.token);
          const files=[];for(const file of loaded.files){const bytes=await file.arrayBuffer();const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),b=>b.toString(16).padStart(2,'0')).join('');files.push({name:file.name,type:file.type,size:file.size,sha256:hash});}
          const missing=await fetch('/.wynd-android-share/ffffffff-ffff-ffff-ffff-ffffffffffff/0',{cache:'no-store'});
          return {token:share.token,text:loaded.text,files,unknownCapabilityStatus:missing.status};
        })()`,
          true,
        );
        assert.equal(result.unknownCapabilityStatus, 404);
        if (expected === "text") {
          assert.equal(result.files.length, 0);
          assert(result.text.includes("<b>без HTML</b>"));
        } else {
          assert.equal(result.files.length, expected === "multi" ? 2 : 1);
          for (const file of result.files)
            assert.equal(
              file.sha256,
              sha(readFileSync(path.join(assets, file.name))),
            );
        }
        writeFileSync(
          path.join(output, expected + ".json"),
          JSON.stringify(result, null, 2),
        );
        console.log(JSON.stringify(result, null, 2));
      }
    }
  } finally {
    cdp.close();
  }
} else throw Error("Unknown phase");
