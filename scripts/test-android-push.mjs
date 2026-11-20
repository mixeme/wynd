// Real Android KeyStore and UnifiedPush connector; Wynd's server writes only to a local fixture.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { AndroidDevice } from "./android-device.mjs";

const root = new URL("../", import.meta.url);
const device = new AndroidDevice(process.argv[2]);
const apk = fileURLToPath(
  new URL(
    "android/push-fixture/build/outputs/apk/debug/push-fixture-debug.apk",
    root,
  ),
);
const fixture = "mixeme.wynd.pushfixture";
const runner = `${fixture}/.PushProbe`;

function probe(...args) {
  const result = device.adb("shell", "am", "instrument", "-w", ...args, runner);
  assert.match(result, /INSTRUMENTATION_CODE: -1\b/, result);
  return result;
}

assert(
  !device
    .adb("shell", "pm", "list", "packages", fixture)
    .split(/\r?\n/)
    .includes(`package:${fixture}`),
  "Remove an earlier probe yourself if you no longer need its data",
);
assert.equal(device.adb("install", apk).split(/\r?\n/).at(-1), "Success");
try {
  const keys = probe("-e", "step", "keys").match(
    /^INSTRUMENTATION_RESULT: keys=(\{[^\r\n]+\})/m,
  )?.[1];
  assert(keys, "The connector did not export fixture subscription keys");
  const go = execFileSync(
    "go",
    [
      "test",
      "./internal/push",
      "-run",
      "^TestAndroidConnectorWebPushFixture$",
      "-count=1",
      "-v",
    ],
    {
      cwd: fileURLToPath(root),
      env: { ...process.env, WYND_ANDROID_PUSH_PROBE_KEYS: keys },
      timeout: 180_000,
      encoding: "utf8",
      windowsHide: true,
    },
  );
  const sealedJson = go.match(
    /WYND_ANDROID_PUSH_PROBE_SEALED=(\{[^\r\n]+\})/,
  )?.[1];
  assert(sealedJson, "Wynd server did not capture two encrypted signals");
  const sealed = JSON.parse(sealedJson);
  const result = probe(
    "-e",
    "step",
    "verify",
    "-e",
    "first",
    sealed.first,
    "-e",
    "second",
    sealed.second,
  );
  assert.match(
    result,
    /verified=two_server_signals_wrong_key_and_tampering_rejected/,
  );
  console.log(
    "UnifiedPush: two Wynd server signals decrypted with their own registrations; foreign and modified signals rejected.",
  );
} finally {
  assert.equal(device.adb("uninstall", fixture), "Success");
}
