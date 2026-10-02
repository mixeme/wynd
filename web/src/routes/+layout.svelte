<script lang="ts">
	import '$lib/styles/tokens.css';
	import '$lib/styles/ui.css';
	import { initQueueDrain } from '$lib/queue/queue';
	import { initPush } from '$lib/push/push';
	import { initSession, initTheme, isDark } from '$lib/session/session.svelte';
	import { startSyncForAllSessions } from '$lib/sync/sync';
	import { initViewportHeight } from '$lib/session/viewport';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { initAppUpdate } from '$lib/session/appUpdate.svelte';
	import { afterNavigate, beforeNavigate } from '$app/navigation';
	import { armEntryGuard, interceptHistoryBack, recordEntry } from '$lib/navigation/up';
	import AudioBar from '$ui/chrome/AudioBar.svelte';
	import { useAudioBar } from '$lib/media/audioBar.svelte';
	import { pwaInfo } from 'virtual:pwa-info';

	// Полоса плеера (4.20): звук играет и вне своего экрана (план 47, 5.4).
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
		// Строка состояния установленного приложения — в цвет фона темы (SW-4).
		// Метка стоит в app.html: так её видит и Vivaldi при запуске.
		document
			.querySelector('meta[name="theme-color"]')
			?.setAttribute('content', isDark() ? '#211E1C' : '#F4F0E9');
	});


	onMount(() => {
		const stopTheme = initTheme();
		const stopQueue = initQueueDrain();
		const stopViewport = initViewportHeight();
		const isLocal =
			location.hostname === '127.0.0.1' || location.hostname === 'localhost';
		// Новая версия не перезагружает открытый экран: она встаёт, когда
		// приложение ушло в фон, при следующем запуске или из «Настроек» (C19).
		if (pwaInfo && !isLocal) initAppUpdate();
		void initSession().then(() => {
			startSyncForAllSessions();
			void initPush();
		});
		return () => {
			stopTheme();
			stopQueue();
			stopViewport();
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
			unicode-range: U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC,
				U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215,
				U+FEFF, U+FFFD;
		}
		@font-face {
			font-family: 'Golos Text';
			src: url('/fonts/GolosText-Variable-latin-ext.woff2') format('woff2');
			font-weight: 400 900;
			font-style: normal;
			font-display: swap;
			unicode-range: U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304,
				U+0308, U+0329, U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB,
				U+20AD-20C0, U+2113, U+2C60-2C7F, U+A720-A7FF;
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
</style>
