<script lang="ts">
	// Своя полоса заголовка окна Мастерской (план 54; docs/visual/installer.html,
	// полоса .tb): окно без системной рамки, заголовок и кнопки рисуем сами,
	// чтобы на любой Windows окно было как на макете. Полоса таскает окно
	// (--wails-draggable), двойной щелчок и прилипание к краю — от системы.
	let {
		title,
		maximised = false,
		onminimise,
		ontoggle,
		onclose
	}: {
		title: string;
		maximised?: boolean;
		onminimise: () => void;
		ontoggle: () => void;
		onclose: () => void;
	} = $props();
</script>

<header class="ins-tb">
	<span class="ins-tb-dot" aria-hidden="true">●</span>
	<b>{title}</b>
	<span class="ins-tb-btns">
		<!-- Значки — свои линии 10 × 10, а не символы шрифта: у «—», «▢», «✕»
		     разная ширина и посадка, в кнопке они стояли левее центра. -->
		<button type="button" aria-label="Свернуть" onclick={onminimise}>
			<svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 5.5h10" /></svg>
		</button>
		<button type="button" aria-label={maximised ? 'Восстановить' : 'Развернуть'} onclick={ontoggle}>
			{#if maximised}
				<svg viewBox="0 0 10 10" aria-hidden="true"
					><path d="M2.5 2.5V.5h7v7h-2" /><rect x=".5" y="2.5" width="7" height="7" /></svg
				>
			{:else}
				<svg viewBox="0 0 10 10" aria-hidden="true"><rect x=".5" y=".5" width="9" height="9" /></svg
				>
			{/if}
		</button>
		<button type="button" class="ins-tb-close" aria-label="Закрыть" onclick={onclose}>
			<svg viewBox="0 0 10 10" aria-hidden="true"><path d="M.5.5l9 9M9.5.5l-9 9" /></svg>
		</button>
	</span>
</header>

<style>
	.ins-tb {
		--wails-draggable: drag;
		flex: none;
		display: flex;
		align-items: center;
		height: 34px;
		padding-left: 12px;
		background: #eae3d8;
		border-bottom: 1px solid var(--line);
		font-size: 12px;
		color: var(--muted);
		user-select: none;
	}
	.ins-tb b {
		margin-left: 8px;
		color: var(--ink);
		font-weight: 600;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.ins-tb-btns {
		--wails-draggable: no-drag;
		display: flex;
		align-self: stretch;
		margin-left: auto;
	}
	.ins-tb-btns button {
		display: grid;
		place-items: center;
		width: 44px;
		padding: 0;
		border: 0;
		background: none;
		/* --muted, не #b9afa3: тот на полосе #eae3d8 давал 1,7 : 1, значки
		   почти не видели (C68); --muted — 3,7 : 1, тёмный — под мышью. */
		color: var(--muted);
		cursor: default;
	}
	.ins-tb-btns svg {
		display: block;
		width: 10px;
		height: 10px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1;
	}
	.ins-tb-btns button:hover {
		background: rgba(0, 0, 0, 0.06);
		color: var(--ink);
	}
	.ins-tb-btns .ins-tb-close:hover {
		background: #b5482f;
		color: #fff;
	}
</style>
