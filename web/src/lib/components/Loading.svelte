<script lang="ts">
	import Mark from './Mark.svelte';

	// Экран ещё грузится. Вместо строки «Загрузка…» в углу — знак Wynd по
	// центру: покачивается, как на ветру. Появляется не сразу: быстрая загрузка
	// не мигает знаком, долгая — показывает, что приложение живо.
	// compact — внутри раздела (панель): не на весь экран, а на строку-другую.
	let {
		compact = false,
		class: className = '',
		style = ''
	}: { compact?: boolean; class?: string; style?: string } = $props();
</script>

<div class="loading {className}" class:compact {style} role="status" aria-live="polite">
	<div class="loading-mark"><Mark class="loading-svg" /></div>
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
		.loading-mark {
			animation: none;
			opacity: 0.8;
		}
	}
</style>
