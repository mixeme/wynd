// Проверка принятого сценария в настоящем WebView. Только собственные
// loopback-источники и маркеры; исходный источник восстанавливается в finally.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFileSync, existsSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, sleep, waitFor } from "./android-device.mjs";

const [serial, original = "http://127.0.0.1:7696"] = process.argv.slice(2);
const output = path.resolve(
  "dev/screens/shots/android-interface-implementation-2026-11-17",
);
mkdirSync(output, { recursive: true });
const root = path.resolve("web/dist");
const ports = [7698, 7699];
const origins = ports.map((port) => `http://127.0.0.1:${port}`);
const fixtures = [];
const checks = [];
const marker = `wynd-interface-check-${Date.now()}`;
let device = new AndroidDevice(serial, original);
let cdp;
let sequence = 5000000;
let initial;
async function connect(origin) {
  cdp?.close();
  cdp = undefined;
  device = new AndroidDevice(serial, origin);
  cdp = await device.connect();
  await waitFor(
    () =>
      cdp.evaluate(
        `!!window.__wyndInterfaces && document.readyState === 'complete'`,
      ),
    "interface bridge",
  );
}
async function native(op, args = {}) {
  const request = { id: ++sequence, op, ...args };
  return cdp.evaluate(
    `new Promise((resolve,reject) => {
    const port=window.__wyndInterfaces, previous=port.onmessage;
    const timer=setTimeout(()=>{port.onmessage=previous;reject(new Error('native timeout'))},25000);
    port.onmessage=event=>{
      const reply=JSON.parse(event.data);
      if(reply.id===${request.id}){clearTimeout(timer);port.onmessage=previous;resolve(reply)}
      else previous?.(event);
    };
    port.postMessage(${JSON.stringify(JSON.stringify(request))});
  })`,
    true,
  );
}
async function click(text) {
  await cdp.evaluate(`(() => {
    const button=[...document.querySelectorAll('button')].find(b=>b.getAttribute('aria-label')===${JSON.stringify(text)} || b.textContent.trim().startsWith(${JSON.stringify(text)}));
    if(!button || button.disabled)throw new Error('button unavailable: '+${JSON.stringify(text)});
    button.click();
  })()`);
  await sleep(200);
}
async function text(value) {
  await waitFor(
    () =>
      cdp.evaluate(
        `document.body.innerText.toLowerCase().includes(${JSON.stringify(value.toLowerCase())})`,
      ),
    value,
  );
}
async function capture(name) {
  await cdp.evaluate("document.fonts.ready", true);
  await sleep(300);
  assert.equal(
    await cdp.evaluate("document.documentElement.scrollWidth > innerWidth"),
    false,
  );
  device.screenshot(path.join(output, `${name}.png`));
}
try {
  for (const [index, port] of ports.entries()) {
    const server = createServer((request, response) => {
      const url = new URL(request.url, origins[index]);
      if (url.pathname === "/api/v1/instance") {
        response.setHeader("Content-Type", "application/json");
        response.end(
          JSON.stringify({
            name: index ? "Второй источник" : "Первый источник",
            version: "0.53.0",
            registration_mode: "closed",
            bootstrapped: true,
            loopback: true,
          }),
        );
        return;
      }
      if (url.pathname.startsWith("/api/")) {
        response.writeHead(401, { "Content-Type": "application/json" });
        response.end('{"error":"unauthorized"}');
        return;
      }
      const asset = path.resolve(root, "." + decodeURIComponent(url.pathname));
      if (asset !== root && !asset.startsWith(root + path.sep)) {
        response.writeHead(403);
        response.end();
        return;
      }
      const file =
        existsSync(asset) && !url.pathname.endsWith("/")
          ? asset
          : path.join(root, "index.html");
      const types = {
        ".js": "text/javascript",
        ".css": "text/css",
        ".woff2": "font/woff2",
        ".svg": "image/svg+xml",
        ".png": "image/png",
      };
      response.setHeader(
        "Content-Type",
        types[path.extname(file)] ?? "text/html",
      );
      response.end(readFileSync(file));
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(port, "127.0.0.1", resolve);
    });
    fixtures.push(server);
    device.adb("reverse", `tcp:${port}`, `tcp:${port}`);
  }
  device.start();
  await connect(original);
  initial = (await native("list")).catalog;
  assert.equal(initial.selected, original);
  assert(
    !initial.servers.some((s) => origins.includes(s.origin)),
    "Fixture origin already in the user catalog",
  );
  await cdp.evaluate(
    `localStorage.setItem(${JSON.stringify(marker)},'original')`,
  );
  const preview = await native("preview", { address: origins[0] });
  assert.equal(preview.preview.name, "Первый источник");
  assert.deepEqual((await native("list")).catalog, initial);
  checks.push("preview does not add or select");
  await cdp.evaluate(`(() => {
    window.__interfaceFrameReply=false;
    const frame=document.createElement('iframe');frame.id='interface-frame-check';
    frame.srcdoc=${JSON.stringify(`<script>if(window.__wyndInterfaces){__wyndInterfaces.onmessage=()=>parent.__interfaceFrameReply=true;__wyndInterfaces.postMessage(JSON.stringify({id:990001,op:'add',address:${JSON.stringify(origins[0])}}));}</script>`)};
    document.body.append(frame);
  })()`);
  await sleep(500);
  assert.equal(await cdp.evaluate("window.__interfaceFrameReply"), false);
  await cdp.evaluate(
    "document.getElementById('interface-frame-check').remove();delete window.__interfaceFrameReply",
  );
  assert.deepEqual((await native("list")).catalog, initial);
  checks.push("subframe cannot add a native interface source");
  assert.equal(
    (await native("preview", { address: "http://192.0.2.1" })).error,
    "failed",
  );
  assert.equal((await native("remove", { origin: original })).error, "failed");
  assert.equal(
    (await native("select", { origin: origins[1] })).error,
    "failed",
  );
  checks.push("cleartext, selected deletion and unknown selection rejected");
  assert.equal(
    (await native("add", { address: origins[0] })).catalog.selected,
    original,
  );
  await sleep(6500); // localStorage должен успеть записаться перед пересозданием WebView.
  assert.equal(
    (await native("select", { origin: origins[0] })).catalog.selected,
    origins[0],
  );
  await connect(origins[0]);
  assert.equal(
    await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
    null,
  );
  await cdp.evaluate(`localStorage.setItem(${JSON.stringify(marker)},'first')`);
  checks.push("selection creates the new origin without copying localStorage");

  await cdp.navigate(`${origins[0]}/settings/app`);
  await text("Сменить сервер интерфейса");
  await capture("settings-entry");
  await click("Сменить сервер интерфейса");
  await text("Серверы интерфейса");
  await text("Первый источник");
  await capture("list");
  await click("Добавить сервер интерфейса");
  await cdp.evaluate('document.querySelector("input").focus()');
  await cdp.send("Input.insertText", { text: origins[1] });
  await cdp.send("Input.dispatchKeyEvent", {
    type: "keyDown",
    key: "Tab",
    code: "Tab",
    windowsVirtualKeyCode: 9,
  });
  await cdp.send("Input.dispatchKeyEvent", {
    type: "keyUp",
    key: "Tab",
    code: "Tab",
    windowsVirtualKeyCode: 9,
  });
  await text("Сервер доступен");
  await capture("add-preview");
  assert.equal(
    (await native("list")).catalog.servers.length,
    initial.servers.length + 1,
  );
  await click("Добавить сервер интерфейса");
  await waitFor(
    () => cdp.evaluate(`!document.querySelector('input')`),
    "add returns to list",
  );
  assert.equal((await native("list")).catalog.selected, origins[0]);
  checks.push("UI checks address and adds without selection or account");
  await click("Убрать сервер «Второй источник» из списка");
  await text("Убрать сервер «Второй источник» из списка?");
  await capture("remove-confirm");
  device.adb("shell", "input", "keyevent", "4");
  await waitFor(
    () => cdp.evaluate(`!document.querySelector('[role=dialog]')`),
    "system back closes dialog",
  );
  assert.equal(
    (await native("list")).catalog.servers.length,
    initial.servers.length + 2,
  );
  await click("Убрать сервер «Второй источник» из списка");
  await click("Убрать");
  await waitFor(
    async () =>
      (await native("list")).catalog.servers.length ===
      initial.servers.length + 1,
    "confirmed removal",
  );
  checks.push(
    "system Back cancels removal; confirmation removes only the source",
  );

  await native("add", { address: origins[1] });
  await sleep(6500);
  await native("select", { origin: origins[1] });
  await connect(origins[1]);
  await cdp.evaluate(
    `localStorage.setItem(${JSON.stringify(marker)},'second')`,
  );
  await sleep(6500);
  cdp.close();
  cdp = undefined;
  device.adb("shell", "am", "force-stop", device.packageName);
  device.start();
  await connect(origins[1]);
  assert.equal((await native("list")).catalog.selected, origins[1]);
  assert.equal(
    await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
    "second",
  );
  checks.push("selected source and its data survive full process restart");
  await native("select", { origin: origins[0] });
  await connect(origins[0]);
  assert.equal(
    await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
    "first",
  );
  await cdp.evaluate(`localStorage.removeItem(${JSON.stringify(marker)})`);
  checks.push("returning restores original per-origin data");
  writeFileSync(
    path.join(output, "checks.json"),
    JSON.stringify({ checks }, null, 2),
  );
  console.log(JSON.stringify({ checks }, null, 2));
} finally {
  if (initial) {
    // Удаляются только собственные маркеры/два источника этого прогона.
    try {
      if (!cdp) await connect(device.origin);
      for (const origin of origins) {
        const state = (await native("list")).catalog;
        if (state.servers.some((item) => item.origin === origin)) {
          if (state.selected !== origin) {
            assert(!(await native("select", { origin })).error);
            await connect(origin);
          }
          await cdp.evaluate(
            `localStorage.removeItem(${JSON.stringify(marker)})`,
          );
        }
      }
      if ((await native("list")).catalog.selected !== original) {
        await native("select", { origin: original });
        await connect(original);
      }
      await cdp.evaluate(`localStorage.removeItem(${JSON.stringify(marker)})`);
      for (const origin of origins) {
        if (
          (await native("list")).catalog.servers.some(
            (s) => s.origin === origin,
          )
        ) {
          await native("remove", { origin });
        }
      }
      assert.equal((await native("list")).catalog.selected, original);
    } catch (error) {
      console.error("Restore requires attention:", error);
      process.exitCode = 1;
    }
  }
  cdp?.close();
  for (const port of ports) {
    try {
      device.adb("reverse", "--remove", `tcp:${port}`);
    } catch {}
  }
  for (const server of fixtures) {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}
