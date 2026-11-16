import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// esbuild берём из зависимостей web — как profiles.test.mjs берёт jsdom.
const require = createRequire(new URL('../../../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const libDir = fileURLToPath(new URL('./', import.meta.url));

const bundled = await build({
	stdin: { contents: 'export * from "./display";', resolveDir: libDir },
	bundle: true,
	format: 'esm',
	write: false
});
const { gb, day, dayTime, mark, noRoom } = await import(
	'data:text/javascript;base64,' + Buffer.from(bundled.outputFiles[0].text).toString('base64')
);

test('gb keeps user-visible sizes', () => {
	assert.equal(gb(0), '1 МБ');
	assert.equal(gb(512 * 1024 * 1024), '512 МБ');
	assert.equal(gb(1536 * 1024 * 1024), '1,5 ГБ');
	assert.equal(gb(1024 ** 4), '1024,0 ГБ');
});

test('day and dayTime fall back to the raw value when the date is unreadable', () => {
	assert.equal(day('не дата'), 'не дата');
	assert.equal(dayTime('не дата'), 'не дата');
});

test('dayTime puts the time after the day', () => {
	const text = dayTime('2026-11-15T12:30:00Z');
	assert.match(text, / в \d{2}:\d{2}$/);
});

test('mark keeps check for ok and exclamation otherwise', () => {
	assert.equal(mark('ok'), '✓');
	assert.equal(mark('note'), '!');
	assert.equal(mark('block'), '!');
});

test('noRoom matches the copy plus reserve rule of frame 7б2', () => {
	const reserve = 64 * 1024 * 1024;
	const base = { state: { free_bytes: 100 * 1024 ** 3 }, copy_bytes: 1024 * 1024 };
	assert.equal(noRoom(undefined), false);
	assert.equal(noRoom({ ...base, state: { ...base.state, free_bytes: 0 } }), false);
	assert.equal(
		noRoom({ ...base, state: { ...base.state, free_bytes: base.copy_bytes + reserve - 1 } }),
		true
	);
	assert.equal(
		noRoom({ ...base, state: { ...base.state, free_bytes: base.copy_bytes + reserve } }),
		false
	);
});
