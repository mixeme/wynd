import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// esbuild берём из зависимостей web — как display.test.mjs.
const require = createRequire(new URL('../../../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const libDir = fileURLToPath(new URL('./', import.meta.url));

const bundled = await build({
	stdin: { contents: 'export * from "./buildlog";', resolveDir: libDir },
	bundle: true,
	format: 'esm',
	write: false
});
const { logChunkBytes, nextLogChunk } = await import(
	'data:text/javascript;base64,' + Buffer.from(bundled.outputFiles[0].text).toString('base64')
);

test('первый запрос — только размер журнала', () => {
	assert.deepEqual(nextLogChunk(-1), { offset: 0, limit: 0 });
});

test('шаги спуска укладываются в журнал без перекрытий', () => {
	// Полный спуск для журнала из живого прогона C71 (wynd2, 2026-11-22):
	// total = 2 113 510, хвост потока 256 КБ → start = 1 851 366.
	const total = 2113510;
	let start = total - logChunkBytes;
	const ranges = [];
	while (start > 0) {
		const { offset, limit } = nextLogChunk(start);
		ranges.push([offset, offset + limit]);
		start = offset;
	}
	// Диапазоны идут вплотную и не пересекаются: конец каждого — начало
	// предыдущего; последний закрывается ровно на нуле.
	for (let i = 1; i < ranges.length; i++) {
		assert.equal(ranges[i][1], ranges[i - 1][0], `шаг ${i} перекрывает предыдущий`);
	}
	assert.equal(ranges[ranges.length - 1][0], 0);
	// Вместе с хвостом покрывают файл целиком.
	assert.equal(ranges[0][1], total - logChunkBytes);
});

test('последний шаг берёт остаток, а не полные 256 КБ от нуля', () => {
	// Дефект из живого прогона: при start = 63 481 прежний код просил
	// [0, 262 144) и дублировал 198 663 байт уже собранного начала.
	assert.deepEqual(nextLogChunk(63481), { offset: 0, limit: 63481 });
});

test('выровненный на порцию start даёт полный шаг без перекрытия', () => {
	assert.deepEqual(nextLogChunk(262144), { offset: 0, limit: 262144 });
});

test('малый журнал: порция не выходит за начало показанного', () => {
	// Хвост меньше порции (сборка выдала меньше 256 КБ): start мал.
	assert.deepEqual(nextLogChunk(1200), { offset: 0, limit: 1200 });
});
