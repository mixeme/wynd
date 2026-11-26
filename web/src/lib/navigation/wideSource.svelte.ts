// План 76: на широком окне поиск остаётся левой колонкой, когда справа
// открыт найденный круг. Здесь живёт адрес поиска, по которому левая колонка
// восстанавливает запрос и фильтры; на телефоне источник не используется.
// Хранится в сессии: перезагрузка на круге возвращает ту же левую колонку.

export type WideSource = { kind: 'search'; url: string };

const STORE = 'wynd:wide-source';

function read(): WideSource | null {
	if (typeof sessionStorage === 'undefined') return null;
	try {
		const raw = JSON.parse(sessionStorage.getItem(STORE) ?? 'null') as WideSource | null;
		if (raw?.kind === 'search' && typeof raw.url === 'string' && raw.url) return raw;
	} catch {
		/* нет хранилища — живём в памяти */
	}
	return null;
}

let source = $state<WideSource | null>(read());

function save() {
	if (typeof sessionStorage === 'undefined') return;
	try {
		if (source) sessionStorage.setItem(STORE, JSON.stringify(source));
		else sessionStorage.removeItem(STORE);
	} catch {
		/* не страшно */
	}
}

/** Запомненный источник левой колонки, если он есть. */
export function wideSource(): WideSource | null {
	return source;
}

/** Круг открыт из поиска: левая колонка — поиск с этим адресом. */
export function setSearchSource(url: string): void {
	// Та же строка не трогает состояние: иначе эффект поиска, который её
	// записывает, будил бы каркас круга на каждом перерисове и упирался в
	// «effect_update_depth_exceeded».
	if (source?.kind === 'search' && source.url === url) return;
	source = { kind: 'search', url };
	save();
}

/** Вышли из поиска насовсем: левая колонка снова улица. */
export function clearWideSource(): void {
	if (!source) return;
	source = null;
	save();
}
