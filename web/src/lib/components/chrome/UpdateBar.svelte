<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Meter from '$ui/forms/Meter.svelte';

	// Полоса обновления оболочек (Настольный 0.3.0, Мастерская A25, кадры
	// 10г–10е): заголовок, отказ подписи, ход скачивания, действия. Место
	// полосы, её фон и мерки кнопок задаёт оболочка классом; здесь — общая
	// разметка и состояния, чтобы две оболочки не держали две копии.
	let {
		head,
		bad = false,
		error = '',
		busy = false,
		done = 0,
		total = 0,
		attestation = false,
		blocked = false,
		hint = '',
		class: className = '',
		onapply,
		onhide,
		onreleases
	}: {
		/** Готовая строка заголовка: «Вышла версия 0.5.5», «Вышла Мастерская 0.5.0». */
		head: string;
		/** Подпись не сошлась: установка отменена. */
		bad?: boolean;
		/** Текст отказа установки (не подписи). */
		error?: string;
		busy?: boolean;
		/** Скачано байт и известный размер (0 — неизвестен). */
		done?: number;
		total?: number;
		/** Показывать «Что нового». */
		attestation?: boolean;
		/** Другие операции заняты: установку не начинаем. */
		blocked?: boolean;
		/** Подсказка рядом с кнопкой («Сначала дождитесь окончания»). */
		hint?: string;
		class?: string;
		onapply: () => void;
		onhide: () => void;
		onreleases: () => void;
	} = $props();

	// Процент на кнопке: «Скачиваем 46 %…»; без размера — «Обновляем…».
	const percent = $derived(total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0);
</script>

<div class="upd {className}">
	<div class="upd-main">
		<div class="upd-head">{head}</div>
		{#if bad}
			<div class="upd-bad">Подпись не сошлась: обновление не установлено.</div>
		{:else if error}
			<div class="upd-bad" role="alert">{error}</div>
		{:else if busy && total > 0}
			<!-- Ход скачивания — внутри полосы: строка Meter под заголовком
			     укладывается в высоту ряда кнопок, полоса не растёт. -->
			<Meter value={done} max={total} />
		{/if}
	</div>
	<div class="upd-actions">
		<Button
			variant="colored"
			disabled={busy || bad || blocked}
			loading={busy}
			loadingLabel={total > 0 ? `Скачиваем ${percent} %…` : 'Обновляем…'}
			onclick={onapply}
		>
			Обновить и перезапустить
		</Button>
		{#if hint}<span class="upd-hint">{hint}</span>{/if}
		<TextButton disabled={busy} onclick={onhide}>Позже</TextButton>
		{#if attestation}
			<TextButton onclick={onreleases}>Что нового</TextButton>
		{/if}
	</div>
</div>

<style>
	.upd {
		display: flex;
		align-items: center;
		gap: 16px;
	}
	.upd-main {
		flex: 1;
		min-width: 0;
	}
	.upd-head {
		font-weight: 600;
	}
	.upd-bad {
		color: var(--terracotta);
	}
	.upd-actions {
		display: flex;
		align-items: center;
		gap: 14px;
		flex: none;
	}
	.upd-hint {
		color: var(--muted);
	}
	/* Meter — под заголовком, внутри ряда кнопок; 2 px зазор, как в полосе
	   Настольного. Класс чужого компонента — только через :global. */
	.upd :global(.meter) {
		margin: 2px 0 0;
	}
</style>
