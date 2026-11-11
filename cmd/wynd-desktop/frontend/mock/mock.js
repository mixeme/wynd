// Кадры окна Wynd Настольный без сервера и GitHub: window.go отвечает тем же,
// что cmd/wynd-desktop и internal/desktop/selfupdate.go.
// ?v=a|b|c — вариант полосы обновления (только для кадров).
// ?apply=1 — нажать «Обновить и перезапустить»: подпись не сходится,
// окно показывает отказ.

const params = new URLSearchParams(location.search);
// ?update=none — кадр без полосы обновления.
// ?screen=app — показать интерфейс сервера из соседнего dev-сервера клиента.
const appMode = params.get('screen') === 'app';
if (appMode) globalThis.__wyndIframePath = 'http://127.0.0.1:5173/';
const server = { origin: 'http://127.0.0.1:5173', name: 'Wynd', version: '', registration_mode: 'open' };

const app = {
	Version: async () => '0.2.0',
	CurrentServer: async () => (appMode ? server : null),
	ListServers: async () => [],
	CheckServer: async () => {
		throw new Error('кадр: сервер не проверяется');
	},
	CheckCurrent: async () => {
		if (appMode) return server;
		throw new Error('кадр: сервер не проверяется');
	},
	PreviewServer: async () => {
		throw new Error('кадр: сервер не проверяется');
	},
	AddInterface: async () => {
		throw new Error('кадр: сервер не проверяется');
	},
	RemoveInterface: async () => {},
	SelectServer: async (origin) => ({ origin, name: 'Wynd', version: '', registration_mode: 'open' }),
	ForgetSelected: async () => {},
	// Ответ как у CheckUpdate при выпуске новее и старше UpdateDelay.
	CheckSelfUpdate: async () =>
		params.get('update') === 'none'
			? { available: false }
			: {
					available: true,
					version: '0.2.0',
					notes: ['Список серверов и пульт в окне', 'Самообновление по подписи'],
					attestation: 'https://github.com/mixeme/wynd/releases/tag/desktop-v0.2.0'
				},
	// Подпись не сошлась: установка отменяется.
	ApplySelfUpdate: async () => (params.get('apply') === '1' ? 'signature' : ''),
	ReleasesPage: async () => {}
};

window.go = { main: { App: app } };
window.runtime = {
	WindowMinimise() {},
	WindowToggleMaximise() {},
	WindowIsMaximised: async () => false,
	Quit() {}
};

// main.ts подключается динамически: статический импорт выполнился бы раньше
// наших window.go и окно не увидело бы ответов.
void import('../src/main.ts');

// Состояние отказа на кадре: то же нажатие, что делает владелец.
const tick = (ms) => new Promise((r) => setTimeout(r, ms));
if (params.get('apply') === '1') {
	void (async () => {
		await tick(900);
		const button = [...document.querySelectorAll('button')].find((b) =>
			b.textContent.trim().startsWith('Обновить и перезапустить')
		);
		if (!button) throw new Error('кадр: нет кнопки обновления');
		button.click();
	})();
}
