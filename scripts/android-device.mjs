// Инструменты проверок только отладочного Wynd: ADB, его WebView/CDP и реальные кадры.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

export const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
export class AndroidDevice {
  constructor(serial, origin = "http://127.0.0.1:7696") {
    assert(serial, "Specify an ADB serial");
    this.serial = serial;
    this.origin = new URL(origin).origin;
    this.packageName = "mixeme.wynd.debug";
    this.component = `${this.packageName}/mixeme.wynd.MainActivity`;
    this.adbPath = path.join(
      process.env.WYND_ANDROID_ENV ?? "D:/android-env",
      "sdk/platform-tools/adb.exe",
    );
  }
  adb(...args) {
    return execFileSync(this.adbPath, ["-s", this.serial, ...args], {
      encoding: "utf8",
      timeout: 30000,
      windowsHide: true,
    }).trim();
  }
  start() {
    this.adb("shell", "input", "keyevent", "224");
    this.adb("shell", "am", "start", "-W", "-n", this.component);
  }
  async connect() {
    let pid;
    await waitFor(() => {
      try { pid = this.adb("shell", "pidof", "-s", this.packageName); return /^\d+$/.test(pid); }
      catch { return false; }
    }, "Wynd is not running");
    const port = this.adb(
      "forward",
      "tcp:0",
      `localabstract:webview_devtools_remote_${pid}`,
    );
    assert(/^\d+$/.test(port));
    let socket;
    try {
      let target;
      await waitFor(async () => {
        try {
          const rows = await fetch(`http://127.0.0.1:${port}/json/list`).then(
            (r) => r.json(),
          );
          target = rows.find(
            (row) =>
              row.type === "page" &&
              row.url &&
              new URL(row.url).origin === this.origin,
          );
          return Boolean(target);
        } catch {
          return false;
        }
      }, "Wynd WebView did not appear");
      const endpoint = new URL(target.webSocketDebuggerUrl);
      assert(
        ["localhost", "127.0.0.1"].includes(endpoint.hostname) &&
          endpoint.port === port,
      );
      socket = new WebSocket(endpoint);
      await new Promise((resolve, reject) => {
        socket.addEventListener("open", resolve, { once: true });
        socket.addEventListener("error", reject, { once: true });
      });
      const cdp = new AndroidCdp(socket, () =>
        this.adb("forward", "--remove", `tcp:${port}`),
      );
      await cdp.send("Runtime.enable");
      await cdp.send("Page.enable");
      return cdp;
    } catch (error) {
      socket?.close();
      this.adb("forward", "--remove", `tcp:${port}`);
      throw error;
    }
  }
  screenshot(file) {
    this.adb("shell", "input", "keyevent", "224");
    const policy = this.adb("shell", "dumpsys", "window", "policy");
    assert(
      !/^\s+showing=true\s*$/m.test(policy),
      "Unlock the phone before capturing",
    );
    const bytes = execFileSync(
      this.adbPath,
      ["-s", this.serial, "exec-out", "screencap", "-p"],
      { timeout: 20000, maxBuffer: 16000000, windowsHide: true },
    );
    assert.equal(bytes.subarray(1, 4).toString(), "PNG");
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, bytes);
  }
  hierarchy() {
    this.adb("shell", "input", "keyevent", "224");
    const temp = `/data/local/tmp/wynd-ui-${Date.now()}-${Math.random().toString(16).slice(2)}.xml`;
    let xml;
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        this.adb("shell", "uiautomator", "dump", temp);
        xml = this.adb("exec-out", "cat", temp);
        if (xml.includes("<hierarchy")) break;
      } catch (error) {
        if (attempt === 2) throw error;
      } finally {
        this.adb("shell", "rm", "-f", temp);
      }
    }
    assert(xml?.includes("<hierarchy"), "Fresh Android UI hierarchy unavailable");
    return [...xml.matchAll(/<node\s+([^>]+)>/g)].map((match) =>
      Object.assign({ text: "", "content-desc": "" }, Object.fromEntries(
        [...match[1].matchAll(/([\w-]+)="([^"]*)"/g)].map((attr) => [
          attr[1],
          attr[2],
        ]),
      )),
    );
  }
  tapNode(node) {
    assert(node && node.enabled !== "false", "UI node is unavailable");
    const bounds = /^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$/.exec(node.bounds);
    assert(bounds, "UI node has no observed bounds");
    const [, x1, y1, x2, y2] = bounds.map(Number);
    this.adb(
      "shell",
      "input",
      "tap",
      String(Math.round((x1 + x2) / 2)),
      String(Math.round((y1 + y2) / 2)),
    );
  }
}

export async function waitFor(check, message, timeout = 20000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await check()) return;
    await sleep(200);
  }
  throw new Error(message);
}

class AndroidCdp {
  constructor(socket, cleanup) {
    this.socket = socket;
    this.cleanup = cleanup;
    this.sequence = 0;
    this.requests = new Map();
    this.exceptions = [];
    socket.addEventListener("message", ({ data }) => {
      const message = JSON.parse(data);
      if (message.method === "Runtime.exceptionThrown")
        this.exceptions.push(
          message.params.exceptionDetails.exception?.description ??
            message.params.exceptionDetails.text,
        );
      const request = this.requests.get(message.id);
      if (!request) return;
      this.requests.delete(message.id);
      clearTimeout(request.timer);
      if (message.error) request.reject(new Error(message.error.message));
      else request.resolve(message.result);
    });
  }
  send(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++this.sequence;
      const timer = setTimeout(() => {
        this.requests.delete(id);
        reject(new Error(`CDP timeout: ${method}`));
      }, 180000);
      this.requests.set(id, { resolve, reject, timer });
      this.socket.send(JSON.stringify({ id, method, params }));
    });
  }
  async evaluate(expression, awaitPromise = false) {
    const result = await this.send("Runtime.evaluate", {
      expression,
      awaitPromise,
      returnByValue: true,
    });
    assert(
      !result.exceptionDetails,
      result.exceptionDetails?.exception?.description ?? "JS evaluation failed",
    );
    return result.result.value;
  }
  async navigate(url) {
    const target = new URL(url).href;
    const navigation = await this.send("Page.navigate", { url: target });
    assert(!navigation.errorText, navigation.errorText);
    await waitFor(async () => {
      try {
        const tree = await this.send("Page.getFrameTree");
        if (navigation.loaderId && tree.frameTree.frame.loaderId !== navigation.loaderId) return false;
        if (tree.frameTree.frame.url !== target) return false;
        return await this.evaluate(
          `location.href === ${JSON.stringify(target)} && document.readyState === 'complete' && !!document.body?.textContent?.trim()`,
        );
      } catch {
        return false;
      }
    }, "Page did not render");
  }
  async native(method, value) {
    await waitFor(
      () => this.evaluate(`document.readyState === 'complete' && typeof window.__wyndAndroid?.onmessage === 'function'`),
      'Web client did not attach its Android message handler',
    );
    const request = {
      id: 1000000 + ++this.sequence,
      method,
      ...(value === undefined ? {} : { value }),
    };
    return this.evaluate(
      `new Promise(resolve => {
      const port = window.__wyndAndroid;
      const previous = port.onmessage;
      port.onmessage = event => {
        const reply = JSON.parse(event.data);
        if (reply.id === ${request.id}) { port.onmessage = previous; resolve(reply.result); }
        else previous?.(event);
      };
      port.postMessage(${JSON.stringify(JSON.stringify(request))});
    })`,
      true,
    );
  }
  close() {
    for (const request of this.requests.values()) {
      clearTimeout(request.timer);
      request.reject(new Error("CDP closed"));
    }
    this.requests.clear();
    this.socket.close();
    this.cleanup();
  }
}
