/**
 * Цвет строки состояния (meta theme-color) по экрану: внутри круга, где шапка
 * цветная, — цвет круга, иначе цвет фона темы (корневой макет). Экран со
 * своей шапкой кладёт цвет на время, пока он открыт; верхний — действует.
 * Браузер, который метку на ходу не читает (Firefox, Vivaldi на Android),
 * останется при цвете из манифеста (план 46, C23).
 */
import { untrack } from 'svelte';

const stack = $state<string[]>([]);

// Кладёт и снимает в untrack: зовут из $effect экрана, и чтение стека внутри
// push подписало бы этот эффект на сам стек — он перезапускался бы по кругу.
export function pushStatusColor(hex: string): () => void {
	untrack(() => stack.push(hex));
	return () =>
		untrack(() => {
			const i = stack.lastIndexOf(hex);
			if (i >= 0) stack.splice(i, 1);
		});
}

export function statusColorOverride(): string | undefined {
	return stack[stack.length - 1];
}
