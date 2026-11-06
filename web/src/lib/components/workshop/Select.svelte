<script lang="ts">
	import { tick } from 'svelte';

	let {
		value = $bindable(''),
		options,
		disabled = false,
		label = ''
	}: {
		value?: string;
		options: { value: string; label: string }[];
		disabled?: boolean;
		label?: string;
	} = $props();

	let open = $state(false);
	let active = $state(0);
	let trigger = $state<HTMLButtonElement>();
	let menuStyle = $state('');
	let root = $state<HTMLDivElement>();

	const current = $derived(options.find((option) => option.value === value) ?? options[0]);

	function placeMenu() {
		if (!trigger) return;
		const rect = trigger.getBoundingClientRect();
		const below = window.innerHeight - rect.bottom;
		const up = below < 190 && rect.top > below;
		menuStyle = [
			`left: ${Math.round(rect.left)}px`,
			`width: ${Math.round(rect.width)}px`,
			up
				? `bottom: ${Math.round(window.innerHeight - rect.top + 4)}px`
				: `top: ${Math.round(rect.bottom + 4)}px`
		].join('; ');
	}

	async function show() {
		if (disabled) return;
		active = Math.max(
			0,
			options.findIndex((option) => option.value === value)
		);
		open = true;
		await tick();
		placeMenu();
	}

	function close() {
		open = false;
	}

	function choose(next: string) {
		value = next;
		close();
		trigger?.focus();
	}

	function onKeydown(event: KeyboardEvent) {
		if (disabled) return;
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			if (!open) {
				void show();
				return;
			}
			const step = event.key === 'ArrowDown' ? 1 : -1;
			active = (active + step + options.length) % options.length;
			return;
		}
		if (event.key === 'Home' || event.key === 'End') {
			if (!open) return;
			event.preventDefault();
			active = event.key === 'Home' ? 0 : options.length - 1;
			return;
		}
		if (event.key === 'Escape') {
			if (open) {
				event.preventDefault();
				close();
			}
			return;
		}
		if (event.key === 'Tab') {
			close();
			return;
		}
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			if (!open) {
				void show();
			} else if (options[active]) {
				choose(options[active].value);
			}
		}
	}

	$effect(() => {
		if (!open) return;
		const outside = (event: PointerEvent) => {
			if (!root?.contains(event.target as Node)) close();
		};
		const reposition = () => placeMenu();
		window.addEventListener('pointerdown', outside);
		window.addEventListener('resize', reposition);
		document.addEventListener('scroll', reposition, true);
		return () => {
			window.removeEventListener('pointerdown', outside);
			window.removeEventListener('resize', reposition);
			document.removeEventListener('scroll', reposition, true);
		};
	});
</script>

<div class="ins-select" bind:this={root}>
	<button
		type="button"
		class="fld"
		bind:this={trigger}
		{disabled}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-label={label || undefined}
		onclick={() => (open ? close() : void show())}
		onkeydown={onKeydown}
	>
		<span class="ins-select-label">{current?.label ?? ''}</span>
		<span class="ins-select-chev" class:open aria-hidden="true"></span>
	</button>
	{#if open}
		<div class="ins-select-menu" role="listbox" style={menuStyle} aria-label={label || undefined}>
			{#each options as option, index (option.value)}
				<button
					type="button"
					role="option"
					aria-selected={option.value === value}
					class:active={index === active}
					class:selected={option.value === value}
					onmouseenter={() => (active = index)}
					onclick={() => choose(option.value)}
				>
					{option.label}
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.ins-select {
		position: relative;
		min-width: 0;
	}
	.ins-select > button {
		display: flex;
		align-items: center;
		width: 100%;
		min-height: 39px;
		padding: 8px 32px 8px 11px;
		box-sizing: border-box;
		border: 1px solid var(--line);
		border-radius: 7px;
		background: var(--card);
		color: var(--ink);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.ins-select > button:focus-visible {
		border-color: var(--c);
		outline: none;
	}
	.ins-select > button:disabled {
		cursor: default;
		opacity: 0.55;
	}
	.ins-select-label {
		display: block;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.ins-select-chev {
		position: absolute;
		right: 13px;
		top: 50%;
		width: 7px;
		height: 7px;
		border-right: 1.5px solid var(--muted);
		border-bottom: 1.5px solid var(--muted);
		transform: translateY(-70%) rotate(45deg);
		transform-origin: center;
		pointer-events: none;
	}
	.ins-select-chev.open {
		transform: translateY(-30%) rotate(225deg);
	}
	.ins-select-menu {
		position: fixed;
		z-index: 60;
		max-height: 210px;
		padding: 4px;
		box-sizing: border-box;
		overflow-y: auto;
		border: 1px solid var(--line);
		border-radius: 7px;
		background: var(--card);
		box-shadow: 0 10px 30px color-mix(in srgb, var(--ink) 18%, transparent);
	}
	.ins-select-menu > button {
		display: block;
		width: 100%;
		padding: 8px 10px;
		border: 0;
		border-radius: 5px;
		background: transparent;
		color: var(--ink);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.ins-select-menu > button.active {
		background: var(--tint);
	}
	.ins-select-menu > button.selected {
		color: var(--c);
		font-weight: 500;
	}
</style>
