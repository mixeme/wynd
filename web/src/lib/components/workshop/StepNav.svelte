<script lang="ts">
	// Колонка шагов слева в окне Мастерской (план 54): пройденные — ✓, текущий
	// выделен, на отказе — «!».
	let {
		steps,
		index,
		bad = false,
		label = 'Шаги установки',
		class: className = '',
		style = ''
	}: {
		steps: string[];
		index: number;
		bad?: boolean;
		label?: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<nav class="ins-side {className}" aria-label={label} {style}>
	{#each steps as step, i (step)}
		<div
			class="ins-step"
			class:done={i < index}
			class:on={i === index && !bad}
			class:bad={i === index && bad}
			aria-current={i === index ? 'step' : undefined}
		>
			<i>{i < index ? '✓' : i === index && bad ? '!' : i + 1}</i>{step}
		</div>
	{/each}
</nav>

<style>
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
