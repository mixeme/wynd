// Предварительная приёмка плана 50. Нужны vite dev в web (5173) и
// cmd/wynd-desktop/frontend (5179). Рендерит настоящие Svelte-исходники;
// демонстрационный Wails-мост существует только в отдельном headless-профиле.
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  mkdirSync,
  readFileSync,
  writeFileSync,
  existsSync,
  rmSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const output = path.join(
  root,
  "dev/screens/shots/android-interface-preview-2026-11-16",
);
const profile = path.join(
  root,
  "tmp",
  `android-interface-render-${process.pid}`,
);
mkdirSync(output, { recursive: true });
mkdirSync(profile, { recursive: true });
const edge = spawn(
  process.env.WYND_EDGE ??
    "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
  [
    "--headless=new",
    "--disable-gpu",
    "--remote-debugging-port=0",
    `--user-data-dir=${profile}`,
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-sync",
    "--hide-scrollbars",
    "about:blank",
  ],
  { windowsHide: true, stdio: "ignore" },
);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function until(check, label) {
  for (let attempt = 0; attempt < 120; attempt++) {
    if (await check()) return;
    await sleep(250);
  }
  throw new Error(`Timeout: ${label}`);
}
let socket;
let browserSocket;
const errors = [];
const captures = [];
try {
  const portFile = path.join(profile, "DevToolsActivePort");
  await until(() => existsSync(portFile), "headless Edge");
  const [port, browserPath] = readFileSync(portFile, "utf8")
    .trim()
    .split(/\r?\n/);
  const targets = await fetch(`http://127.0.0.1:${port}/json/list`).then((r) =>
    r.json(),
  );
  socket = new WebSocket(
    targets.find((t) => t.type === "page").webSocketDebuggerUrl,
  );
  await new Promise((resolve, reject) => {
    socket.addEventListener("open", resolve, { once: true });
    socket.addEventListener("error", reject, { once: true });
  });
  let sequence = 0;
  const requests = new Map();
  socket.addEventListener("message", ({ data }) => {
    const reply = JSON.parse(data);
    if (reply.method === "Runtime.exceptionThrown")
      errors.push(reply.params.exceptionDetails);
    const pending = requests.get(reply.id);
    if (!pending) return;
    requests.delete(reply.id);
    clearTimeout(pending.timer);
    if (reply.error) pending.reject(new Error(reply.error.message));
    else pending.resolve(reply.result);
  });
  function send(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++sequence;
      const timer = setTimeout(() => {
        requests.delete(id);
        reject(new Error(method));
      }, 30000);
      requests.set(id, { resolve, reject, timer });
      socket.send(JSON.stringify({ id, method, params }));
    });
  }
  async function evaluate(expression) {
    const result = await send("Runtime.evaluate", {
      expression,
      returnByValue: true,
      awaitPromise: true,
    });
    assert(
      !result.exceptionDetails,
      result.exceptionDetails?.exception?.description,
    );
    return result.result.value;
  }
  async function waitElement(selector) {
    await until(
      () => evaluate(`!!document.querySelector(${JSON.stringify(selector)})`),
      selector,
    );
  }
  async function click(label) {
    await evaluate(`(() => {
      const button = [...document.querySelectorAll('button')].find((el) =>
        el.getAttribute('aria-label') === ${JSON.stringify(label)} || el.textContent.trim() === ${JSON.stringify(label)});
      if (!button || button.disabled) throw new Error('Unavailable button: ' + ${JSON.stringify(label)});
      button.click();
    })()`);
    await sleep(150);
  }
  async function capture(name, width, height) {
    await evaluate("document.fonts.ready");
    await sleep(200);
    const geometry = await evaluate(`({
      width: innerWidth, height: innerHeight, dpr: devicePixelRatio,
      overflow: document.documentElement.scrollWidth > innerWidth,
      elements: [...document.querySelectorAll('.bar, .hint, .lab, .row2, .btn, .dlg, .ib, input')].map(el => {
        const r = el.getBoundingClientRect();
        return { text: el.getAttribute('aria-label') || el.textContent.trim(),
          x: r.x, y: r.y, width: r.width, height: r.height, font: getComputedStyle(el).fontSize };
      })
    })`);
    assert.equal(geometry.width, width);
    assert.equal(geometry.height, height);
    assert.equal(geometry.overflow, false, name);
    const shot = await send("Page.captureScreenshot", {
      format: "png",
      captureBeyondViewport: false,
    });
    const bytes = Buffer.from(shot.data, "base64");
    writeFileSync(path.join(output, `${name}.png`), bytes);
    captures.push({
      name,
      ...geometry,
      sha256: createHash("sha256").update(bytes).digest("hex"),
    });
    console.log(`${name}: ${width}x${height}`);
  }
  await send("Page.enable");
  await send("Runtime.enable");
  await send("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-color-scheme", value: "light" }],
  });
  await send("Page.addScriptToEvaluateOnNewDocument", {
    source: `
    if (location.port === '5179') {
      const servers = [
        { name: 'Дом Ани', origin: 'https://home.example.org' },
        { name: 'У Славы', origin: 'https://wynd.u-slavy.ru' }
      ];
      window.__wyndIframePath = 'about:blank';
      window.go = { main: { App: {
        Version: async () => '', CheckSelfUpdate: async () => ({ available: false }),
        CurrentServer: async () => servers[0], CheckCurrent: async () => servers[0],
        ListServers: async () => servers,
        PreviewServer: async () => ({ name: 'Файлы', origin: 'https://files.example.org' })
      } } };
    }
  `,
  });
  for (const [width, height] of [
    [390, 780],
    [360, 640],
  ]) {
    await send("Emulation.setDeviceMetricsOverride", {
      width,
      height,
      deviceScaleFactor: 1,
      mobile: true,
    });
    for (const baseline of [true, false]) {
      const prefix = `${baseline ? "desktop" : "android"}-${width}`;
      await send("Page.navigate", {
        url: baseline
          ? "http://127.0.0.1:5179/__shell/"
          : "http://127.0.0.1:5173/dev/smoke/android-interface",
      });
      if (baseline) {
        await waitElement(".server-host");
        await evaluate(
          `window.postMessage({ type: 'wynd:open-interface' }, location.origin)`,
        );
      }
      await waitElement(".removable-server");
      await capture(`${prefix}-choice`, width, height);
      await click("Убрать сервер «У Славы» из списка");
      await waitElement("[role=dialog]");
      await capture(`${prefix}-confirm`, width, height);
      await click("Оставить");
      assert.equal(
        await evaluate(`!!document.querySelector('[role=dialog]')`),
        false,
      );
      await click("Добавить сервер интерфейса");
      await waitElement("input");
      await send("Page.bringToFront");
      await evaluate(`document.querySelector('input').focus()`);
      await send("Input.insertText", { text: "files.example.org" });
      await sleep(100);
      await send("Input.dispatchKeyEvent", {
        type: "keyDown",
        key: "Tab",
        code: "Tab",
        windowsVirtualKeyCode: 9,
      });
      await send("Input.dispatchKeyEvent", {
        type: "keyUp",
        key: "Tab",
        code: "Tab",
        windowsVirtualKeyCode: 9,
      });
      await waitElement(".server-card");
      await capture(`${prefix}-add`, width, height);
    }
  }
  assert.equal(errors.length, 0, JSON.stringify(errors));
  const files = [
    "web/src/routes/dev/smoke/android-interface/+page.svelte",
    "cmd/wynd-desktop/frontend/src/App.svelte",
    "cmd/wynd-desktop/frontend/src/app.css",
    "web/src/lib/layouts/FormLayout.svelte",
    "web/src/lib/components/data/ServerRow.svelte",
    "web/src/lib/components/overlays/ConfirmDialog.svelte",
    "web/src/lib/styles/ui.css",
    "web/src/routes/+layout.svelte",
    "web/src/lib/styles/tokens.css",
    "web/src/lib/components/chrome/PhoneFrame.svelte",
    "web/src/lib/components/chrome/BackBar.svelte",
    "web/src/lib/components/data/Row.svelte",
    "web/src/lib/components/forms/Button.svelte",
    "web/src/lib/components/forms/Hint.svelte",
    "web/src/lib/components/forms/Input.svelte",
    "web/src/lib/components/forms/Label.svelte",
    "web/src/lib/components/forms/IconButton.svelte",
    "web/src/lib/components/overlays/Dialog.svelte",
    "web/src/lib/components/overlays/Scrim.svelte",
    "scripts/render-android-interface.mjs",
  ];
  writeFileSync(
    path.join(output, "source-sha256.json"),
    JSON.stringify(
      {
        headBeforeRender: execFileSync("git", ["rev-parse", "HEAD"], {
          cwd: root,
          encoding: "utf8",
        }).trim(),
        sources: Object.fromEntries(
          files.map((f) => [
            f,
            createHash("sha256")
              .update(readFileSync(path.join(root, f)))
              .digest("hex"),
          ]),
        ),
        captures,
        errors,
      },
      null,
      2,
    ),
  );
  browserSocket = new WebSocket(`ws://127.0.0.1:${port}${browserPath}`);
  await new Promise((resolve) =>
    browserSocket.addEventListener("open", resolve, { once: true }),
  );
  browserSocket.send(JSON.stringify({ id: 1, method: "Browser.close" }));
  await Promise.race([
    new Promise((resolve) =>
      browserSocket.addEventListener("close", resolve, { once: true }),
    ),
    sleep(3000),
  ]);
} finally {
  socket?.close();
  browserSocket?.close();
  if (edge.exitCode === null) {
    await Promise.race([
      new Promise((resolve) => edge.once("exit", resolve)),
      sleep(1500),
    ]);
    if (edge.exitCode === null) edge.kill();
  }
  // Только собственный временный профиль под <repo>/tmp, никогда пользовательский.
  assert(
    path
      .resolve(profile)
      .startsWith(path.join(path.resolve(root), "tmp") + path.sep),
  );
  await sleep(1000);
  try {
    rmSync(profile, {
      recursive: true,
      force: true,
      maxRetries: 8,
      retryDelay: 300,
    });
  } catch {
    console.warn(`Temporary profile remains: ${profile}`);
  }
}
