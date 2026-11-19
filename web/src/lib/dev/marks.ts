// Временные метки замера отзывчивости (C95, план 46): нажатие → данные →
// отрисованный текст. Инструментация временная, в git не попадает — после
// сравнения «до/после» откатывается. Имена: t:* — нажатие/начало, d:* —
// данные пришли, r:* — отрисовано (кадр запрошен).
import { tick } from 'svelte';

export function mark(name: string): void {
	performance.mark(name);
}

export async function markPainted(name: string): Promise<void> {
	await tick();
	await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
	mark(name);
}
