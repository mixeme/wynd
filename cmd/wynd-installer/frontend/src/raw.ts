// Ответы сервера на команды осмотра для окна «Что проверили» (кадр 2е):
// разделы, из-за которых отказ, идут первыми и помечены, остальное — как пришло.

export interface RawSection {
	name: string;
	body: string;
	/** Из-за этого раздела осмотр отказал. */
	cause: boolean;
}

/** Какие разделы осмотра стоят за находкой (internal/installer/inspect.go). */
const SECTIONS_OF: Record<string, string[]> = {
	os: ['os', 'arch'],
	root: ['uid', 'sudo'],
	disk: ['disk'],
	flush: ['flush'],
	clock: ['time', 'ntp'],
	docker: ['docker', 'compose'],
	ports: ['ports'],
	proxy: ['nginx', 'apache', 'caddy']
};

export function rawSections(raw: string, blockIds: string[]): RawSection[] {
	const causes = new Set(blockIds.flatMap((id) => SECTIONS_OF[id] ?? [id]));
	const sections: RawSection[] = [];
	for (const line of raw.split('\n')) {
		if (line.startsWith('## ')) {
			sections.push({ name: line.slice(3).trim(), body: '', cause: false });
		} else if (sections.length) {
			const last = sections[sections.length - 1];
			last.body += (last.body ? '\n' : '') + line;
		}
	}
	for (const s of sections) {
		s.body = s.body.trimEnd();
		// Пустой раздел причиной не бывает: у «proxy» из трёх веб-серверов стоит один.
		s.cause = causes.has(s.name) && s.body !== '';
	}
	return [...sections.filter((s) => s.cause), ...sections.filter((s) => !s.cause)];
}
