// Проверка настоящего WebView отладочного пакета через его собственный CDP.
// Не очищает данные приложения и не выполняет вход или отправку записей.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

const [
  serial,
  serverUrl = "http://127.0.0.1:7696",
  outputDir = "dev/screens/shots",
] = process.argv.slice(2);
assert(
  serial,
  "Usage: node scripts/test-android-webview.mjs SERIAL [SERVER_URL] [OUTPUT_DIR]",
);
const origin = new URL(serverUrl).origin;
const adbPath = path.join(
  process.env.WYND_ANDROID_ENV ?? "D:/android-env",
  "sdk/platform-tools/adb.exe",
);
const packageName = "mixeme.wynd.debug";
const component = `${packageName}/mixeme.wynd.MainActivity`;
const forwards = new Set();
const connections = new Set();
const inspectOnly = process.argv.includes("--inspect");
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const adb = (...args) =>
  execFileSync(adbPath, ["-s", serial, ...args], {
    encoding: "utf8",
    timeout: 20000,
    windowsHide: true,
  }).trim();

async function waitFor(check, message) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    if (await check()) return;
    await sleep(200);
  }
  throw new Error(message);
}

async function connect() {
  const appPid = adb("shell", "pidof", "-s", packageName);
  assert(/^\d+$/.test(appPid), "App is not running");
  const port = adb(
    "forward",
    "tcp:0",
    `localabstract:webview_devtools_remote_${appPid}`,
  );
  assert(/^\d+$/.test(port), "ADB did not return a local CDP port");
  forwards.add(port);
  let target;
  await waitFor(async () => {
    const targets = await fetch(`http://127.0.0.1:${port}/json/list`).then(
      (r) => r.json(),
    );
    target = targets.find(
      (item) => item.type === "page" && new URL(item.url).origin === origin,
    );
    return Boolean(target);
  }, "Selected server page did not appear in WebView");
  const endpoint = new URL(target.webSocketDebuggerUrl);
  assert(
    ["127.0.0.1", "localhost"].includes(endpoint.hostname) &&
      endpoint.port === port,
  );
  const socket = new WebSocket(endpoint);
  connections.add(socket);
  const pending = new Map();
  const exceptions = [];
  let sequence = 0;
  socket.addEventListener("message", ({ data }) => {
    const event = JSON.parse(data);
    if (event.method === "Runtime.exceptionThrown") {
      const details = event.params.exceptionDetails;
      exceptions.push(details.exception?.description ?? details.text);
    }
    if (!event.id) return;
    const command = pending.get(event.id);
    if (!command) return;
    pending.delete(event.id);
    clearTimeout(command.timer);
    if (event.error) command.reject(new Error(event.error.message));
    else command.resolve(event.result);
  });
  await new Promise((resolve, reject) => {
    socket.addEventListener("open", resolve, { once: true });
    socket.addEventListener("error", reject, { once: true });
  });
  const send = (method, params = {}) =>
    new Promise((resolve, reject) => {
      const id = ++sequence;
      const timer = setTimeout(() => {
        pending.delete(id);
        reject(new Error(`CDP timeout: ${method}`));
      }, 10000);
      pending.set(id, { resolve, reject, timer });
      socket.send(JSON.stringify({ id, method, params }));
    });
  const evaluate = async (expression) => {
    const result = await send("Runtime.evaluate", {
      expression,
      returnByValue: true,
    });
    assert(
      !result.exceptionDetails,
      result.exceptionDetails?.exception?.description ??
        "WebView JavaScript evaluation failed",
    );
    return result.result.value;
  };
  await send("Runtime.enable");
  await send("Page.enable");
  return { send, evaluate, exceptions, socket };
}

try {
  assert.equal(adb("get-state"), "device");
  const server = new URL(serverUrl);
  if (
    server.protocol === "http:" &&
    ["localhost", "127.0.0.1", "[::1]"].includes(server.hostname)
  ) {
    const port = server.port || "80";
    adb("reverse", `tcp:${port}`, `tcp:${port}`);
  }
  adb("shell", "am", "start", "-W", "-n", component);
  let cdp = await connect();
  await cdp.send("Page.reload");
  if (inspectOnly) {
    await sleep(2000);
    console.log(
      JSON.stringify(
        {
          page: await cdp.evaluate(`({url:location.href, title:document.title, ready:document.readyState,
                text:document.body?.innerText.slice(0,1500), userAgent:navigator.userAgent})`),
          exceptions: cdp.exceptions,
        },
        null,
        2,
      ),
    );
  } else {
    await waitFor(
      () =>
        cdp.evaluate(
          `location.pathname === '/' && document.body?.innerText.toLowerCase().includes('почта')`,
        ),
      "Existing login UI did not render",
    );
    const runtime = await cdp.evaluate(`({
        width: innerWidth, height: innerHeight, scale: devicePixelRatio,
        userAgent: navigator.userAgent, secureContext: isSecureContext,
        serviceWorkerAvailable: 'serviceWorker' in navigator,
        desktopBridgeAbsent: typeof window.__wyndDesktop === 'undefined',
        hasEmailInput: !!document.querySelector('input[type=email]'),
        horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
        paper: getComputedStyle(document.querySelector('.ph')).backgroundColor
    })`);
    assert(runtime.hasEmailInput && runtime.desktopBridgeAbsent);
    assert.equal(runtime.horizontalOverflow, false);
    const markerKey = "wynd:android-scaffold-smoke";
    const marker = `smoke-${Date.now()}`;
    await cdp.evaluate(
      `localStorage.setItem(${JSON.stringify(markerKey)}, ${JSON.stringify(marker)})`,
    );

    await cdp.send("Page.navigate", { url: `${origin}/invite?from=login` });
    await waitFor(
      () =>
        cdp.evaluate(
          `location.pathname === '/invite' && document.body?.innerText.toLowerCase().includes('приглашение')`,
        ),
      "Same-origin invitation route did not render",
    );
    adb("shell", "input", "keyevent", "4");
    await waitFor(
      () =>
        cdp.evaluate(
          `location.pathname === '/' && document.body?.innerText.toLowerCase().includes('почта')`,
        ),
      "System Back did not return to login",
    );

    await cdp.evaluate(
      `location.href = 'https://example.invalid/wynd-blocked-navigation'; true`,
    );
    await sleep(500);
    assert.equal(
      await cdp.evaluate("location.origin"),
      origin,
      "Foreign document entered the app WebView",
    );
    assert.deepEqual(
      cdp.exceptions,
      [],
      "Uncaught JavaScript errors while rendering or navigating",
    );
    cdp.socket.close();
    connections.delete(cdp.socket);

    adb("shell", "am", "force-stop", packageName);
    adb("shell", "am", "start", "-W", "-n", component);
    cdp = await connect();
    await waitFor(
      () =>
        cdp.evaluate(
          `document.body?.innerText.toLowerCase().includes('почта')`,
        ),
      "Login did not render after a cold restart",
    );
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(markerKey)})`),
      marker,
      "WebView local data was lost on a cold restart",
    );
    await cdp.evaluate(`localStorage.removeItem(${JSON.stringify(markerKey)})`);
    assert.deepEqual(
      cdp.exceptions,
      [],
      "Uncaught JavaScript errors after a cold restart",
    );
    const activity = adb("shell", "dumpsys", "activity", "activities");
    assert(
      activity
        .split("\n")
        .some(
          (line) => /mResumedActivity/.test(line) && line.includes(component),
        ),
      "Wynd is not the resumed activity; unlock the phone before capturing",
    );

    mkdirSync(outputDir, { recursive: true });
    const screenshot = path.resolve(outputDir, "android-scaffold-login.png");
    const pixels = execFileSync(
      adbPath,
      ["-s", serial, "exec-out", "screencap", "-p"],
      {
        timeout: 20000,
        maxBuffer: 16000000,
        windowsHide: true,
      },
    );
    assert.equal(pixels.subarray(1, 4).toString(), "PNG");
    writeFileSync(screenshot, pixels);
    const result = {
      capturedAt: new Date().toISOString(),
      packageName,
      serverOrigin: origin,
      checks: [
        "login-rendered",
        "same-origin-navigation",
        "system-back",
        "foreign-document-blocked",
        "cold-restart-keeps-local-storage",
        "no-uncaught-js-errors",
      ],
      runtime,
      screenshot,
    };
    console.log(JSON.stringify(result, null, 2));
  }
} finally {
  for (const socket of connections) socket.close();
  for (const port of forwards) adb("forward", "--remove", `tcp:${port}`);
}
