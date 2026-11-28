// Кадры переключения Android на вшитый интерфейс (план 50, решение
// владельца 28.11): настоящие страницы /settings/app и /settings/app/interface
// из проверяемого исходника при vite dev; нативные мосты подменяет
// демонстрационный порт в headless-профиле, сеть и хранилище не меняются.
// Размеры, тема и масштаб — как в принятых кадрах 17.11
// (docs/testing/android-interface-implementation-2026-11-17.md).
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const output = path.join(
  root,
  "dev/screens/shots/android-embedded-default-2026-11-28",
);
const profile = path.join(root, "tmp", `android-embedded-render-${process.pid}`);
const port = 5273;
const base = `http://127.0.0.1:${port}`;
const sizes = [
  [390, 780],
  [360, 640],
];
mkdirSync(output, { recursive: true });
mkdirSync(profile, { recursive: true });

const vite = spawn(
  process.execPath,
  [
    "node_modules/vite/bin/vite.js",
    "dev",
    "--port",
    String(port),
    "--strictPort",
  ],
  { cwd: path.join(root, "web"), windowsHide: true, stdio: "pipe" },
);
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
  for (let attempt = 0; attempt < 160; attempt++) {
    if (await check()) return;
    await sleep(250);
  }
  throw new Error(`Timeout: ${label}`);
}
let socket;
const captures = [];
const errors = [];
try {
  await until(async () => {
    try {
      const response = await fetch(`${base}/`);
      return response.ok;
    } catch {
      return false;
    }
  }, "vite dev");
  await until(
    () => existsSync(path.join(profile, "DevToolsActivePort")),
    "headless Edge",
  );
  const [debugPort] = readFileSync(
    path.join(profile, "DevToolsActivePort"),
    "utf8",
  )
    .trim()
    .split(/\r?\n/);
  const targets = await fetch(`http://127.0.0.1:${debugPort}/json/list`).then(
    (r) => r.json(),
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
    assert(!result.exceptionDetails, result.exceptionDetails?.exception?.description);
    return result.result.value;
  }
  await send("Page.enable");
  await send("Runtime.enable");
  await send("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-color-scheme", value: "light" }],
  });
  // Демонстрационные мосты на том же протоколе, что нативные: каталог
  // источников в памяти вкладки (переживает навигацию, как перезапуск
  // оболочки на телефоне), уведомления — выключены, входы общие.
  await send("Page.addScriptToEvaluateOnNewDocument", {
    source: `
    (function () {
      const defaults = {
        selected: 'https://home.example.org',
        source: 'embedded',
        servers: [
          { origin: 'https://home.example.org', name: 'Дом Ани' },
          { origin: 'https://files.example.org', name: 'Файлы' }
        ]
      };
      let state = defaults;
      try {
        const saved = JSON.parse(sessionStorage.getItem('wyndReview') || 'null');
        if (saved && Array.isArray(saved.servers)) state = saved;
      } catch {}
      window.__wyndReview = state;
      const persist = () => {
        try { sessionStorage.setItem('wyndReview', JSON.stringify(state)); } catch {}
      };
      window.__wyndReviewReset = () => {
        try { sessionStorage.removeItem('wyndReview'); } catch {}
        state = { ...defaults, servers: defaults.servers.slice() };
        window.__wyndReview = state;
        persist();
      };
      window.__wyndSecrets = { postMessage() {}, onmessage: null };
      window.__wyndPush = {
        onmessage: null,
        postMessage(raw) {
          const req = JSON.parse(raw);
          queueMicrotask(() => this.onmessage && this.onmessage({
            data: JSON.stringify({ id: req.id, result: { enabled: false, allowed: true, reason: '' } })
          }));
        }
      };
      window.__wyndInterfaces = {
        onmessage: null,
        postMessage(raw) {
          const req = JSON.parse(raw);
          const state = window.__wyndReview;
          const reply = { id: req.id, protocol: 1 };
          if (req.op === 'list') reply.catalog = { ...state };
          else if (req.op === 'preview')
            reply.preview = { origin: 'https://files.example.org', name: 'Файлы' };
          else if (req.op === 'add') {
            if (!state.servers.some(s => s.origin === 'https://files.example.org'))
              state.servers.push({ origin: 'https://files.example.org', name: 'Файлы' });
            persist();
            reply.catalog = { ...state };
          } else if (req.op === 'remove') {
            state.servers = state.servers.filter(s => s.origin !== req.origin);
            persist();
            reply.catalog = { ...state };
          } else if (req.op === 'select') {
            state.selected = req.origin;
            state.source = 'server';
            persist();
            reply.catalog = { ...state };
          } else if (req.op === 'embedded') {
            state.source = 'embedded';
            persist();
            reply.catalog = { ...state };
          } else return;
          queueMicrotask(() => this.onmessage && this.onmessage({
            data: JSON.stringify(reply)
          }));
        }
      };
      persist();
    })();
  `,
  });
  async function navigate(url) {
    const target = new URL(url, base).href;
    const navigation = await send("Page.navigate", { url: target });
    assert(!navigation.errorText, navigation.errorText);
    await until(async () => {
      try {
        return await evaluate(
          `location.href === ${JSON.stringify(target)} && document.readyState === 'complete' && !!document.body?.textContent?.trim()`,
        );
      } catch {
        return false;
      }
    }, target);
  }
  async function waitText(text) {
    // Капитель (Label) меняет регистр в innerText — сравниваем без него.
    await until(
      () =>
        evaluate(
          `(document.body?.innerText ?? '').toLowerCase().includes(${JSON.stringify(text.toLowerCase())})`,
        ),
      text,
    );
  }
  async function capture(name, width, height) {
    await evaluate("document.fonts.ready");
    await evaluate(
      `new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`,
    );
    await sleep(200);
    const geometry = await evaluate(`({
      width: innerWidth, height: innerHeight, dpr: devicePixelRatio,
      overflow: document.documentElement.scrollWidth > innerWidth
    })`);
    assert.equal(geometry.width, width, name);
    assert.equal(geometry.height, height, name);
    assert.equal(geometry.dpr, 1, name);
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
  async function setViewport(width, height) {
    await send("Emulation.setDeviceMetricsOverride", {
      width,
      height,
      deviceScaleFactor: 1,
      mobile: true,
    });
  }
  for (const [width, height] of sizes) {
    await setViewport(width, height);
    // Каждый размер начинается с вшитого источника, как первый запуск.
    await evaluate("window.__wyndReviewReset ? window.__wyndReviewReset() : null");
    await navigate("/settings/app");
    await waitText("Интерфейс");
    await waitText("Встроенный");
    await capture(`settings-app-${width}`, width, height);

    await navigate("/settings/app/interface");
    await waitText("Сейчас открывает интерфейс");
    await capture(`source-list-embedded-${width}`, width, height);

    // Смена источника выбором сервера, как на телефоне: оболочка
    // перезапускается, список открывают заново и читают каталог.
    await evaluate(`(() => {
      const row = [...document.querySelectorAll('button.row2')]
        .find(el => el.innerText.includes('Файлы') && !el.getAttribute('aria-label')?.startsWith('Убрать'));
      if (!row) throw new Error('server row unavailable');
      row.click();
    })()`);
    await until(
      () =>
        evaluate(
          `window.__wyndReview.source === 'server' && window.__wyndReview.selected === 'https://files.example.org'`,
        ),
      "server choice",
    );
    await navigate("/settings/app/interface");
    await waitText("Серверы интерфейса");
    await capture(`source-list-server-${width}`, width, height);
  }
  assert.deepEqual(errors, [], "page exceptions");
  await navigate("/settings/app/interface");
  await waitText("Встроенный");
  const listRows = await evaluate(`(() => {
    const rows = [...document.querySelectorAll('button.row2, div.row2')]
      .map(el => el.innerText.replace(/\\s+/g, ' ').trim());
    return { source: window.__wyndReview.source, rows };
  })()`);
  console.log("final list:", JSON.stringify(listRows));
  writeFileSync(
    path.join(output, "manifest.json"),
    JSON.stringify({ sizes, captures }, null, 2) + "\n",
  );
} finally {
  socket?.close();
  edge.kill();
  vite.kill();
}
console.log(`OK: ${captures.length} captures in ${output}`);
