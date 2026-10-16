<script lang="ts">
	import App from './App.svelte';
	import { backend } from './api';

	// Своя рамка окна (docs/visual/installer.html, полоса .tb): окно без
	// системной рамки (Frameless в main.go), заголовок и кнопки рисуем сами,
	// чтобы на любой Windows окно было как на макете. Полоса таскает окно
	// (--wails-draggable), двойной щелчок и прилипание к краю экрана — от
	// системы; кнопки — из runtime Wails.
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
	<header class="ins-tb">
		<span class="ins-tb-dot" aria-hidden="true">●</span>
		<b>Wynd — установка на свой сервер{version ? ` [${version}]` : ''}</b>
		<span class="ins-tb-btns">
			<button type="button" aria-label="Свернуть" onclick={() => rt?.WindowMinimise()}>—</button>
			<button
				type="button"
				aria-label={maximised ? 'Восстановить' : 'Развернуть'}
				onclick={() => rt?.WindowToggleMaximise()}>{maximised ? '❐' : '▢'}</button
			>
			<button type="button" class="ins-tb-close" aria-label="Закрыть" onclick={() => rt?.Quit()}
				>✕</button
			>
		</span>
	</header>
	<App />
</div>
