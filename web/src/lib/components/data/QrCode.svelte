<script lang="ts">
	import QRCode from 'qrcode';

	// QR-код ссылки в рамке `.qr` (план 47, 2.12): приглашение в круг (6.7),
	// живая ссылка (6.21), приглашение на сервер в админке (9.x). Раньше каждый
	// экран сам звал QRCode.toString и вставлял SVG через {@html}.
	let {
		value,
		size = 'md',
		class: className = ''
	}: {
		/** Что закодировать; пусто — рамки нет. */
		value: string;
		/** md — 168 px на экранах телефона, sm — 150 px в колонке админки. */
		size?: 'md' | 'sm';
		class?: string;
	} = $props();

	let svg = $state('');

	$effect(() => {
		const text = value;
		let stale = false;
		if (!text) {
			svg = '';
			return;
		}
		// Размер SVG задаёт рамка (.qr > svg тянется на всю), width — лишь
		// разрешение растра модулей.
		void QRCode.toString(text, { type: 'svg', margin: 0, width: 168 }).then((out) => {
			if (!stale) svg = out;
		});
		return () => {
			stale = true;
		};
	});
</script>

{#if svg}
	<div class="qr {size === 'sm' ? 'sm' : ''} {className}" aria-hidden="true">
		<!-- eslint-disable-next-line svelte/no-at-html-tags -- SVG из qrcode, не ввод человека -->
		{@html svg}
	</div>
{/if}
