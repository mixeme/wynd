import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import VoiceRow from './VoiceRow.svelte';

// В очереди у голосового нет blobId, и играть его нечем. Строка тогда не
// кнопка: нажатие проходит к карточке записи и открывает правку, как у
// реплик с текстом (план 46, C78).
describe('VoiceRow', () => {
	it('renders a span instead of a button when there is nothing to play', () => {
		const target = document.createElement('div');
		const instance = mount(VoiceRow, { target, props: { blobId: '', durationMs: 4200 } });
		expect(target.querySelector('.voice')?.tagName).toBe('SPAN');
		unmount(instance);
	});

	it('stays a play button with a blob', () => {
		const target = document.createElement('div');
		const instance = mount(VoiceRow, { target, props: { blobId: 'b1' } });
		expect(target.querySelector('.voice')?.tagName).toBe('BUTTON');
		unmount(instance);
	});
});
