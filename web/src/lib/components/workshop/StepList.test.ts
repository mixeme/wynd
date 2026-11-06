import { flushSync, mount, unmount } from 'svelte';
import { expect, it } from 'vitest';
import StepList from './StepList.svelte';

function render(steps: Array<{
	id: string;
	title: string;
	status: 'pending' | 'running' | 'done' | 'failed';
	note?: string;
	latest?: string;
	latest_note?: string;
}>): string {
	const target = document.createElement('div');
	const instance = mount(StepList, { target, props: { steps } });
	flushSync();
	const html = target.innerHTML;
	unmount(instance);
	return html;
}

// C71.1/C71.2: под шагом сборки — последняя строка и её давность; у обычных
// шагов этих строк нет.
it('shows the build line and its age under the running step', () => {
	const html = render([
		{
			id: 'image',
			title: 'Собираем образ Wynd',
			status: 'running',
			latest: '#8 [build 3/5] RUN go build ./cmd/wynd',
			latest_note: 'Последнее сообщение — сейчас'
		},
		{ id: 'copy', title: 'Копия данных', status: 'pending' }
	]);
	expect(html).toContain('Собираем образ Wynd');
	expect(html).toContain('ins-step-latest');
	expect(html).toContain('#8 [build 3/5] RUN go build ./cmd/wynd');
	expect(html).toContain('Последнее сообщение — сейчас');
});
