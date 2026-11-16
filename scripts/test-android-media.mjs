// Диагностика настоящего WebView. Нет входа/отправки, камера/микрофон только в памяти.
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, waitFor, sleep } from "./android-device.mjs";

const [serial, phase = "capture"] = process.argv.slice(2);
const device = new AndroidDevice(serial);
const fixtureName = "wynd-android-media-test.txt";
const fixtureText = "WYND_ANDROID_MEDIA_TEST\n";
const local = path.join(process.env.WYND_ANDROID_ENV ?? "D:/android-env", "test-media");
device.adb("reverse", "tcp:7696", "tcp:7696");
if (phase === "capture" || phase === "file-open") device.start();
else device.adb("shell", "input", "keyevent", "224");
const cdp = await device.connect();
const evaluateGesture = (expression) => cdp.send("Runtime.evaluate", { expression, userGesture: true, returnByValue: true });

try {
  if (phase === "capture") {
    await cdp.navigate(device.origin + "/");
    const checks = [];
    for (const kind of ["video", "audio", "video-audio"]) {
      const before = device.adb("shell", "dumpsys", "package", device.packageName);
      await evaluateGesture(`window.__wyndMediaProbe = {state:'requesting'};
        navigator.mediaDevices.getUserMedia(${JSON.stringify(kind === "video" ? { video: true, audio: false } : kind === "audio" ? { audio: true } : { video: true, audio: true })})
        .then(async stream => {
          const tracks = stream.getTracks();
          try {
            if (${JSON.stringify(kind)} === 'video') {
              const video = document.createElement('video'); video.muted = true; video.srcObject = stream;
              await video.play(); await new Promise(r => setTimeout(r, 250));
              const width = video.videoWidth, height = video.videoHeight;
              video.pause(); video.srcObject = null;
              window.__wyndMediaProbe = {state:'ok',kind:'video',width,height,tracks:tracks.map(t => t.kind)};
            } else {
              let preview;
              if (${JSON.stringify(kind)} === 'video-audio') {
                preview=document.createElement('video'); preview.muted=true; preview.srcObject=stream;
                await preview.play(); await new Promise(r => setTimeout(r,250));
              }
              const recorder = new MediaRecorder(stream); const chunks=[];
              recorder.ondataavailable = e => { if(e.data.size) chunks.push(e.data); };
              await new Promise((resolve,reject) => {
                recorder.onstop=resolve; recorder.onerror=reject; recorder.start(); setTimeout(() => recorder.stop(),${kind === "audio" ? 600 : 1500});
              });
              if(preview) { preview.pause(); preview.srcObject=null; }
              const blob=new Blob(chunks,{type:recorder.mimeType}); chunks.length=0;
              let decoded;
              if (${JSON.stringify(kind)} === 'audio') {
                const context=new AudioContext();
                try { const audio=await context.decodeAudioData(await blob.arrayBuffer()); decoded={samples:audio.length,channels:audio.numberOfChannels}; }
                finally { await context.close(); }
              } else {
                const video=document.createElement('video'), url=URL.createObjectURL(blob); video.muted=true;
                try {
                  await new Promise((resolve,reject) => {
                    const timer=setTimeout(() => reject(Error('Recorded video did not decode')),5000);
                    video.onloadeddata=() => {clearTimeout(timer);resolve();};
                    video.onerror=() => {clearTimeout(timer);reject(Error('Recorded video failed'));}; video.src=url;
                  });
                  decoded={width:video.videoWidth,height:video.videoHeight};
                } finally { video.removeAttribute('src'); video.load(); URL.revokeObjectURL(url); }
              }
              window.__wyndMediaProbe = {state:'ok',kind:${JSON.stringify(kind)},bytes:blob.size,mime:recorder.mimeType,tracks:tracks.map(t => t.kind),decoded};
            }
          } catch(e) { window.__wyndMediaProbe={state:'failed',name:e.name}; }
          finally { tracks.forEach(t => t.stop()); }
        }).catch(e => window.__wyndMediaProbe={state:'failed',name:e.name});`);
      await sleep(500);
      const nodes = device.hierarchy();
      const prompt = nodes.find(n => /permissioncontroller$/.test(n.package) && n.text.includes("Wynd"));
      if (prompt) {
        assert(kind === "video" ? /снимать/.test(prompt.text) : /аудио|звук/i.test(prompt.text), "Unexpected permission request");
        const allow = nodes.find(n => n.package === prompt.package && n.text === "РАЗРЕШИТЬ");
        assert(allow, "Expected Android allow button");
        device.tapNode(allow);
      }
      await waitFor(async () => (await cdp.evaluate("window.__wyndMediaProbe?.state")) !== "requesting", "Capture request did not finish");
      const result = await cdp.evaluate("window.__wyndMediaProbe");
      assert.equal(result.state, "ok", JSON.stringify(result));
      if (kind === "video") {
        assert(result.width > 0 && result.height > 0);
        assert.deepEqual(result.tracks, ["video"]);
        if (!before.includes("android.permission.RECORD_AUDIO: granted=true")) {
          assert(!device.adb("shell", "dumpsys", "package", device.packageName).includes("android.permission.RECORD_AUDIO: granted=true"), "Camera unexpectedly gained microphone permission");
        }
      } else {
        assert(result.bytes > 0 && result.mime);
        if (kind === "audio") assert(result.decoded.samples > 0 && result.decoded.channels > 0);
        else assert(result.decoded.width > 0 && result.decoded.height > 0);
      }
      checks.push(result);
      await cdp.evaluate("delete window.__wyndMediaProbe; true");
    }
    mkdirSync(local, { recursive: true });
    writeFileSync(path.join(local, "capture-result.json"), JSON.stringify({ checks, at: new Date().toISOString() }, null, 2));
    console.log(JSON.stringify(checks, null, 2));
  } else if (phase === "file-open") {
    mkdirSync(local, { recursive: true });
    writeFileSync(path.join(local, fixtureName), fixtureText);
    device.adb("shell", "mkdir", "-p", "/sdcard/Download/wynd-android-media-test");
    device.adb("push", path.join(local, fixtureName), "/sdcard/Download/wynd-android-media-test/" + fixtureName);
    await cdp.navigate(device.origin + "/invite?from=login");
    // Диагностический input: не часть продукта и не материал приёмки вёрстки.
    await evaluateGesture(`document.getElementById('__wyndMediaInput')?.remove();
      const input=document.createElement('input'); input.type='file'; input.accept='text/plain';
      input.id='__wyndMediaInput'; input.hidden=true; document.body.append(input);
      window.__wyndFileProbe={state:'waiting'};
      input.onchange=async () => {
        const file=input.files[0]; window.__wyndFileProbe=file
          ? {state:'selected',name:file.name,size:file.size,text:await file.text()}
          : {state:'cancelled'};
      }; input.oncancel=() => window.__wyndFileProbe={state:'cancelled'}; input.click();`);
    console.log("Select only the generated fixture in Downloads/wynd-android-media-test, or cancel.");
    console.log(JSON.stringify(device.hierarchy().filter(n => /Загрузки|Недавние|wynd-|Поиск|Показать корни|Открыть/.test(n.text + " " + n["content-desc"])).map(n => ({ text:n.text,description:n["content-desc"],id:n["resource-id"],bounds:n.bounds })), null, 2));
  } else if (phase === "file-select") {
    const enterOwnRow = (name) => {
      for (let attempt = 0; attempt < 8; attempt++) {
        const nodes = device.hierarchy();
        const target = nodes.find(n => n.text === name && n["resource-id"] === "android:id/title");
        assert(target, "Expected own test file/folder in the current picker view");
        const focus = nodes.find(n => n.focused === "true" && n["resource-id"] === "com.google.android.documentsui:id/item_root");
        const box = node => /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/.exec(node.bounds).slice(1).map(Number);
        if (focus) {
          const [x1,y1,x2,y2] = box(focus), [tx1,ty1,tx2,ty2] = box(target);
          if (x1 <= tx1 && y1 <= ty1 && x2 >= tx2 && y2 >= ty2) {
            device.adb("shell", "input", "keyevent", "66"); return;
          }
        }
        device.adb("shell", "input", "keyevent", "20");
      }
      throw Error("Could not focus the generated fixture safely");
    };
    if (!device.hierarchy().some(n => n.text === fixtureName)) enterOwnRow("wynd-android-media-test");
    enterOwnRow(fixtureName);
    console.log("Selected generated fixture through observed keyboard focus.");
  } else if (phase === "file-result" || phase === "file-cancel") {
    await waitFor(async () => {
      const state = await cdp.evaluate("window.__wyndFileProbe?.state");
      assert(state, "Diagnostic page was recreated; the old input no longer exists");
      return state !== "waiting";
    }, "File chooser callback did not finish");
    const result = await cdp.evaluate("window.__wyndFileProbe");
    if (phase === "file-result") {
      assert.equal(result.state, "selected"); assert.equal(result.name, fixtureName); assert.equal(result.text, fixtureText);
    } else assert.equal(result.state, "cancelled");
    console.log(JSON.stringify(result));
    await cdp.evaluate("document.getElementById('__wyndMediaInput')?.remove(); delete window.__wyndFileProbe; true");
    await cdp.navigate(device.origin + "/");
  } else throw Error("Expected capture, file-open, file-select, file-result or file-cancel");
} finally { cdp.close(); }
