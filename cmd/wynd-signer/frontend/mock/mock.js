// Кадры окна «Wynd Подпись» без GitHub и ключа: vite отдаёт эту страницу
// вместо собранного окна, а window.go.main.App отвечает тем же, что отдаёт
// cmd/wynd-signer (данные повторяют выпуски mixeme/wynd на 2026-11-17:
// черновики 0.4.5 и 0.3.1 ждут подписи, 0.4.4 опубликована).
//
// mock.html?key=missing   — экран «ключа нет» (12а);
// mock.html?products=workshop — список без выпусков окна.

const params = new URLSearchParams(location.search);
const keyExists = params.get("key") !== "missing";
const includeDesktop = params.get("products") !== "workshop";

const state = {
  key_path: "C:\\Users\\Mix\\.wynd\\workshop-signing.key",
  key_exists: keyExists,
  pub_path: "D:\\Local\\Git\\wynd\\cmd\\wynd-workshop\\signing-key.pub",
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
  ...(includeDesktop
    ? [
        release(
          "desktop",
          "Wynd Настольный",
          "wynd-desktop.exe",
          "desktop-v0.3.1",
          "0.3.1",
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
];

function done(rows) {
  return rows.map(([id, title, note]) => ({ id, title, status: "done", note }));
}

const checks = {
  "desktop-v0.3.1": {
    check_id: "mock-check-desktop",
    product: "desktop",
    name: "Wynd Настольный",
    exe: "wynd-desktop.exe",
    self_update: true,
    tag: "desktop-v0.3.1",
    draft: true,
    size: 13397504,
    sum_ok: true,
    commit: "6b5964e3",
    attest_ok: true,
    key_in_release: true,
    notes:
      "## 0.3.1\n\n- Служебный порт окна отвечает только ему\n- Чужие страницы не читают ответы",
    steps: done([
      ["release", "Получаем сведения о выпуске"],
      ["download", "Скачиваем программу и контрольную сумму"],
      ["checksum", "Проверяем контрольную сумму"],
      ["attestation", "Проверяем происхождение сборки", "коммит 6b5964e3"],
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
      ["attestation", "Проверяем происхождение сборки", "коммит a0c6c556"],
      ["key", "Проверяем открытый ключ"],
      ["notes", "Читаем описание выпуска"],
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
  SignRelease: async (checkId, tag, requestId, pass) => ({
    ok: true,
    published: true,
    url: `https://github.com/mixeme/wynd/releases/tag/${tag}`,
    steps: done([
      ["release", "Проверяем состояние выпуска"],
      ["signature", "Подписываем и проверяем подпись"],
      ["upload", "Загружаем подпись на GitHub"],
      ["publish", "Публикуем выпуск"],
    ]),
  }),
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
  if (params.get("open")) {
    await tick(1200);
    const link = button("подписать");
    if (!link) throw new Error("кадр: нет строки выпуска для подписи");
    link.click();
    await tick(800);
    if (pass) {
      const input = document.querySelector('main input[type="password"]');
      type(input, pass);
      await tick(200);
      button("Подписать").click();
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
