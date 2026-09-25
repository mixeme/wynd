<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import {
		clampCropScale,
		clampCropTransform,
		cropAvatarBitmap,
		cropWindow,
		decodeAvatarBitmap,
		initialCropTransform,
		pinchCropTransform,
		zoomCropAroundPoint,
		type CropTransform,
		type CropViewport,
		type CroppedImage
	} from '$lib/media/crop';
	import { isDark } from '$lib/session/session.svelte';
	import type { CircleColor } from '$lib/theme/colors';

	let {
		file,
		color,
		ondone,
		oncancel
	}: {
		file: File;
		color: CircleColor;
		ondone: (crop: CroppedImage) => void | Promise<void>;
		oncancel: () => void;
	} = $props();

	let cropEl: HTMLDivElement | undefined = $state();
	let viewportEl: HTMLDivElement | undefined = $state();
	let cancelBtn: HTMLButtonElement | undefined = $state();
	let doneBtn: HTMLButtonElement | undefined = $state();
	let previewEl: HTMLImageElement | undefined = $state();
	let viewportW = $state(0);
	let viewportH = $state(0);
	let imageW = $state(0);
	let imageH = $state(0);
	let transform = $state<CropTransform>({ scale: 1, centerX: 0, centerY: 0 });
	let objectUrl = $state('');
	let saving = $state(false);
	let ready = $state(false);
	let error = $state('');
	let seededFor = $state('');
	let viewportKey = $state('');

	let dragStart: { x: number; y: number; centerX: number; centerY: number } | undefined;
	let pinchStart:
		| {
				distance: number;
				scale: number;
				centerX: number;
				centerY: number;
				anchorX: number;
				anchorY: number;
		  }
		| undefined;

	const colorClass = $derived(color !== 'terracotta' ? color : undefined);
	const darkClass = $derived(isDark());
	const seedKey = $derived(`${file.name}:${file.size}:${file.lastModified}`);

	const cropViewport = $derived.by((): CropViewport | undefined => {
		if (!viewportW || !viewportH) return undefined;
		const cropDiameter = Math.min(viewportW - 32, viewportH - 32, 300);
		return { width: viewportW, height: viewportH, cropDiameter };
	});

	const cropRing = $derived.by(() => {
		if (!cropViewport) return undefined;
		return cropWindow(cropViewport);
	});

	const display = $derived.by(() => {
		if (!imageW || !imageH) return undefined;
		return {
			width: imageW * transform.scale,
			height: imageH * transform.scale,
			left: transform.centerX - (imageW * transform.scale) / 2,
			top: transform.centerY - (imageH * transform.scale) / 2
		};
	});

	$effect(() => {
		const url = URL.createObjectURL(file);
		objectUrl = url;
		ready = false;
		seededFor = '';
		error = '';
		let cancelled = false;
		const img = new Image();
		img.onload = () => {
			if (cancelled) return;
			imageW = img.naturalWidth;
			imageH = img.naturalHeight;
			ready = true;
		};
		img.onerror = () => {
			if (cancelled) return;
			error = 'Не удалось открыть фото';
			ready = false;
		};
		img.src = url;
		return () => {
			cancelled = true;
			URL.revokeObjectURL(url);
		};
	});

	$effect(() => {
		if (!ready || !cropViewport || !imageW || !imageH) return;
		if (seededFor === seedKey) return;
		transform = initialCropTransform(imageW, imageH, cropViewport);
		seededFor = seedKey;
		viewportKey = `${viewportW}x${viewportH}`;
	});

	$effect(() => {
		if (!ready || !cropViewport || !imageW || !imageH || seededFor !== seedKey) return;
		const key = `${viewportW}x${viewportH}`;
		if (!viewportKey || key === viewportKey) return;
		viewportKey = key;
		transform = clampCropTransform(imageW, imageH, cropViewport, transform);
	});

	$effect(() => {
		if (ready && doneBtn && !saving) doneBtn.focus();
	});

	function applyTransform(next: CropTransform) {
		if (!cropViewport || !imageW || !imageH) return;
		const scale = clampCropScale(imageW, imageH, cropViewport, next.scale);
		transform = clampCropTransform(imageW, imageH, cropViewport, { ...next, scale });
	}

	function zoomAroundPoint(
		anchorX: number,
		anchorY: number,
		nextScale: number,
		base: CropTransform
	) {
		applyTransform(zoomCropAroundPoint(base, anchorX, anchorY, nextScale));
	}

	function touchMidpoint(touches: TouchList, rect: DOMRect) {
		return {
			x: (touches[0].clientX + touches[1].clientX) / 2 - rect.left,
			y: (touches[0].clientY + touches[1].clientY) / 2 - rect.top
		};
	}

	function onPointerDown(e: PointerEvent) {
		if (!viewportEl || e.button !== 0) return;
		viewportEl.setPointerCapture(e.pointerId);
		dragStart = {
			x: e.clientX,
			y: e.clientY,
			centerX: transform.centerX,
			centerY: transform.centerY
		};
	}

	function onPointerMove(e: PointerEvent) {
		if (!dragStart) return;
		applyTransform({
			...transform,
			centerX: dragStart.centerX + (e.clientX - dragStart.x),
			centerY: dragStart.centerY + (e.clientY - dragStart.y)
		});
	}

	function onPointerUp(e: PointerEvent) {
		if (dragStart) {
			viewportEl?.releasePointerCapture(e.pointerId);
			dragStart = undefined;
		}
	}

	function touchDistance(touches: TouchList): number {
		const dx = touches[0].clientX - touches[1].clientX;
		const dy = touches[0].clientY - touches[1].clientY;
		return Math.hypot(dx, dy);
	}

	function onTouchStart(e: TouchEvent) {
		if (e.touches.length === 2 && viewportEl) {
			const rect = viewportEl.getBoundingClientRect();
			const mid = touchMidpoint(e.touches, rect);
			pinchStart = {
				distance: touchDistance(e.touches),
				scale: transform.scale,
				centerX: transform.centerX,
				centerY: transform.centerY,
				anchorX: mid.x,
				anchorY: mid.y
			};
			dragStart = undefined;
		}
	}

	function onTouchMove(e: TouchEvent) {
		if (e.touches.length !== 2 || !pinchStart || !cropViewport || !viewportEl) return;
		if (pinchStart.distance === 0) return;
		e.preventDefault();
		const rect = viewportEl.getBoundingClientRect();
		const mid = touchMidpoint(e.touches, rect);
		const nextScale = clampCropScale(
			imageW,
			imageH,
			cropViewport,
			pinchStart.scale * (touchDistance(e.touches) / pinchStart.distance)
		);
		const distance =
			pinchStart.scale === 0
				? pinchStart.distance
				: pinchStart.distance * (nextScale / pinchStart.scale);
		applyTransform(
			pinchCropTransform(
				pinchStart,
				pinchStart.anchorX,
				pinchStart.anchorY,
				pinchStart.distance,
				mid.x,
				mid.y,
				distance
			)
		);
	}

	function onTouchEnd() {
		pinchStart = undefined;
	}

	function onWheel(e: WheelEvent) {
		if (!cropViewport || !viewportEl) return;
		e.preventDefault();
		const rect = viewportEl.getBoundingClientRect();
		const factor = e.deltaY < 0 ? 1.08 : 1 / 1.08;
		const nextScale = clampCropScale(imageW, imageH, cropViewport, transform.scale * factor);
		zoomAroundPoint(e.clientX - rect.left, e.clientY - rect.top, nextScale, transform);
	}

	function requestCancel() {
		if (saving) return;
		oncancel();
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			requestCancel();
			return;
		}
		if (e.key !== 'Tab') return;
		const focusables = [cancelBtn, doneBtn].filter(Boolean) as HTMLButtonElement[];
		if (!focusables.length) return;
		const active = document.activeElement;
		const idx = focusables.indexOf(active as HTMLButtonElement);
		if (idx === -1) {
			e.preventDefault();
			focusables[0].focus();
			return;
		}
		e.preventDefault();
		const next = e.shiftKey
			? focusables[(idx - 1 + focusables.length) % focusables.length]
			: focusables[(idx + 1) % focusables.length];
		next.focus();
	}

	function onFocusIn(e: FocusEvent) {
		if (!cropEl) return;
		const target = e.target as Node | null;
		if (target && cropEl.contains(target)) return;
		doneBtn?.focus();
	}

	async function finish() {
		if (!cropViewport || saving || !ready) return;
		saving = true;
		error = '';
		try {
			const source = previewEl ?? file;
			const bitmap = await decodeAvatarBitmap(source);
			try {
				const crop = await cropAvatarBitmap(bitmap, cropViewport, transform, file.name);
				await ondone(crop);
			} finally {
				bitmap.close();
			}
		} catch (err) {
			error =
				err instanceof Error && err.message
					? err.message
					: 'Не удалось кадрировать фото';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:window onkeydown={onKeydown} onfocusin={onFocusIn} />

<div
	class="crop ph app {colorClass}"
	class:dark={darkClass}
	bind:this={cropEl}
	role="dialog"
	aria-modal="true"
	aria-label="Кадр"
	aria-busy={saving}
>
	<div class="cbar" style="padding-bottom:8px">
		<div class="top">
			<IconButton
				bind:el={cancelBtn}
				name="x"
				label="Отмена"
				onclick={requestCancel}
			/>
			<span class="t">Кадр</span>
			<button
				type="button"
				class="done"
				class:off={saving || !ready}
				disabled={!ready}
				bind:this={doneBtn}
				onclick={() => void finish()}
			>
				{saving ? '…' : 'Готово'}
			</button>
		</div>
	</div>

	<div
		class="viewport"
		role="img"
		aria-label="Кадрирование фото"
		bind:this={viewportEl}
		bind:clientWidth={viewportW}
		bind:clientHeight={viewportH}
		onpointerdown={onPointerDown}
		onpointermove={onPointerMove}
		onpointerup={onPointerUp}
		onpointercancel={onPointerUp}
		ontouchstart={onTouchStart}
		ontouchmove={onTouchMove}
		ontouchend={onTouchEnd}
		onwheel={onWheel}
	>
		{#if objectUrl && display}
			<img
				bind:this={previewEl}
				class="photo"
				src={objectUrl}
				alt=""
				draggable="false"
				style:left="{display.left}px"
				style:top="{display.top}px"
				style:width="{display.width}px"
				style:height="{display.height}px"
			/>
		{/if}
		{#if cropRing}
			<div
				class="ring"
				style:left="{cropRing.left}px"
				style:top="{cropRing.top}px"
				style:width="{cropRing.size}px"
				style:height="{cropRing.size}px"
			></div>
		{/if}
	</div>

	<Hint centered style="margin:14px 16px 22px">
		{#if error}
			{error}
		{:else}
			Так вас увидят в круге. Сдвиньте снимок или разведите пальцы.
		{/if}
	</Hint>
</div>

<style>
	.crop {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		flex-direction: column;
		width: auto;
		height: auto;
		min-height: 0;
		overflow: hidden;
		background: var(--paper);
		color: var(--ink);
		border: none;
		border-radius: 0;
	}

	.done {
		margin-left: auto;
		font-size: 15px;
		font-weight: 600;
		color: #fff;
	}

	.done.off,
	.done:disabled {
		opacity: 0.55;
		pointer-events: none;
	}

	.viewport {
		position: relative;
		flex: 1;
		overflow: hidden;
		touch-action: none;
		cursor: grab;
	}

	.viewport:active {
		cursor: grabbing;
	}

	.photo {
		position: absolute;
		user-select: none;
		pointer-events: none;
	}

	.ring {
		position: absolute;
		border-radius: 50%;
		border: 1px solid var(--ink);
		box-shadow: 0 0 0 9999px color-mix(in srgb, var(--paper) 88%, transparent);
		pointer-events: none;
	}
</style>
