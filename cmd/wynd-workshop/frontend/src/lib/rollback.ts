// Тексты итога отката (кадры 2б, 5а, 5д, 5е, 5е2): чистая сборка строк из
// результата Rollback. Тот же текст, что был в App.svelte до выноса (план 60).
import type { RollbackResult } from '../api';

/** Данные удаляли — сертификат отзывали: сказать, вышло ли. */
export function certNote(cert: 'revoked' | 'failed' | undefined): string {
	if (cert === 'revoked') return ' Сертификат сайта отозван.';
	if (cert === 'failed') {
		return ' Отозвать сертификат сайта не получилось: он удалён вместе с ключом и перестанет действовать сам, когда выйдет его срок.';
	}
	return '';
}

/** Окно удаления без галочки: что останется и чем это грозит. Каталог —
 * фактический (C130), дата — из осмотра; без неё говорим «и истечёт». */
export function removeWarning(input: {
	root: string;
	proxyName: string;
	foreignSite: string[];
	site: string;
	expiry: string;
}): string {
	if (input.foreignSite.length) {
		return `Без галочки папка ${input.root} останется.`;
	}
	if (input.proxyName) {
		const when = input.expiry ? `— он истечёт ${input.expiry}.` : 'и истечёт.';
		return `Без галочки папка ${input.root} и сертификат останутся. Но сайт уйдёт из ${input.proxyName}: домен перестанет открываться, а сертификат перестанет продлеваться ${when}`;
	}
	return `Без галочки папка ${input.root} останется как есть.`;
}

/** Кадр 5д: что именно помешало, знаем по проверке до удаления и после него. */
export function staleNote(proxyName: string, stale: RollbackResult['stale']): string {
	if (!stale || !proxyName) return '';
	if (stale === 'before') {
		return ` Но ${proxyName} это изменение ещё не принял: в его настройке ошибка, и она была там ещё до удаления — о ней ниже. Когда её исправят, сайт Wynd пропадёт из ${proxyName} сам.`;
	}
	if (stale === 'after') {
		return ` Но ${proxyName} это изменение не принял: без файла сайта Wynd его настройка перестала проходить проверку — похоже, на этот файл ссылается чужая настройка. Что не так, сказано ниже.`;
	}
	return ` Но перечитать настройку ${proxyName} не удалось, хотя ошибок в ней нет. Сайт Wynd пропадёт из ${proxyName}, когда его попросят перечитать настройку или перезапустят.`;
}

/** Итог отката на экране плана (не прошло) или осмотра («новый сервер»). */
export function removalNote(input: {
	dropData: boolean;
	/** Откат неудавшейся установки: назад к плану, без слов про чужие файлы. */
	failed: boolean;
	proxyName: string;
	foreignSite: string[];
	cert: RollbackResult['cert'];
	stale: RollbackResult['stale'];
}): string {
	const stays = input.proxyName ? `Docker и ${input.proxyName} остались.` : 'Docker остался.';
	const foreign = input.foreignSite.length;
	const leftFiles = foreign
		? ` Файл сайта ${input.foreignSite.join(', ')} остался — его ставили не мы.`
		: '';
	if (input.failed) {
		return (
			(input.dropData
				? 'Установка откачена: Wynd и его данные с сервера убраны. '
				: 'Установка откачена: Wynd с сервера убран, его данные оставлены. ') +
			stays +
			certNote(input.cert) +
			staleNote(input.proxyName, input.stale)
		);
	}
	// Убрали работавший Wynd (окна 2б, 5а): назад к осмотру, сервер уже без него.
	// Сертификат у чужой установки получали не мы — о нём ни слова (5е2).
	return (
		(input.dropData
			? foreign
				? 'Wynd и его данные с сервера убраны. '
				: 'Wynd, его данные и сертификат с сервера убраны. '
			: foreign
				? 'Wynd с сервера убран, его данные оставлены. '
				: 'Wynd с сервера убран, его данные и сертификат оставлены. ') +
		stays +
		leftFiles +
		certNote(input.cert) +
		staleNote(input.proxyName, input.stale)
	);
}
