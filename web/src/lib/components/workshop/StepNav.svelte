<script lang="ts">
	// Колонка шагов слева в окне Мастерской (план 54): пройденные — ✓, текущий
	// выделен, на отказе — «!».
	let {
		steps,
		index,
		servers,
		selected = '',
		onselect,
		onadd,
		bad = false,
		label = 'Шаги установки',
		class: className = '',
		style = ''
	}: {
		steps?: string[];
		index?: number;
		servers?: { id: string; name: string; host: string; status: string; busy?: boolean }[];
		selected?: string;
		onselect?: (id: string) => void;
		onadd?: () => void;
		bad?: boolean;
		label?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if servers}
	<nav class="ins-server-rail {className}" aria-label="Серверы" {style}>
		<div class="ins-rail-head">
			<h3>Серверы</h3>
			<button type="button" class="under" disabled={!onadd} onclick={onadd}>+ Добавить</button>
		</div>
		{#if servers.length === 0}
			<p class="ins-empty-servers">Сохранённых серверов пока нет.</p>
		{/if}
		{#each servers as server (server.id)}
			<button
				type="button"
				class="ins-server-entry"
				data-profile={server.id}
				aria-label={`${server.name} · ${server.host} · ${server.status}`}
				class:selected={server.id === selected}
				aria-pressed={server.id === selected}
				onclick={() => onselect?.(server.id)}
			>
				<strong>{server.name}</strong><small>{server.host}</small><small
					class:ins-rail-progress={server.busy}>{server.status}</small
				>
			</button>
		{/each}
	</nav>
{:else}
	<nav class="ins-side {className}" aria-label={label} {style}>
		{#each steps ?? [] as step, i (step)}
			<div
				class="ins-step"
				class:done={i < (index ?? 0)}
				class:on={i === index && !bad}
				class:bad={i === index && bad}
				aria-current={i === index ? 'step' : undefined}
			>
				<i>{i < (index ?? 0) ? '✓' : i === index && bad ? '!' : i + 1}</i>{step}
			</div>
		{/each}
	</nav>
{/if}

<style>
	.ins-empty-servers {
		margin: 0;
		padding: 0 8px;
		color: var(--muted);
	}
	.ins-server-rail {
		box-sizing: border-box;
		width: 225px;
		flex-shrink: 0;
		background: var(--tint);
		border-right: 1px solid var(--line);
		padding: 17px 10px;
		overflow-y: auto;
	}
	.ins-rail-head {
		display: flex;
		gap: 10px;
		align-items: center;
		justify-content: space-between;
		padding: 0 8px 13px;
	}
	.ins-rail-head h3 {
		margin: 0;
		font-size: 15px;
		font-weight: 500;
	}
	.ins-rail-head .under {
		color: var(--c);
		white-space: nowrap;
	}
	.ins-server-entry {
		display: block;
		width: 100%;
		text-align: left;
		padding: 11px 9px;
		box-sizing: border-box;
		background: transparent;
		color: var(--ink);
		border: 1px solid transparent;
		border-radius: 8px;
		margin-bottom: 7px;
		font: inherit;
	}
	.ins-server-entry.selected {
		background: var(--card);
		border-color: var(--line);
	}
	.ins-server-entry strong {
		display: block;
		font-weight: 500;
	}
	.ins-server-entry small {
		display: block;
		color: var(--muted);
		font-size: 12px;
		margin-top: 3px;
		overflow-wrap: anywhere;
	}
	.ins-server-entry .ins-rail-progress {
		color: var(--c);
	}
	@media (max-width: 680px) {
		.ins-server-rail {
			box-sizing: border-box;
			width: 100%;
			border-right: 0;
			border-bottom: 1px solid var(--line);
			padding: 12px 10px;
			max-height: 180px;
		}
		.ins-server-entry {
			padding: 7px 9px;
			margin-bottom: 3px;
		}
		.ins-server-entry small {
			display: inline;
			margin-right: 8px;
		}
	}
	.ins-side {
		flex: none;
		width: 170px;
		padding: 18px 0;
		border-right: 1px solid var(--line);
		background: var(--tint);
	}
	.ins-step {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 7px 16px;
		color: var(--faint);
	}
	.ins-step i {
		display: grid;
		place-items: center;
		width: 18px;
		height: 18px;
		border: 1.5px solid currentColor;
		border-radius: 50%;
		font-style: normal;
		font-size: 10px;
	}
	.ins-step.done {
		color: var(--ok);
	}
	.ins-step.done i {
		border-color: var(--ok);
		background: var(--ok);
		color: #fff;
	}
	.ins-step.on {
		color: var(--ink);
		font-weight: 600;
	}
	.ins-step.on i {
		border-color: var(--c);
		color: var(--c);
	}
	.ins-step.bad {
		color: var(--bad);
		font-weight: 600;
	}
</style>
