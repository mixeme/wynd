<script lang="ts">
	import { Call } from '@wailsio/runtime';

	function open() {
		void Call.ByName('main.App.TrayOpen');
	}
	function quit() {
		void Call.ByName('main.App.TrayQuit');
	}
	function onkeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') void Call.ByName('main.App.HideTrayMenu');
	}
</script>

<svelte:window on:keydown={onkeydown} />

<div class="tray" role="menu" aria-label="Wynd Настольный">
	<button type="button" role="menuitem" onclick={open}>Открыть</button>
	<div class="sep" role="separator"></div>
	<button type="button" role="menuitem" onclick={quit}>Выход</button>
</div>

<style>
	.tray {
		height: 100%;
		padding: 2px 0;
		background: var(--paper);
		box-shadow: inset 0 0 0 1px #cfc5b8;
		color: var(--ink);
		font-family: var(--ui);
		font-size: 12px; /* как у меню Windows: 9 pt = 12 px при 100% */
		line-height: 1.45;
		user-select: none;
	}
	button {
		display: flex;
		align-items: center;
		width: 100%;
		height: 24px;
		margin: 0;
		padding: 0 12px;
		border: 0;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: default;
	}
	button:hover,
	button:focus-visible {
		background: var(--tint);
		outline: none;
	}
	.sep {
		height: 1px;
		margin: 1px 12px;
		background: var(--line);
	}
</style>
