<script lang="ts">
	import Logo from './Logo.svelte';
	import Mark from './Mark.svelte';

	// Экран ещё грузится. Начальная загрузка — логотип по центру, тот же лок,
	// что на «Войти». Внутри раздела остаётся знак: покачивается, как на ветру.
	// Появляется не сразу: быстрая загрузка не мигает, долгая — показывает,
	// что приложение живо.
	// compact — внутри раздела (панель): не на весь экран, а на строку-другую.
	let {
		compact = false,
		logo = false,
		class: className = '',
		style = ''
	}: { compact?: boolean; logo?: boolean; class?: string; style?: string } = $props();
</script>

<div class="loading {className}" class:compact {style} role="status" aria-live="polite">
	{#if logo}
		<div class="loading-logo"><Logo height={48} /></div>
	{:else}
		<div class="loading-mark"><Mark class="loading-svg" /></div>
	{/if}
	<span class="vh">Загрузка…</span>
</div>

<style>
	.loading {
		display: grid;
		place-items: center;
		min-height: 70vh;
		opacity: 0;
		animation: loading-in 0.4s ease-out 0.3s forwards;
	}
	/* Знак — в одной точке окна при любой шапке: блок высотой 70vh начинался
	   под шапкой, а шапки у улочки, круга, форм и админки разные — знак
	   прыгал по высоте (план 46, C10). Касаний не перехватывает. */
	.loading:not(.compact) {
		position: fixed;
		inset: 0;
		min-height: 0;
		pointer-events: none;
	}
	.loading.compact {
		min-height: 160px;
	}
	.loading-mark {
		width: 30px;
		height: 45px;
		color: var(--c, var(--faint));
		transform-origin: 50% 90%;
		animation: loading-sway 2.4s ease-in-out infinite;
	}
	.loading-mark :global(.loading-svg) {
		width: 100%;
		height: 100%;
	}
	.loading-logo {
		color: var(--ink);
		animation: loading-breathe 2.4s ease-in-out infinite;
	}
	.vh {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
	}
	@keyframes loading-in {
		to {
			opacity: 1;
		}
	}
	@keyframes loading-breathe {
		0%,
		100% {
			opacity: 0.55;
		}
		50% {
			opacity: 1;
		}
	}
	@keyframes loading-sway {
		0%,
		100% {
			transform: rotate(-7deg);
			opacity: 0.55;
		}
		50% {
			transform: rotate(7deg);
			opacity: 1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.loading {
			animation: none;
			opacity: 1;
		}
		.loading-mark,
		.loading-logo {
			animation: none;
			opacity: 0.8;
		}
	}
</style>
