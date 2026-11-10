// Кадры окна «Wynd Подпись» без GitHub и ключа: vite отдаёт эту страницу
// вместо собранного окна, а window.go.main.App отвечает тем же, что отдаёт
// cmd/wynd-signer (данные повторяют выпуски mixeme/wynd на 2026-11-10).
//
// mock.html?key=missing   — экран «ключа нет» (12а);
// mock.html?products=workshop — список без выпусков окна (вид до правки).

const params = new URLSearchParams(location.search);
const keyExists = params.get('key') !== 'missing';
const includeDesktop = params.get('products') !== 'workshop';
// ?demo=draft — Мастерская 0.4.1 показана черновиком: так виден экран подписи
// в исходной версии, где у выпуска ещё нет подписи.
const demoDraft = params.get('demo') === 'draft';

const state = {
	key_path: 'C:\\Users\\Mix\\.wynd\\workshop-signing.key',
	key_exists: keyExists,
	pub_path: 'D:\\Local\\Git\\wynd\\cmd\\wynd-installer\\signing-key.pub'
};

function release(product, tag, version, draft, publishedAt, signed, url) {
	return { product, tag, version, draft, published_at: publishedAt, signed, url };
}

const releases = [
	...(includeDesktop
		? [release('desktop', 'desktop-v0.1.1', '0.1.1', true, undefined, false, 'https://github.com/mixeme/wynd/releases/tag/desktop-v0.1.1')]
		: []),
	release('workshop', 'workshop-v0.4.1', '0.4.1', demoDraft, demoDraft ? undefined : '2026-11-06T20:16:11Z', !demoDraft, 'https://github.com/mixeme/wynd/releases/tag/workshop-v0.4.1'),
	release('workshop', 'workshop-v0.3.1', '0.3.1', false, '2026-11-03T06:17:53Z', true, 'https://github.com/mixeme/wynd/releases/tag/workshop-v0.3.1')
];

function done(rows) {
	return rows.map(([id, title, note]) => ({ id, title, status: 'done', note }));
}

const checks = {
	'desktop-v0.1.1': {
		check_id: 'mock-check-desktop',
		product: 'desktop',
		tag: 'desktop-v0.1.1',
		draft: true,
		size: 47185920,
		sum_ok: true,
		commit: 'c72af8c1d5e7',
		attest_ok: true,
		key_in_release: false,
		steps: done([
			['release', 'Получаем сведения о выпуске'],
			['download', 'Скачиваем программу и контрольную сумму'],
			['checksum', 'Проверяем контрольную сумму'],
			['attestation', 'Проверяем происхождение сборки', 'коммит c72af8c1']
		])
	},
	'workshop-v0.4.1': {
		check_id: 'mock-check-workshop',
		product: 'workshop',
		tag: 'workshop-v0.4.1',
		draft: demoDraft,
		size: 91250688,
		sum_ok: true,
		commit: '5f9203b5a1b2',
		attest_ok: true,
		key_in_release: true,
		notes: '## 0.4.1\n\n- Список серверов и пульт\n- Понятное сообщение у чужого файла',
		steps: done([
			['release', 'Получаем сведения о выпуске'],
			['download', 'Скачиваем программу и контрольную сумму'],
			['checksum', 'Проверяем контрольную сумму'],
			['attestation', 'Проверяем происхождение сборки', 'коммит 5f9203b5'],
			['key', 'Проверяем открытый ключ'],
			['notes', 'Читаем описание выпуска']
		])
	}
};

const app = {
	State: async () => state,
	Releases: async () => ({ ok: true, releases }),
	CheckRelease: async (tag, requestId) => ({ ...checks[tag], request_id: requestId }),
	SignRelease: async (checkId, tag, requestId, pass) => ({
		ok: true,
		published: demoDraft && tag.startsWith('workshop-'),
		url: `https://github.com/mixeme/wynd/releases/tag/${tag}`,
		steps: done([
			['release', 'Проверяем состояние выпуска'],
			['signature', 'Подписываем и проверяем подпись'],
			['upload', 'Загружаем подпись на GitHub']
		])
	}),
	PickExe: async () => 'C:\\Users\\Mix\\Downloads\\wynd-desktop.exe',
	FileSize: async () => 47185920,
	SignFile: async (path) => ({ ok: true, path: path + '.sig' }),
	ChooseKeyPlace: async (current) => current,
	ChooseKey: async () => state,
	CreateKey: async () => ({ ok: false, message: 'Кадр: создание ключа не выполняется' }),
	OpenLink: async () => {},
	ShowInFolder: async () => {}
};

window.go = { main: { App: app } };
window.runtime = {
	EventsOn: () => () => {},
	WindowMinimise() {},
	WindowToggleMaximise() {},
	WindowIsMaximised: async () => false,
	Quit() {}
};

// main.ts подключается динамически: статический импорт выполнился бы раньше
// наших window.go и окно не увидело бы ответов.
void import('../src/main.ts');

// Кадры открытых состояний доводятся теми же нажатиями, что у владельца:
// ?open=<tag> — открыть выпуск, ?pass=<пароль> — ввести пароль и подписать,
// ?file=1 — подписать файл на этом компьютере.
const tick = (ms) => new Promise((r) => setTimeout(r, ms));
const button = (text) => [...document.querySelectorAll('main button')].find((b) => b.textContent.trim().startsWith(text));
function type(input, value) {
	Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, value);
	input.dispatchEvent(new Event('input', { bubbles: true }));
}
async function drive() {
	const pass = params.get('pass');
	if (params.get('open')) {
		await tick(300);
		const link = button('подписать');
		if (!link) throw new Error('кадр: нет строки выпуска для подписи');
		link.click();
		await tick(800);
		if (pass) {
			const input = document.querySelector('main input[type="password"]');
			type(input, pass);
			await tick(200);
			button('Подписать').click();
		}
	}
	if (params.get('file') === '1') {
		await tick(300);
		button('выбрать').click();
		await tick(500);
		type(document.querySelector('main input[type="password"]'), pass ?? 'frame-password');
		await tick(200);
		button('Подписать').click();
	}
}
void drive();
