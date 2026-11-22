// Арифметика догрузки начала журнала сборки (C71) — отдельно от контроллера,
// чтобы держать охраняющий тест: последний шаг спуска берёт остаток [0, start),
// а не полные 256 КБ от нуля — иначе показанный журнал дублирует свой хвост
// (живой прогон на wynd2, 2026-11-22: перекрытие 198 663 байт).

// logChunkBytes — порция полного вывода сборки при догрузке начала (C71).
export const logChunkBytes = 256 << 10;

// nextLogChunk — какой порцией окно достраивает начало до показанного хвоста.
// start — байт, с которого начинается уже показанное (−1: размер ещё не
// узнавали — первый запрос только за размером).
export function nextLogChunk(start: number): { offset: number; limit: number } {
	if (start < 0) {
		return { offset: 0, limit: 0 };
	}
	const want = Math.min(logChunkBytes, start);
	return { offset: start - want, limit: want };
}
