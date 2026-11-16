// Свои dev-адреса, синтетические маркеры и настоящее окно Android WebView.
// Не выполняет вход, публикацию или очистку данных приложения.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { AndroidDevice, waitFor, sleep } from "./android-device.mjs";
import QRCode from "../web/node_modules/qrcode/lib/index.js";

const [serial, mode = "test"] = process.argv.slice(2);
const origin = "http://127.0.0.1:7696";
const nextOrigin = "http://127.0.0.1:7698";
const device = new AndroidDevice(serial, origin);
const out = "dev/screens/shots/android-interface-2026-11-16";
mkdirSync(out, { recursive: true });
device.start();
let cdp = await device.connect();
const checks = [];
const mark = (label) => {
  checks.push(label);
  console.log(label);
};
async function rendered(text) {
  await waitFor(
    () =>
      cdp.evaluate(
        `document.body.textContent.includes(${JSON.stringify(text)}) && document.readyState==='complete'`,
      ),
    `Missing screen: ${text}`,
  );
}
async function shot(name) {
  await cdp.evaluate("document.activeElement?.blur()");
  await sleep(400);
  device.screenshot(`${out}/${name}.png`);
}
async function click(text) {
  const target = await cdp.evaluate(
    `(() => { const button = [...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)} || b.querySelector('.g')?.firstElementChild?.textContent.trim()===${JSON.stringify(text)}); if(!button || button.disabled) throw Error('Unavailable button'); const rect=button.getBoundingClientRect(); return {x:rect.x+rect.width/2,y:rect.y+rect.height/2,width:innerWidth,height:innerHeight}; })()`,
  );
  // Настоящий жест важен: Chromium может пропускать pushState без user activation.
  const surface = device
    .hierarchy()
    .find(
      (n) =>
        n.class === "android.webkit.WebView" &&
        n.package === device.packageName,
    );
  assert(surface, "Own WebView surface is unavailable");
  const bounds = /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/
    .exec(surface.bounds)
    .slice(1)
    .map(Number);
  const [left, top, right, bottom] = bounds,
    scale = (right - left) / target.width;
  // OPPO accessibility обрезает нижнюю границу WebView; масштаб берём по ширине.
  assert(
    top + target.y * scale < bottom,
    "Button is below the observed WebView surface",
  );
  assert(
    target.x > 0 && target.y > 0 && target.y < target.height,
    "Button is outside the visible viewport",
  );
  device.adb(
    "shell",
    "input",
    "tap",
    String(Math.round(left + target.x * scale)),
    String(Math.round(top + target.y * scale)),
  );
}
async function input(index, value) {
  await cdp.evaluate(
    `(() => { const field=document.querySelectorAll('input:not([type=file])')[${index}]; if(!field) throw Error('Missing field'); field.value=${JSON.stringify(value)}; field.dispatchEvent(new Event('input',{bubbles:true})); })()`,
  );
}
async function rpc(method, values = {}) {
  return cdp.evaluate(
    `new Promise((resolve,reject)=>{
    const port=window.__wyndInterface, previous=port?.onmessage, id=1900000000;
    if(!port){reject(Error('No interface capability'));return;}
    const timer=setTimeout(()=>{port.onmessage=previous;reject(Error('interface timeout'))},24000);
    port.onmessage=({data})=>{const value=JSON.parse(data);if(value.id===id){clearTimeout(timer);port.onmessage=previous;resolve(value)}else previous?.({data})};
    port.postMessage(${JSON.stringify(JSON.stringify({ id: 1900000000, method, ...values }))});
  })`,
    true,
  );
}
async function incoming(op, token) {
  return cdp.evaluate(
    `new Promise((resolve,reject)=>{
    const port=window.__wyndIncomingShare, previous=port?.onmessage, id=1800000000;
    if(!port){reject(Error('No Share capability'));return;}
    const timer=setTimeout(()=>{port.onmessage=previous;reject(Error('Share timeout'))},6000);
    port.onmessage=({data})=>{const value=JSON.parse(data);if(value.id===id){clearTimeout(timer);port.onmessage=previous;resolve(value)}else previous?.({data})};
    port.postMessage(${JSON.stringify(JSON.stringify({ id: 1800000000, op, token }))});
  })`,
    true,
  );
}
async function connectTo(address) {
  cdp.close();
  cdp = await new AndroidDevice(serial, address).connect();
  await rendered("Войти");
}
async function source(address, invitation = "") {
  await cdp.navigate(`${device.origin}/settings/interface`);
  await rendered("Где ваш сервер?");
  await input(invitation ? 1 : 0, invitation || address);
  await waitFor(
    () =>
      cdp.evaluate(
        "!!document.querySelector('.server-card') && ![...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='Дальше').disabled",
      ),
    "Server card not checked",
  );
}

try {
  await cdp.navigate(`${origin}/settings`);
  await rendered("Настройки");
  if (mode === "before" || mode === "reference") {
    if (mode === "before") await shot("settings-before");
    const baselineCommit = "825921cc100f5cd8b793b5a245fa916d14ba426f";
    let html = execFileSync(
      "git",
      ["show", `${baselineCommit}:docs/visual/screens.html`],
      { encoding: "utf8", maxBuffer: 8000000 },
    );
    html = html
      .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, "")
      .replaceAll("../../web/static/fonts/", "/fonts/");
    const fonts = readFileSync("web/src/routes/+layout.svelte", "utf8")
      .match(/@font-face\s*\{[^}]+\}/g)
      .join("\n");
    html = html.replace("</head>", `<style>${fonts}</style></head>`);
    const url = `${origin}/android-interface-baseline`;
    const listener = ({ data }) => {
      const event = JSON.parse(data);
      if (event.method === "Fetch.requestPaused")
        void cdp.send("Fetch.fulfillRequest", {
          requestId: event.params.requestId,
          responseCode: 200,
          responseHeaders: [
            { name: "Content-Type", value: "text/html; charset=utf-8" },
          ],
          body: Buffer.from(html).toString("base64"),
        });
    };
    cdp.socket.addEventListener("message", listener);
    await cdp.send("Fetch.enable", {
      patterns: [
        { urlPattern: url, resourceType: "Document", requestStage: "Request" },
      ],
    });
    await cdp.navigate(url);
    await cdp.evaluate("document.fonts.ready", true);
    for (const id of ["e7-41", "e7-42", "e7-44", "e7-46", "e7-47"]) {
      await cdp.evaluate(`(() => {
        document.querySelectorAll('figure.s').forEach(f=>f.style.display='none');
        const figure=document.getElementById(${JSON.stringify(id)}), frame=figure.querySelector('.ph');
        figure.style.cssText='display:block;position:fixed;inset:0;width:100vw;height:100vh;margin:0;z-index:9999';
        frame.style.cssText='width:100vw;height:100vh;border:0;border-radius:0;box-shadow:none;margin:0;transform:none';
        frame.querySelector('.sb')?.remove();
      })()`);
      await shot(`baseline-${id}`);
    }
    await cdp.send("Fetch.disable");
    cdp.socket.removeEventListener("message", listener);
    await cdp.navigate(origin);
    mark(
      "Captured approved HTML frames and current settings at the real phone viewport",
    );
  } else if (mode === "screens") {
    await shot("settings-after");
    await click("Сменить сервер");
    await rendered("Ваши круги от него не зависят");
    await shot("confirm");
    await click("Сменить");
    await rendered("Где ваш сервер?");
    await shot("address");
    await input(0, "http://127.0.0.1:1");
    await rendered("Сервер Wynd по этому адресу не отвечает");
    await shot("unavailable");
    await input(0, nextOrigin);
    await waitFor(
      () => cdp.evaluate("!!document.querySelector('.server-card')"),
      "Server card not checked",
    );
    await shot("checked");
    assert.equal((await rpc("status")).result.origin, `${origin}/`);
    await cdp.navigate(origin);
    await rendered("Войти");
    await shot("restored-login");
    mark(
      "Captured final rendered screens without changing the selected source",
    );
  } else {
    assert.equal((await rpc("status")).result.origin, `${origin}/`);
    assert.equal(
      (await incoming("peek")).pending,
      null,
      "Existing incoming Share must be handled by its owner",
    );
    await shot("settings-after");
    await click("Сменить сервер");
    await rendered("Ваши круги от него не зависят");
    await shot("confirm");
    device.adb("shell", "input", "keyevent", "4");
    await waitFor(
      () =>
        cdp.evaluate(
          "location.pathname==='/settings' && !document.querySelector('[role=dialog]')",
        ),
      "Back did not dismiss confirmation in place",
    );
    mark("Native Back cancels confirmation without switching");
    await click("Сменить сервер");
    await rendered("Ваши круги от него не зависят");
    await click("Сменить");
    await rendered("Где ваш сервер?");
    await shot("address");
    await input(0, "http://127.0.0.1:1");
    await rendered("Сервер Wynd по этому адресу не отвечает");
    await shot("unavailable");
    assert.equal((await rpc("status")).result.origin, `${origin}/`);
    mark("Unreachable address leaves the selected origin unchanged");
    const first = await rpc("check", { address: nextOrigin });
    assert(first.result?.ticket);
    const second = await rpc("check", { address: origin });
    assert(second.result?.ticket);
    assert.equal(
      (await rpc("select", { ticket: first.result.ticket })).error,
      "expired",
    );
    assert.equal((await rpc("status")).result.origin, `${origin}/`);
    mark("A stale check ticket cannot switch source");
    device.adb(
      "shell",
      "am",
      "start",
      "-W",
      "-a",
      "android.intent.action.SEND",
      "-t",
      "text/plain",
      "--es",
      "android.intent.extra.TEXT",
      "WYND_ANDROID_INTERFACE_FIXTURE",
      "-n",
      device.component,
    );
    let pending;
    await waitFor(async () => {
      pending = (await incoming("peek")).pending;
      return !!pending;
    }, "Synthetic Share did not arrive");
    assert.equal(pending.text, "WYND_ANDROID_INTERFACE_FIXTURE");
    assert.equal(pending.files.length, 0);
    const busy = await rpc("check", { address: nextOrigin });
    assert.equal(
      (await rpc("select", { ticket: busy.result.ticket })).error,
      "busy",
    );
    assert.equal((await incoming("peek")).pending.token, pending.token);
    await incoming("finish", pending.token);
    assert.equal((await incoming("peek")).pending, null);
    mark("A pending incoming Share blocks a switch without losing its text");
    await cdp.navigate(`${origin}/settings/interface`);
    await rendered("Где ваш сервер?");
    await click("Сканировать QR-код");
    await rendered("Сканер");
    await waitFor(
      () =>
        cdp.evaluate(
          "!!document.querySelector('video')?.srcObject?.getVideoTracks().some(t=>t.readyState==='live') || document.body.textContent.includes('Нет доступа')",
        ),
      "Scanner neither started nor explained the permission",
    );
    await cdp.evaluate(
      "window.__wyndInterfaceTracks = [...(document.querySelector('video')?.srcObject?.getVideoTracks() ?? [])]",
    );
    device.adb("shell", "input", "keyevent", "4");
    await rendered("Где ваш сервер?");
    assert(
      await cdp.evaluate(
        "window.__wyndInterfaceTracks.length > 0 && window.__wyndInterfaceTracks.every(t=>t.readyState==='ended')",
      ),
    );
    await cdp.evaluate("delete window.__wyndInterfaceTracks");
    const png = await QRCode.toBuffer(
      `${nextOrigin}/invite/android-interface-qr`,
      { width: 512, margin: 4 },
    );
    await cdp.evaluate(`(() => {
      const bytes=Uint8Array.from(atob(${JSON.stringify(png.toString("base64"))}),c=>c.charCodeAt(0));
      const transfer=new DataTransfer();transfer.items.add(new File([bytes],'wynd-interface-qr.png',{type:'image/png'}));
      const field=document.querySelector('input[type=file]');field.files=transfer.files;field.dispatchEvent(new Event('change',{bubbles:true}));
    })()`);
    await waitFor(
      () =>
        cdp.evaluate(
          "document.querySelector('.server-card')?.textContent.includes('127.0.0.1:7698')",
        ),
      "QR photo did not choose its server",
    );
    mark(
      "Existing camera scanner opens; a synthetic QR photo checks its own server",
    );
    const marker = "wynd.android.interface-fixture";
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
      null,
      "Fixture key already exists",
    );
    await cdp.evaluate(
      `localStorage.setItem(${JSON.stringify(marker)},'old-origin')`,
    );
    await source(nextOrigin);
    await shot("checked");
    await click("Дальше");
    await connectTo(nextOrigin);
    device.origin = nextOrigin;
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
      null,
    );
    await cdp.evaluate(
      `localStorage.setItem(${JSON.stringify(marker)},'new-origin')`,
    );
    mark(
      "Switch recreates the WebView at the checked origin without copying localStorage",
    );
    // localStorage возвращает управление до фоновой записи Chromium на диск.
    await sleep(6500);
    device.adb("shell", "am", "force-stop", device.packageName);
    cdp.close();
    device.start();
    cdp = await device.connect();
    await rendered("Войти");
    assert.equal((await rpc("status")).result.origin, `${nextOrigin}/`);
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
      "new-origin",
    );
    mark(
      "Cold launcher start restores the selected source and its own storage",
    );
    await source(origin, `${origin}/invite/android-interface-fixture`);
    await click("Дальше");
    cdp.close();
    cdp = await new AndroidDevice(serial, origin).connect();
    device.origin = origin;
    await waitFor(
      () =>
        cdp.evaluate("location.pathname==='/invite/android-interface-fixture'"),
      "Invitation target was not retained",
    );
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
      "old-origin",
    );
    await cdp.evaluate(`localStorage.removeItem(${JSON.stringify(marker)})`);
    mark(
      "Invitation opens on its own server; the original storage is still present",
    );
    // Remove only our marker at the second origin, then restore the original one.
    await source(nextOrigin);
    await click("Дальше");
    await connectTo(nextOrigin);
    device.origin = nextOrigin;
    await cdp.evaluate(`localStorage.removeItem(${JSON.stringify(marker)})`);
    await source(origin);
    await click("Дальше");
    await connectTo(origin);
    device.origin = origin;
    await shot("restored-login");
    mark("Restored the original source and removed both synthetic markers");
  }
  writeFileSync(
    `${out}/${mode}-checks.json`,
    JSON.stringify(
      {
        checks,
        viewport: await cdp.evaluate(
          "({width:innerWidth,height:innerHeight,dpr:devicePixelRatio})",
        ),
        errors: cdp.exceptions,
      },
      null,
      2,
    ),
  );
  assert.deepEqual(cdp.exceptions, []);
} finally {
  cdp.close();
}
