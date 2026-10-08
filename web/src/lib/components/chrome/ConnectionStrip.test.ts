import { flushSync, mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import type { ConnectionNotice } from '$lib/session/connection.svelte';
import ConnectionStrip from './ConnectionStrip.svelte';
import CircleRow from '$ui/data/CircleRow.svelte';

function render(notice: ConnectionNotice | undefined): HTMLElement {
	const target = document.createElement('div');
	const instance = mount(ConnectionStrip, { target, props: { notice } });
	flushSync();
	const copy = target.cloneNode(true) as HTMLElement;
	unmount(instance);
	return copy;
}

describe('ConnectionStrip', () => {
	// На связи знака нет вовсе (7.5).
	it('без повода не рисует ничего', () => {
		expect(render(undefined).querySelector('.conn')).toBeNull();
	});

	it('нет сети (7.6)', () => {
		const el = render({ kind: 'offline' });
		expect(el.textContent?.trim()).toBe('Нет сети. Новое уйдёт само, когда связь появится');
		expect(el.querySelector('.conn-bar')).toBeNull();
	});

	it('сервер не отвечает (7.7) — без кнопки «повторить»', () => {
		const el = render({ kind: 'down' });
		expect(el.textContent?.trim()).toBe('Сервер не отвечает — доступно сохранённое');
		expect(el.querySelector('button')).toBeNull();
	});

	it('связь вернулась (7.8): сколько уходит и общий ход', () => {
		const el = render({ kind: 'sending', posts: 2, comments: 0, progress: 0.55 });
		expect(el.textContent?.trim()).toBe('Соединение установлено — отправляем 2 записи');
		expect((el.querySelector('.conn-bar i') as HTMLElement).style.width).toBe('55%');
	});

	it('записи и комментарии называются порознь', () => {
		expect(
			render({ kind: 'sending', posts: 1, comments: 5, progress: 0 }).textContent?.trim()
		).toBe('Соединение установлено — отправляем 1 запись и 5 комментариев');
		expect(
			render({ kind: 'sending', posts: 0, comments: 1, progress: 0 }).textContent?.trim()
		).toBe('Соединение установлено — отправляем 1 комментарий');
	});
});

// Круг молчащего сервера на улочке (7.9).
describe('CircleRow silent', () => {
	it('вместо последней строки — какой сервер не отвечает, строка приглушена', () => {
		const target = document.createElement('div');
		const instance = mount(CircleRow, {
			target,
			props: {
				initial: 'П',
				name: 'Поход на Алтай',
				preview: 'Мышка теперь Мышь',
				time: 'вчера',
				silent: 'dacha.example.ru',
				onclick: () => {}
			}
		});
		flushSync();
		expect(target.querySelector('button.r.silent')).not.toBeNull();
		expect(target.querySelector('.p')?.textContent?.trim()).toBe('dacha.example.ru не отвечает');
		expect(target.textContent).not.toContain('Мышка теперь Мышь');
		unmount(instance);
	});
});
