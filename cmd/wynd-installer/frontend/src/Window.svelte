<script lang="ts">
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import App from './App.svelte';
	import { backend } from './api';

	// Своя рамка окна (docs/visual/installer.html, полоса .tb): окно без
	// системной рамки (Frameless в main.go), полосу заголовка рисует TitleBar,
	// кнопки — из runtime Wails.
	interface Runtime {
		WindowMinimise(): void;
		WindowToggleMaximise(): void;
		WindowIsMaximised(): Promise<boolean>;
		Quit(): void;
	}
	// В `wails dev` в обычном браузере runtime нет — кнопки ничего не делают.
	const rt = (window as unknown as { runtime?: Runtime }).runtime;

	let version = $state('');
	let maximised = $state(false);

	void backend()
		.Version()
		.then((v) => (version = v))
		.catch(() => {});

	async function syncMaximised() {
		maximised = (await rt?.WindowIsMaximised().catch(() => false)) ?? false;
	}
	void syncMaximised();
</script>

<svelte:window onresize={() => void syncMaximised()} />

<div class="ins-win" class:maximised>
	<TitleBar
		title={`Wynd Мастерская${version ? ` [${version}]` : ''}`}
		{maximised}
		onminimise={() => rt?.WindowMinimise()}
		ontoggle={() => rt?.WindowToggleMaximise()}
		onclose={() => rt?.Quit()}
	/>
	<App />
</div>
