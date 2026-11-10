// Общие Wynd UI используют навигацию SvelteKit. Оболочка — не SvelteKit:
// системная «Назад» для модальных окон работает через setBackHandler, а переходы
// между состояниями оболочки делает App.svelte без маршрутизатора.
export async function goto(
	_url: string,
	_options?: { replaceState?: boolean }
): Promise<void> {}

export function pushState(_url: string, _state: Record<string, unknown>): void {}
