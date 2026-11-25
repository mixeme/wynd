import { flushSync, mount, unmount } from 'svelte';
import { expect, it } from 'vitest';
import StepList from './StepList.svelte';

type Steps = Array<{
	id: string;
	title: string;
	status: 'pending' | 'running' | 'done' | 'failed';
	note?: string;
	latest?: string;
	latest_note?: string;
}>;

function render(steps: Steps): string {
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

// Три точки текущего шага — по центру строки: обычное многоточие «…» стоит
// на базовой линии и читается ниже текста.
it('puts the running mark in the middle of the line', () => {
	const html = render([{ id: 'upload', title: 'Загружаем APK', status: 'running' }]);
	expect(html).toContain('···');
	expect(html).not.toContain('…');
});

// Шов двух списков («Подпись»: проверка, потом подпись): списки шагов стоят
// подряд и несут один класс — по нему MarkRows снимает шаг между ними и
// оставляет черту. Иначе на стыке пропадает разделитель и зазор выходит
// больше строки. Если класс или соседство пропадут, шов вернётся.
it('puts two step lists next to each other with the seam class', () => {
	const target = document.createElement('div');
	const first = mount(StepList, {
		target,
		props: { steps: [{ id: 'check', title: 'Читаем описание выпуска', status: 'done' }] }
	});
	const second = mount(StepList, {
		target,
		props: { steps: [{ id: 'release', title: 'Проверяем состояние выпуска', status: 'done' }] }
	});
	flushSync();
	const rows = Array.from(target.querySelectorAll<HTMLElement>('.ins-rows'));
	expect(rows).toHaveLength(2);
	expect(rows[0].classList.contains('ins-steps')).toBe(true);
	expect(rows[1].classList.contains('ins-steps')).toBe(true);
	expect(rows[0].nextElementSibling).toBe(rows[1]);
	unmount(first);
	unmount(second);
});
