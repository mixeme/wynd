// Реальные настройки темы и кадры телефона; разметку экрана не подменяет.
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, waitFor, sleep } from "./android-device.mjs";

const [serial, phase = "after"] = process.argv.slice(2);
assert(["before", "after"].includes(phase));
const device = new AndroidDevice(serial);
device.start();
let cdp = await device.connect();
const labels = ["Как в системе", "Светлая", "Тёмная"];
async function controls() {
  await cdp.navigate(device.origin + "/settings/app");
  await waitFor(() => cdp.evaluate(`Array.from(document.querySelectorAll('button')).some(n=>n.textContent.trim()==='Как в системе')`), "Theme controls absent");
}
async function choose(label) {
  await controls();
  await cdp.evaluate(`Array.from(document.querySelectorAll('button')).find(n=>n.textContent.trim()===${JSON.stringify(label)}).click()`);
  await waitFor(() => cdp.evaluate(`Array.from(document.querySelectorAll('button')).some(n=>n.textContent.trim()===${JSON.stringify(label)} && n.getAttribute('aria-pressed')==='true')`), "Theme did not change");
  const key = label === "Светлая" ? "light" : label === "Тёмная" ? "dark" : "system";
  await waitFor(() => cdp.evaluate(`new Promise((resolve,reject)=>{
    const request=indexedDB.open('wynd'); request.onerror=()=>reject(request.error);
    request.onsuccess=()=>{const db=request.result, tx=db.transaction('settings','readonly'), item=tx.objectStore('settings').get('app');
      item.onsuccess=()=>resolve(item.result?.theme===${JSON.stringify(key)}); item.onerror=()=>reject(item.error); tx.oncomplete=()=>db.close();};
  })`, true), "Theme was not persisted before reloading the page");
  await cdp.navigate(device.origin + "/");
  await waitFor(() => cdp.evaluate(`!!document.querySelector('input[type=email]')`), "Login did not render");
}
let original;
try {
  await controls();
  original = await cdp.evaluate(`Array.from(document.querySelectorAll('button')).find(n=>${JSON.stringify(labels)}.includes(n.textContent.trim()) && n.getAttribute('aria-pressed')==='true')?.textContent.trim()`);
  assert(labels.includes(original), "Initial theme choice unknown");
  const result = [];
  for (const [label, key, color] of [["Светлая", "light", "#F4F0E9"], ["Тёмная", "dark", "#211E1C"]]) {
    await choose(label);
    const info = await cdp.evaluate(`({dark:document.body.classList.contains('dark'),meta:document.querySelector('meta[name="theme-color"]').content,width:innerWidth,height:innerHeight,dpr:devicePixelRatio})`);
    assert.equal(info.dark, key === "dark");
    assert.equal(info.meta.toUpperCase(), color);
    // До диагностического сообщения: кадр проверяет автоматическую передачу темы.
    await sleep(250);
    device.screenshot(`dev/screens/shots/android-bars-${phase}-${key}.png`);
    if (phase === "after") {
      const reply = await cdp.evaluate(`new Promise((resolve,reject)=>{
        const port=window.__wyndAppearance; if(!port) return reject(Error('Appearance capability absent'));
        const timer=setTimeout(()=>reject(Error('Appearance reply timeout')),5000);
        port.onmessage=e=>{clearTimeout(timer);port.onmessage=null;resolve(JSON.parse(e.data));};
        port.postMessage(JSON.stringify({status:${JSON.stringify(color)},navigation:${JSON.stringify(color)}}));
      })`, true);
      assert.equal(reply.status.toUpperCase(), color);
      assert.equal(reply.navigation.toUpperCase(), color);
      result.push({ ...info, native: reply });
    } else result.push(info);
  }
  if (phase === "after") {
    cdp.close();
    device.adb("shell", "am", "force-stop", device.packageName);
    device.start();
    cdp = await device.connect();
    await waitFor(() => cdp.evaluate(`!!document.querySelector('input[type=email]') && document.body.classList.contains('dark')`), "Dark theme lost after cold restart");
    await sleep(250);
    device.screenshot("dev/screens/shots/android-bars-after-cold-dark.png");
  }
  const output = path.join(process.env.WYND_ANDROID_ENV ?? "D:/android-env", "test-appearance");
  mkdirSync(output, { recursive: true });
  writeFileSync(path.join(output, `${phase}.json`), JSON.stringify({ original, result }, null, 2));
  console.log(JSON.stringify({ original, result }, null, 2));
} finally {
  try { if (original) await choose(original); }
  finally { cdp.close(); }
}
