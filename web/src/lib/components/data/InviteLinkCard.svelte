<script lang="ts">
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Button from '$ui/forms/Button.svelte';

	// Ссылка-приглашение целиком и «Поделиться / Скопировать» под ней (6.7,
	// 6.21; план 47, 2.12). Была скопирована в «Пригласить» и в лист живой
	// ссылки. QR над ней экран ставит сам (QrCode) — между ними бывает подпись.
	let {
		url,
		shared = false,
		copied = false,
		onshare,
		oncopy
	}: {
		url: string;
		/** Только что поделились — «Отправлено». */
		shared?: boolean;
		/** Только что скопировали — «Скопировано». */
		copied?: boolean;
		onshare: () => void;
		oncopy: () => void;
	} = $props();
</script>

<FieldDisplay mono value={url} class="invite-url" />
<div class="rowin mt-12">
	<Button variant="colored" class="grow" onclick={onshare}>
		{shared ? 'Отправлено' : 'Поделиться'}
	</Button>
	<Button variant="ghost" class="grow" onclick={oncopy}>
		{copied ? 'Скопировано' : 'Скопировать'}
	</Button>
</div>
