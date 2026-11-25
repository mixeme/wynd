// Кадры окна «Wynd Подпись» без GitHub и ключа: vite отдаёт эту страницу
// вместо собранного окна, а window.go.main.App отвечает тем же, что отдаёт
// cmd/wynd-signer (данные повторяют выпуски mixeme/wynd на 2026-11-17:
// черновики 0.4.5 и 0.3.1 ждут подписи, 0.4.4 опубликована).
//
// mock.html?key=missing   — экран «ключа нет» (12а);
// mock.html?products=workshop — список без выпусков окна;
// mock.html?open=android-v0.1.0 — экран Android-выпуска (план 65);
// ?androidkey=missing — keystore Android не выбран; ?ready=0 — releaseReady=false.

const params = new URLSearchParams(location.search);
const keyExists = params.get("key") !== "missing";
const includeDesktop = params.get("products") !== "workshop";
const androidReady = params.get("ready") !== "0";
// signed — в черновике уже лежит подписанный APK (прежний прогон оборвался
// после загрузки): окно сверяет подпись и публикует, пароля нет.
const androidSigned = params.get("signed") === "1";

const state = {
  key_path: "C:\\Users\\Mix\\.wynd\\workshop-signing.key",
  key_exists: keyExists,
  pub_path: "D:\\Local\\Git\\wynd\\cmd\\wynd-workshop\\signing-key.pub",
  key_store_path: "D:\\Local\\Git\\myenv\\wynd\\wynd-android.p12",
  key_store_exists: params.get("androidkey") !== "missing",
};

function release(product, name, exe, tag, version, draft, publishedAt, signed) {
  return {
    product,
    name,
    exe,
    tag,
    version,
    draft,
    published_at: publishedAt,
    signed,
    url: "https://github.com/mixeme/wynd/releases/tag/" + tag,
  };
}

const releases = [
  release(
    "android",
    "Wynd Android",
    "wynd-android.apk",
    "android-v0.1.0",
    "0.1.0",
    true,
    undefined,
    androidSigned,
  ),
  ...(includeDesktop
    ? [
        release(
          "desktop",
          "Wynd Настольный",
          "wynd-desktop.exe",
          "desktop-v0.5.0",
          "0.5.0",
          true,
          undefined,
          false,
        ),
      ]
    : []),
  release(
    "workshop",
    "Wynd Мастерская",
    "wynd-workshop.exe",
    "workshop-v0.4.5",
    "0.4.5",
    true,
    undefined,
    false,
  ),
  release(
    "workshop",
    "Wynd Мастерская",
    "wynd-workshop.exe",
    "workshop-v0.4.4",
    "0.4.4",
    false,
    "2026-11-16T20:23:08Z",
    true,
  ),
  // Опубликованные выпуски в стенде — как в mixeme/wynd: сворачиваются одной
  // строкой «Опубликованные выпуски», чтобы не мешать черновикам.
  release(
    "android",
    "Wynd Android",
    "wynd-android.apk",
    "android-v0.1.3",
    "0.1.3",
    false,
    "2026-11-25T07:33:25Z",
    true,
  ),
  release(
    "desktop",
    "Wynd Настольный",
    "wynd-desktop.exe",
    "desktop-v0.5.1",
    "0.5.1",
    false,
    "2026-11-25T16:25:32Z",
    true,
  ),
];

// Итог шага приходит вместе с действием в строке — как в настоящем окне.
function done(rows) {
  return rows.map(([id, title, note, link]) => ({
    id,
    title,
    status: "done",
    note,
    ...(link ? { link } : {}),
  }));
}

const attestationLink = {
  label: "проверить",
  url: "https://github.com/mixeme/wynd/attestations",
};

// Android-выпуск (план 65): свои этапы проверки, публикация по releaseReady
// (имя флага на экране не пишется — решение владельца 19.11).
const androidBlocked =
  "Публикация выпуска заблокирована. Подпишем и выложим подписанный APK в черновик — опубликовать его можно будет позже.";
const androidDetail = androidReady
  ? "Файл wynd-android.apk и его контрольная сумма выложены в выпуск на GitHub. Подпись APK проверена: сертификат не отладочный. Временные файлы на этом компьютере удалены."
  : "Файл wynd-android.apk и его контрольная сумма выложены в черновик. Выпуск не опубликован: публикация заблокирована. Временные файлы на этом компьютере удалены.";
const androidSignSteps = done([
  ["release", "Проверяем состояние выпуска"],
  ...(androidSigned
    ? [["signature", "Проверяем подпись в выпуске"]]
    : [
        ["signature", "Подписываем и проверяем подпись"],
        ["upload", "Загружаем подписанный APK на GitHub"],
      ]),
  ...(androidReady ? [["publish", "Публикуем выпуск"]] : []),
]);

const checks = {
  "desktop-v0.5.0": {
    check_id: "mock-check-desktop",
    product: "desktop",
    name: "Wynd Настольный",
    exe: "wynd-desktop.exe",
    self_update: true,
    publishes: true,
    tag: "desktop-v0.5.0",
    draft: true,
    size: 13397504,
    sum_ok: true,
    commit: "6b5964e3",
    attest_ok: true,
    key_in_release: true,
    extra: [
      {
        name: "wynd-desktop_0.5.0_amd64.deb",
        size: 5450000,
        sum_ok: true,
        attest_ok: true,
        commit: "6b5964e3",
      },
      {
        name: "Wynd-0.5.0-x86_64.AppImage",
        size: 7010000,
        sum_ok: true,
        attest_ok: true,
        commit: "6b5964e3",
      },
    ],
    notes:
      "## 0.5.0\n\n- Впервые Linux-версия окна\n- Автозапуск и уведомления Linux",
    steps: done([
      ["release", "Получаем сведения о выпуске"],
      ["download", "Скачиваем программу и контрольную сумму"],
      ["checksum", "Проверяем контрольную сумму"],
      ["attestation", "Проверяем происхождение сборки", "коммит 6b5964e3", attestationLink],
      ["key", "Проверяем открытый ключ"],
      ["notes", "Читаем описание выпуска"],
    ]),
  },
  "workshop-v0.4.5": {
    check_id: "mock-check-workshop",
    product: "workshop",
    name: "Wynd Мастерская",
    exe: "wynd-workshop.exe",
    self_update: true,
    publishes: true,
    tag: "workshop-v0.4.5",
    draft: true,
    size: 91250688,
    sum_ok: true,
    commit: "a0c6c556",
    attest_ok: true,
    key_in_release: true,
    notes:
      "## 0.4.5\n\n- Удаление «с данными» сверяет каталог установки\n- Понятные отказы, если каталог или том заняты",
    steps: done([
      ["release", "Получаем сведения о выпуске"],
      ["download", "Скачиваем программу и контрольную сумму"],
      ["checksum", "Проверяем контрольную сумму"],
      ["attestation", "Проверяем происхождение сборки", "коммит a0c6c556", attestationLink],
      ["key", "Проверяем открытый ключ"],
      ["notes", "Читаем описание выпуска"],
    ]),
  },
  "android-v0.1.0": {
    check_id: "mock-check-android",
    product: "android",
    name: "Wynd Android",
    exe: "wynd-android.apk",
    self_update: false,
    publishes: androidReady,
    publish_note: androidReady ? undefined : androidBlocked,
    tag: "android-v0.1.0",
    draft: true,
    signed: androidSigned,
    size: 1998548,
    sum_ok: true,
    commit: "df026c9f",
    attest_ok: true,
    key_in_release: false,
    notes: "## 0.1.0\n\n- Начальная оболочка на системном WebView\n- Установка обновления с проверкой подписи и подтверждением Android",
    steps: done([
      ["release", "Получаем сведения о выпуске"],
      ["download", "Скачиваем APK и контрольную сумму"],
      ["checksum", "Проверяем контрольную сумму"],
      ["attestation", "Проверяем происхождение сборки", "коммит df026c9f", attestationLink],
      ["package", "Проверяем пакет и версию"],
      ["align", "Проверяем выравнивание"],
      ["notes", "Читаем описание выпуска"],
      androidReady
        ? ["readiness", "Проверяем готовность к публикации"]
        : ["readiness", "Проверяем готовность к публикации", "публикация заблокирована"],
    ]),
  },
};

const app = {
  State: async () => state,
  Releases: async () => ({ ok: true, releases }),
  CheckRelease: async (tag, requestId) => ({
    ...checks[tag],
    request_id: requestId,
  }),
  ChooseKeystore: async () => state,
  ChooseKeystorePlace: async (current) => current,
  CreateKeystore: async () => ({
    ok: false,
    message: "Кадр: создание ключа не выполняется",
  }),
  SignRelease: async (checkId, tag, requestId, pass) => {
    if (tag === "android-v0.1.0") {
      return {
        ok: true,
        published: androidReady,
        url: `https://github.com/mixeme/wynd/releases/tag/${tag}`,
        detail: androidDetail,
        steps: androidSignSteps,
      };
    }
    return {
      ok: true,
      published: true,
      url: `https://github.com/mixeme/wynd/releases/tag/${tag}`,
      steps: done([
        ["release", "Проверяем состояние выпуска"],
        ["signature", "Подписываем и проверяем подпись"],
        ["upload", "Загружаем подпись на GitHub"],
        ["publish", "Публикуем выпуск"],
      ]),
    };
  },
  PickExe: async () => "C:\\Users\\Mix\\Downloads\\wynd-desktop.exe",
  FileSize: async () => 47185920,
  SignFile: async (path) => ({ ok: true, path: path + ".sig" }),
  ChooseKeyPlace: async (current) => current,
  ChooseKey: async () => state,
  CreateKey: async () => ({
    ok: false,
    message: "Кадр: создание ключа не выполняется",
  }),
  OpenLink: async () => {},
  ShowInFolder: async () => {},
};

window.go = { main: { App: app } };
window.runtime = {
  EventsOn: () => () => {},
  WindowMinimise() {},
  WindowToggleMaximise() {},
  WindowIsMaximised: async () => false,
  Quit() {},
};

// main.ts подключается динамически: статический импорт выполнился бы раньше
// наших window.go и окно не увидело бы ответов.
void import("../src/main.ts");

// Кадры открытых состояний доводятся теми же нажатиями, что у владельца:
// ?open=<tag> — открыть выпуск, ?pass=<пароль> — ввести пароль и подписать,
// ?file=1 — подписать файл на этом компьютере.
const tick = (ms) => new Promise((r) => setTimeout(r, ms));
const button = (text) =>
  [...document.querySelectorAll("main button")].find((b) =>
    b.textContent.trim().startsWith(text),
  );
function type(input, value) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(
    input,
    value,
  );
  input.dispatchEvent(new Event("input", { bubbles: true }));
}
async function drive() {
  const pass = params.get("pass");
  if (params.get("published") === "1") {
    // Свернутая строка опубликованных выпусков: раскрываем тем же нажатием.
    await tick(1200);
    const link = button("показать");
    if (!link) throw new Error("кадр: нет строки опубликованных выпусков");
    link.click();
    await tick(300);
  }
  if (params.get("keys") === "1") {
    await tick(1200);
    const link = button("настроить");
    if (!link) throw new Error("кадр: нет строки ключей");
    link.click();
    await tick(400);
  }
  if (params.get("androidcreate") === "1") {
    // Экран создания ключа Android: из кадра «ключ не выбран» по «Создать».
    await tick(1200);
    const link = button("подписать");
    if (!link) throw new Error("кадр: нет строки выпуска для подписи");
    link.click();
    await tick(800);
    const create = button("Создать");
    if (!create) throw new Error("кадр: нет кнопки «Создать»");
    create.click();
    await tick(400);
  }
  if (params.get("open")) {
    await tick(1200);
    const tag = params.get("open");
    const label = tag.startsWith("desktop")
      ? "Wynd Настольный"
      : tag.startsWith("android")
        ? "Wynd Android"
        : "Wynd Мастерская";
    const row = [...document.querySelectorAll("main .ins-row")].find((r) =>
      r.textContent.includes(label),
    );
    const action = androidSigned ? "опубликовать" : "подписать";
    const link = row
      ? [...row.querySelectorAll("button")].find((b) =>
          b.textContent.trim().startsWith(action),
        )
      : undefined;
    if (!link) throw new Error("кадр: нет строки выпуска для подписи");
    link.click();
    await tick(800);
    if (pass) {
      const input = document.querySelector('main input[type="password"]');
      type(input, pass);
      await tick(200);
      button("Подписать").click();
    } else if (androidSigned && params.get("press") === "1") {
      button(androidReady ? "Опубликовать" : "Проверить подпись").click();
    }
  }
  if (params.get("scroll") === "1") {
    // Итог подписи длиннее окна: кадр снимаем у конца экрана, где кнопки.
    await tick(pass ? 1400 : 200);
    document.querySelector("main.ins-body")?.scrollTo(0, 999999);
  }
  if (params.get("file") === "1") {
    await tick(1200);
    button("выбрать").click();
    await tick(500);
    type(
      document.querySelector('main input[type="password"]'),
      pass ?? "frame-password",
    );
    await tick(200);
    button("Подписать").click();
  }
}
void drive();
