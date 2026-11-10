<script lang="ts">
	import '$lib/styles/tokens.css';
	import '$lib/styles/ui.css';
	import { initQueueDrain } from '$lib/queue/queue';
	import { initConnection } from '$lib/session/connection.svelte';
	import { initPush } from '$lib/push/push';
	import { getAvatarShape, initSession, initTheme, isDark } from '$lib/session/session.svelte';
	import { initSyncResume, startSyncForAllSessions } from '$lib/sync/sync';
	import { initViewportHeight } from '$lib/session/viewport';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { initAppUpdate } from '$lib/session/appUpdate.svelte';
	import { statusColorOverride } from '$lib/session/statusColor.svelte';
	import { afterNavigate, beforeNavigate } from '$app/navigation';
	import { armEntryGuard, interceptHistoryBack, recordEntry } from '$lib/navigation/up';
	import AudioBar from '$ui/chrome/AudioBar.svelte';
	import Icon from '$ui/Icon.svelte';
	import StatusCard from '$ui/data/StatusCard.svelte';
	import { useAudioBar } from '$lib/media/audioBar.svelte';
	import { desktopBridge } from '$lib/desktop/bridge';
	import {
		acceptDesktopDrop,
		beginDesktopDrop,
		desktopDropActive,
		desktopDropEnabled,
		endDesktopDrop
	} from '$lib/desktop/drop.svelte';
	import { pwaInfo } from 'virtual:pwa-info';

	// Полоса плеера (4.9): звук играет и вне своего экрана (план 47, 5.4).
	const audioBar = useAudioBar();

	let { children } = $props();

	// «Назад» на уровень вверх и системной кнопкой (план 46, C13).
	beforeNavigate((nav) => {
		if (nav.type === 'popstate' && interceptHistoryBack(nav.delta)) nav.cancel();
	});
	afterNavigate((nav) => {
		if (!nav.to) return;
		recordEntry(nav.to.url);
		const url = nav.to.url;
		setTimeout(() => armEntryGuard(url));
	});

	$effect(() => {
		if (typeof document === 'undefined') return;
		document.body.classList.toggle('dark', isDark());
		// Форма аватаров (A16): стили в ui.css по body[data-avatars].
		document.body.dataset.avatars = getAvatarShape();
		// Строка состояния установленного приложения — в цвет фона темы (SW-4).
		// Метка стоит в app.html: так её видит и Vivaldi при запуске.
		document
			.querySelector('meta[name="theme-color"]')
			?.setAttribute('content', statusColorOverride() ?? (isDark() ? '#211E1C' : '#F4F0E9'));
	});

	onMount(() => {
		window.__wyndBoot?.('mounted');
		const stopTheme = initTheme();
		const stopQueue = initQueueDrain();
		const stopViewport = initViewportHeight();
		const stopSyncResume = initSyncResume();
		const stopConnection = initConnection();
		// В настольном окне служебный воркер не нужен: оболочка приносит
		// клиент вместе с сервером, а кэш PWA в окне отдавал старый интерфейс.
		const isLocal =
			location.hostname === '127.0.0.1' ||
			location.hostname === 'localhost' ||
			Boolean(desktopBridge());
		// Новая версия не перезагружает открытый экран: она встаёт при
		// следующем запуске или из «Настроек» (C19).
		if (pwaInfo && !isLocal) initAppUpdate();
		void initSession().then(() => {
			window.__wyndBoot?.('session');
			startSyncForAllSessions();
			void initPush();
		});
		return () => {
			stopTheme();
			stopSyncResume();
			stopConnection();
			stopQueue();
			stopViewport();
		};
	});

	// Файлы из Проводника (ПК6/ПК6а): только в окне для компьютера.
	onMount(() => {
		if (!desktopDropEnabled()) return;
		let depth = 0;
		const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer?.types ?? []).includes('Files');
		const enter = (e: DragEvent) => {
			if (!hasFiles(e)) return;
			e.preventDefault();
			depth += 1;
			beginDesktopDrop(location.pathname);
		};
		const over = (e: DragEvent) => {
			if (!hasFiles(e)) return;
			e.preventDefault();
			if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
		};
		const leave = (e: DragEvent) => {
			if (!hasFiles(e)) return;
			depth = Math.max(0, depth - 1);
			if (depth === 0) endDesktopDrop();
		};
		const drop = (e: DragEvent) => {
			if (!hasFiles(e)) return;
			e.preventDefault();
			depth = 0;
			void acceptDesktopDrop(location.pathname, Array.from(e.dataTransfer?.files ?? []));
		};
		const stop = () => {
			depth = 0;
			endDesktopDrop();
		};
		window.addEventListener('dragenter', enter);
		window.addEventListener('dragover', over);
		window.addEventListener('dragleave', leave);
		window.addEventListener('drop', drop);
		window.addEventListener('dragend', stop);
		return () => {
			window.removeEventListener('dragenter', enter);
			window.removeEventListener('dragover', over);
			window.removeEventListener('dragleave', leave);
			window.removeEventListener('drop', drop);
			window.removeEventListener('dragend', stop);
		};
	});
</script>

<svelte:head>
	<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32.png" />
	<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
	<link rel="apple-touch-icon" href="/icon-192.png" />
	<style>
		/* Golos Text (SIL OFL), variable, split by unicode-range as upstream ships it. */
		@font-face {
			font-family: 'Golos Text';
			src: url('/fonts/GolosText-Variable-cyrillic.woff2') format('woff2');
			font-weight: 400 900;
			font-style: normal;
			font-display: swap;
			unicode-range: U+0301, U+0400-045F, U+0490-0491, U+04B0-04B1, U+2116;
		}
		@font-face {
			font-family: 'Golos Text';
			src: url('/fonts/GolosText-Variable-cyrillic-ext.woff2') format('woff2');
			font-weight: 400 900;
			font-style: normal;
			font-display: swap;
			unicode-range: U+0460-052F, U+1C80-1C8A, U+20B4, U+2DE0-2DFF, U+A640-A69F, U+FE2E-FE2F;
		}
		@font-face {
			font-family: 'Golos Text';
			src: url('/fonts/GolosText-Variable-latin.woff2') format('woff2');
			font-weight: 400 900;
			font-style: normal;
			font-display: swap;
			unicode-range:
				U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308,
				U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
		}
		@font-face {
			font-family: 'Golos Text';
			src: url('/fonts/GolosText-Variable-latin-ext.woff2') format('woff2');
			font-weight: 400 900;
			font-style: normal;
			font-display: swap;
			unicode-range:
				U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329,
				U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C0, U+2113,
				U+2C60-2C7F, U+A720-A7FF;
		}
	</style>
</svelte:head>

<div class="app" class:dev={$page.url.pathname.startsWith('/dev')}>
	{@render children()}
	{#if audioBar.view}
		<AudioBar
			{...audioBar.view}
			onopen={audioBar.open}
			ontoggle={audioBar.toggle}
			onstop={audioBar.stop}
		/>
	{/if}
	{#if desktopDropActive()}
		<div class="drop-zone" aria-hidden="true">
			<div class="drop-zone-frame">
				<span class="drop-zone-icon"><Icon name="file" /></span>
				<StatusCard
					class="drop-card"
					title="Отпустите файлы"
					text="Они станут вложениями новой записи. Можно фото, видео, аудио и любые файлы."
				/>
			</div>
		</div>
	{/if}
</div>

<style>
	:global(*) {
		box-sizing: border-box;
	}

	:global(html) {
		overflow-x: clip;
		max-width: 100%;
		-webkit-text-size-adjust: 100%;
		text-size-adjust: 100%;
	}

	:global(body) {
		margin: 0;
		overflow-x: clip;
		max-width: 100%;
		background: var(--paper);
		color: var(--ink);
		font-family: var(--ui);
	}

	/* Тёмная тема красит окно (.ph.dark), а страница под ним оставалась
	   светлой. Её видно, пока клавиатура уезжает, а окно ещё не выросло:
	   белая полоса над клавиатурой на пару кадров (Firefox). Цвет — --paper
	   тёмной темы из tokens.css. */
	:global(body.dark) {
		background: #211e1c;
	}

	/* Safari на iPhone увеличивает страницу, если в поле кегль меньше 16.
	   После этого экран шире окна: его листают вбок или щипают обратно. */
	@media (pointer: coarse) {
		:global(input:not([type='file']):not([type='checkbox']):not([type='radio'])),
		:global(textarea),
		:global(select) {
			font-size: 16px;
		}
	}

	@media (min-width: 481px) {
		:global(body) {
			background: var(--app-gutter);
		}

		:global(body.dark) {
			background: var(--app-gutter-dark);
			color: #efe9e0;
		}
	}

	.app.dev {
		min-height: 100vh;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: 32px 16px 64px;
	}

	/* ПК6а: зона приёма, пока файлы несут над открытым кругом. */
	.drop-zone {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: grid;
		place-items: center;
		background: rgba(244, 240, 233, 0.86);
	}
	.drop-zone-frame {
		width: 360px;
		max-width: calc(100% - 48px);
		padding: 26px 26px 24px;
		border: 2px dashed var(--terracotta);
		border-radius: 16px;
		text-align: center;
		background: var(--paper);
	}
	.drop-zone-icon {
		display: block;
		width: 32px;
		height: 32px;
		margin: 0 auto 10px;
		color: var(--terracotta);
	}
	.drop-zone-icon :global(.ic) {
		width: 32px;
		height: 32px;
	}
	:global(.drop-card) {
		margin: 0;
		border: 0;
		padding: 0;
		background: transparent;
	}
	:global(.drop-card .status-card-title) {
		font-size: 18px;
	}
	:global(body.dark) .drop-zone {
		background: rgba(33, 30, 28, 0.86);
	}
</style>
