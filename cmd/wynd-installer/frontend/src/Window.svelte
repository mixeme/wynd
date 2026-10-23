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
		<b>Wynd Мастерская{version ? ` [${version}]` : ''}</b>
		<span class="ins-tb-btns">
			<!-- Значки — свои линии 10 × 10, а не символы шрифта: у «—», «▢», «✕»
			     разная ширина и посадка, в кнопке они стояли левее центра. -->
			<button type="button" aria-label="Свернуть" onclick={() => rt?.WindowMinimise()}>
				<svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 5.5h10" /></svg>
			</button>
			<button
				type="button"
				aria-label={maximised ? 'Восстановить' : 'Развернуть'}
				onclick={() => rt?.WindowToggleMaximise()}
			>
				{#if maximised}
					<svg viewBox="0 0 10 10" aria-hidden="true"
						><path d="M2.5 2.5V.5h7v7h-2" /><rect x=".5" y="2.5" width="7" height="7" /></svg
					>
				{:else}
					<svg viewBox="0 0 10 10" aria-hidden="true"><rect x=".5" y=".5" width="9" height="9" /></svg>
				{/if}
			</button>
			<button type="button" class="ins-tb-close" aria-label="Закрыть" onclick={() => rt?.Quit()}>
				<svg viewBox="0 0 10 10" aria-hidden="true"><path d="M.5.5l9 9M9.5.5l-9 9" /></svg>
			</button>
		</span>
	</header>
	<App />
</div>
