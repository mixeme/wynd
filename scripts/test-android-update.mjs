import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, waitFor, sleep } from "./android-device.mjs";

const [serial, phase = "prepare"] = process.argv.slice(2);
const device = new AndroidDevice(serial);
const fixtures = path.join(
  process.env.WYND_ANDROID_ENV ?? "D:/android-env",
  "test-releases",
);
const shots = "dev/screens/shots";
const reportPath = path.join(fixtures, "device-result.json");
const marker = "wynd-android-update-preserved";
const fixture = async (scenario) => {
  const response = await fetch("http://127.0.0.1:7697/__test/scenario", {
    method: "POST",
    body: JSON.stringify({ scenario }),
  });
  assert(response.ok, "Fixture scenario could not be selected");
};
const result =
  phase === "finish"
    ? JSON.parse(readFileSync(reportPath, "utf8"))
    : { checks: [] };
let cdp;
const restart = async () => {
  cdp?.close();
  cdp = undefined;
  device.adb("shell", "am", "force-stop", device.packageName);
  device.start();
  cdp = await device.connect();
  await waitFor(
    () =>
      cdp.evaluate(`typeof window.__wyndAndroid?.postMessage === 'function'`),
    "Native update port is absent",
  );
};
const stored = async (write) =>
  cdp.evaluate(
    `new Promise((resolve,reject) => {
  const request = indexedDB.open('wynd-android-update-test',1);
  request.onupgradeneeded = () => request.result.createObjectStore('marker');
  request.onerror = () => reject(request.error);
  request.onsuccess = () => {
    const database = request.result;
    const transaction = database.transaction('marker', ${JSON.stringify(write ? "readwrite" : "readonly")});
    const item = ${write ? `transaction.objectStore('marker').put(${JSON.stringify(marker)}, 'key')` : "transaction.objectStore('marker').get('key')"};
    item.onsuccess = () => { const value = item.result; transaction.oncomplete = () => { database.close(); resolve(value); }; };
    item.onerror = () => reject(item.error);
  };
})`,
    true,
  );

try {
  device.adb("reverse", "tcp:7696", "tcp:7696");
  device.adb("reverse", "tcp:7697", "tcp:7697");
  if (phase === "prepare") {
    await fixture("young");
    device.adb("install", "-r", "-d", path.join(fixtures, "base-0.1.0.apk"));
    await restart();
    let status = await cdp.native("setImmediately", false);
    assert.equal(status.version, "0.1.0");
    assert.equal(
      status.available,
      false,
      "Young release escaped 72-hour delay",
    );
    result.checks.push("young-release-waits-72-hours");
    for (const scenario of ["draft", "prerelease"]) {
      await fixture(scenario);
      status = await cdp.native("check");
      assert.equal(status.available, false);
    }
    result.checks.push("draft-and-prerelease-ignored");
    await fixture("young");
    await cdp.navigate(`${device.origin}/settings/app`);
    await waitFor(
      () =>
        cdp.evaluate(
          `!!document.querySelector('[role="switch"][aria-label="Скачивать сразу"]')`,
        ),
      "Development switch is absent",
    );
    await cdp.evaluate(
      `document.querySelector('[role="switch"][aria-label="Скачивать сразу"]').click()`,
    );
    await waitFor(
      async () => (await cdp.native("status")).immediately,
      "Development switch was not saved",
    );
    status = await cdp.native("status");
    assert(status.available, "Development switch did not bypass the delay");
    await cdp.evaluate(`document.querySelector('[role="switch"][aria-label="Скачивать сразу"]').scrollIntoView({block:'center'})`);
    await sleep(300);
    device.screenshot(`${shots}/android-update-after-development.png`);
    result.checks.push("development-switch-allows-young-release");
    await cdp.navigate(`${device.origin}/`);
    await waitFor(
      () =>
        cdp.evaluate(`document.body?.innerText.includes('Вышла версия 0.1.1')`),
      "Update banner did not appear",
    );
    device.screenshot(`${shots}/android-update-after-login.png`);
    await cdp.navigate(`${device.origin}/settings`);
    await waitFor(
      () =>
        cdp.evaluate(
          `document.body?.innerText.includes('Установить версию 0.1.1')`,
        ),
      "Update action did not appear",
    );
    device.screenshot(`${shots}/android-update-after-settings.png`);
    await cdp.native("setImmediately", false);
    assert.equal((await cdp.native("check")).available, false);
    result.checks.push("disabling-development-switch-restores-delay");
    await fixture("mature");
    assert((await cdp.native("check")).available);
    result.checks.push("mature-release-available-without-bypass");
    for (const [scenario, error] of [
      ["bad-checksum", "checksum"],
      ["wrong-signature", "signature"],
      ["tampered", "signature"],
    ]) {
      await fixture(scenario);
      await restart();
      assert((await cdp.native("check")).available);
      status = await cdp.native("prepare");
      assert.equal(
        status.error,
        error,
        `Candidate ${scenario} was not rejected`,
      );
      assert(status.blocked && !status.ready && !status.installing);
      const attempted = await cdp.native("install");
      assert(!attempted.installing, "Rejected APK entered installer");
      result.checks.push(`${scenario}-rejected-before-installer`);
      if (scenario === "wrong-signature") {
        await cdp.navigate(`${device.origin}/settings`);
        await waitFor(
          () =>
            cdp.evaluate(
              `document.body?.innerText.includes('Подпись не сошлась')`,
            ),
          "Signature refusal not shown",
        );
        device.screenshot(`${shots}/android-update-signature-refusal.png`);
      }
    }
    await fixture("mature");
    await restart();
    assert((await cdp.native("check")).available);
    status = await cdp.native("prepare");
    assert(
      status.ready && !status.error && !status.blocked,
      "Valid signed APK was not prepared",
    );
    await cdp.evaluate(
      `localStorage.setItem(${JSON.stringify(marker)}, ${JSON.stringify(marker)})`,
    );
    await stored(true);
    result.checks.push("valid-apk-verified-and-prepared");
    result.preparedVersion = status.nextVersion;
    result.preparedAt = new Date().toISOString();
    mkdirSync(fixtures, { recursive: true });
    writeFileSync(reportPath, JSON.stringify(result, null, 2));
    console.log(
      JSON.stringify(
        {
          ...result,
          next: "Run install phase; system permission and confirmation may be required",
        },
        null,
        2,
      ),
    );
  } else if (phase === "install") {
    device.start();
    cdp = await device.connect();
    await cdp.navigate(`${device.origin}/settings`);
    await waitFor(
      () =>
        cdp.evaluate(
          `Array.from(document.querySelectorAll('button')).some(button => button.textContent.includes('Установить версию 0.1.1'))`,
        ),
      "Install action absent",
    );
    await cdp.evaluate(
      `Array.from(document.querySelectorAll('button')).find(button => button.textContent.includes('Установить версию 0.1.1')).click()`,
    );
    await sleep(1000);
    console.log(
      JSON.stringify(
        {
          state: await cdp.native("status"),
          ui: device
            .hierarchy()
            .filter((node) => node.text || node["resource-id"])
            .map((node) => ({
              text: node.text,
              id: node["resource-id"],
              package: node.package,
              checked: node.checked,
              bounds: node.bounds,
            })),
        },
        null,
        2,
      ),
    );
  } else if (phase === "finish") {
    // Не возвращать Wynd поверх дополнительного системного запроса (Play Protect).
    assert(
      device.adb('shell', 'dumpsys', 'package', device.packageName).includes('versionName=0.1.1-debug'),
      'Installation is not finished; complete or inspect the system verifier first',
    );
    device.start();
    cdp = await device.connect();
    await waitFor(
      () =>
        cdp.evaluate(`typeof window.__wyndAndroid?.postMessage === 'function'`),
      "Updated native port absent",
    );
    const status = await cdp.native("status");
    assert.equal(
      status.version,
      "0.1.1",
      "PackageInstaller did not install the candidate",
    );
    assert.equal(
      await cdp.evaluate(`localStorage.getItem(${JSON.stringify(marker)})`),
      marker,
    );
    assert.equal(await stored(false), marker);
    assert.equal(status.immediately, false, "Development preference was lost");
    await cdp.evaluate(
      `localStorage.removeItem(${JSON.stringify(marker)}); indexedDB.deleteDatabase('wynd-android-update-test'); true`,
    );
    assert.equal(
      (await cdp.native("check")).available,
      false,
      "Same version was offered as an update",
    );
    await cdp.navigate(`${device.origin}/settings`);
    await waitFor(
      () => cdp.evaluate(`document.body?.innerText.includes('0.1.1')`),
      "Updated version not shown in settings",
    );
    device.screenshot(`${shots}/android-update-installed.png`);
    result.checks.push(
      "packageinstaller-updated-to-0.1.1",
      "localstorage-and-indexeddb-preserved",
      "development-preference-preserved",
      "same-version-not-offered",
    );
    result.finishedAt = new Date().toISOString();
    writeFileSync(reportPath, JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result, null, 2));
  } else throw new Error("Phase must be prepare, install or finish");
} finally {
  cdp?.close();
}
