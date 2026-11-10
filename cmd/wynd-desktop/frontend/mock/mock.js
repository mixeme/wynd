// Кадры окна Wynd Настольный без сервера и GitHub: window.go отвечает тем же,
// что cmd/wynd-desktop и internal/desktop/selfupdate.go.
// mock.html?apply=1 — нажать «Обновить и перезапустить»: подпись не сходится,
// окно показывает отказ (как кадр 10в у Мастерской).

const params = new URLSearchParams(location.search);

const app = {
	CurrentServer: async () => null,
	ListServers: async () => [],
	CheckServer: async () => {
		throw new Error('кадр: сервер не проверяется');
	},
	CheckCurrent: async () => {
		throw new Error('кадр: сервер не проверяется');
	},
	SelectServer: async (origin) => ({ origin, name: 'Wynd', version: '', registration_mode: 'open' }),
	ForgetSelected: async () => {},
	// Ответ как у CheckUpdate при выпуске новее и старше UpdateDelay.
	CheckSelfUpdate: async () => ({
		available: true,
		version: '0.2.0',
		notes: ['Список серверов и пульт в окне', 'Самообновление по подписи'],
		attestation: 'https://github.com/mixeme/wynd/releases/tag/desktop-v0.2.0'
	}),
	// Подпись не сошлась: установка отменяется (кадр 10в).
	ApplySelfUpdate: async () => 'signature',
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
		await tick(800);
		const button = [...document.querySelectorAll('button')].find(
			(b) => b.textContent.trim() === 'Обновить и перезапустить'
		);
		if (!button) throw new Error('кадр: нет кнопки обновления');
		button.click();
	})();
}
