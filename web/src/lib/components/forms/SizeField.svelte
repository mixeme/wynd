<script lang="ts">
	import { tick } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';

	// Размер места «Своё…»: число и единица чипами «МБ / ГБ» (вариант А,
	// владелец 2026-10-17: «Нужно дать возможность просить и ставить
	// мегабайты»). Число набирается на цифровой клавиатуре, единица —
	// касанием. Наружу — байты; 0, пока число не задано.
	const MB = 1024 * 1024;
	const GB = 1024 * MB;

	let {
		bytes = $bindable(),
		admin = false,
		onchange,
		class: className = ''
	}: {
		bytes: number;
		/** Поле панели администратора. */
		admin?: boolean;
		/** Число или единицу поменяли — сохранить. */
		onchange?: () => void;
		class?: string;
	} = $props();

	function unitFor(b: number): 'MB' | 'GB' {
		return b >= GB && b % GB === 0 ? 'GB' : 'MB';
	}

	let unit = $state<'MB' | 'GB'>('MB');
	let amount = $state<string | number>('');
	// Что поле само отдало наружу: новое значение снаружи (загрузка, другой
	// круг) перечитывается, своё — нет.
	let emitted: number | undefined;

	$effect(() => {
		if (bytes === emitted) return;
		emitted = bytes;
		unit = unitFor(bytes);
		amount = bytes > 0 ? Math.round(bytes / (unit === 'GB' ? GB : MB)) : '';
	});

	$effect(() => {
		const n = Number(amount);
		const next = Number.isFinite(n) && n > 0 ? Math.round(n * (unit === 'GB' ? GB : MB)) : 0;
		if (next !== emitted) {
			emitted = next;
			bytes = next;
		}
	});

	async function changed() {
		await tick();
		onchange?.();
	}

	function pick(next: 'MB' | 'GB') {
		if (unit === next) return;
		unit = next;
		void changed();
	}
</script>

<div class="{admin ? 'flex-mid gap-8' : 'rowin gap-8 mt-10'} {className}">
	<Input
		{admin}
		active={!admin}
		type="number"
		min="1"
		class={admin ? 'w88' : 'num'}
		bind:value={amount}
		onchange={() => void changed()}
	/>
	<Chip selected={unit === 'MB'} onclick={() => pick('MB')}>МБ</Chip>
	<Chip selected={unit === 'GB'} onclick={() => pick('GB')}>ГБ</Chip>
</div>
