import { describe, expect, it } from 'vitest';
import { applyOwnReaction, canReact } from './reactions';
import type { Reaction } from './types';

const actor = { identityId: 'me', identityName: 'Аня' };

function reaction(over: Partial<Reaction> = {}): Reaction {
	return {
		id: 'r1',
		post_id: 'p1',
		emoji: 'heart',
		author_name: 'Боб',
		identity_id: 'bob',
		created_at: '2026-08-30T10:00:00Z',
		...over
	};
}

describe('своя реакция', () => {
	it('ставится, когда её ещё нет', () => {
		const out = applyOwnReaction([reaction()], 'p1', 'laugh', actor, new Date('2026-09-01T00:00:00Z'));
		expect(out).toHaveLength(2);
		expect(out[1]).toMatchObject({ emoji: 'laugh', identity_id: 'me', author_name: 'Аня', post_id: 'p1' });
	});

	it('заменяет свою, а не добавляет вторую', () => {
		const mine = reaction({ id: 'r2', identity_id: 'me', emoji: 'heart' });
		const out = applyOwnReaction([reaction(), mine], 'p1', 'anger', actor);
		expect(out).toHaveLength(2);
		expect(out.filter((r) => r.identity_id === 'me')).toHaveLength(1);
		expect(out.find((r) => r.identity_id === 'me')?.emoji).toBe('anger');
	});

	it('снимается по null и чужие не трогает', () => {
		const mine = reaction({ id: 'r2', identity_id: 'me' });
		const out = applyOwnReaction([reaction(), mine], 'p1', null, actor);
		expect(out).toHaveLength(1);
		expect(out[0].identity_id).toBe('bob');
	});

	it('снимать нечего — список прежний', () => {
		const out = applyOwnReaction([reaction()], 'p1', null, actor);
		expect(out).toHaveLength(1);
	});
});

describe('можно ли реагировать', () => {
	const opts = { canWrite: true, solo: false, locked: false };

	it('читателю, соло-кругу и запертой записи — нет', () => {
		expect(canReact([], actor, { ...opts, canWrite: false })).toBe(false);
		expect(canReact([], actor, { ...opts, solo: true })).toBe(false);
		expect(canReact([], actor, { ...opts, locked: true })).toBe(false);
	});

	it('своей реакции ещё нет — да', () => {
		expect(canReact([reaction()], actor, opts)).toBe(true);
	});

	it('своя реакция вне окна правок — нет', () => {
		const mine = reaction({ identity_id: 'me', editable_until: '2000-01-01T00:00:00Z' });
		expect(canReact([mine], actor, opts)).toBe(false);
	});

	it('своя реакция в окне правок — да', () => {
		const mine = reaction({ identity_id: 'me', editable_until: '2099-01-01T00:00:00Z' });
		expect(canReact([mine], actor, opts)).toBe(true);
	});
});
