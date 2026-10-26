// Снимок экрана для сверки «до и после»: где стоит каждый кусок текста и каким
// шрифтом, где поля ввода и какие у блоков рамки и фон. Разметка может
// поменяться (обёртки, компоненты) — снимок ловит только то, что видно.

export interface Item {
	k: string;
	x: number;
	y: number;
	w: number;
	h: number;
	s: string;
}

const r = (n: number) => Math.round(n * 2) / 2;

function box(el: Element | Range, o: DOMRect): [number, number, number, number] {
	const b = el.getBoundingClientRect();
	return [r(b.left - o.left), r(b.top - o.top), r(b.width), r(b.height)];
}

function visible(el: Element): boolean {
	const cs = getComputedStyle(el);
	if (cs.display === 'none' || cs.visibility === 'hidden') return false;
	const b = el.getBoundingClientRect();
	return b.width > 0 && b.height > 0;
}

export function capture(root: HTMLElement): Item[] {
	const o = root.getBoundingClientRect();
	const out: Item[] = [];
	const walk = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
	for (let n = walk.nextNode(); n; n = walk.nextNode()) {
		if (n.nodeType === Node.TEXT_NODE) {
			const t = (n.textContent ?? '').replace(/\s+/g, ' ').trim();
			const el = n.parentElement;
			if (!t || !el || !visible(el)) continue;
			const range = document.createRange();
			range.selectNodeContents(n);
			// Строки текста по отдельности: перенос в другом месте — тоже разница.
			for (const line of range.getClientRects()) {
				if (line.width < 0.5) continue;
				const cs = getComputedStyle(el);
				out.push({
					k: 'text:' + t,
					x: r(line.left - o.left),
					y: r(line.top - o.top),
					w: r(line.width),
					h: r(line.height),
					s: [cs.fontFamily, cs.fontSize, cs.fontWeight, cs.fontStyle, cs.color, cs.textDecorationLine, cs.opacity].join('|')
				});
			}
			continue;
		}
		const el = n as Element;
		if (!visible(el)) continue;
		const cs = getComputedStyle(el);
		const bg = cs.backgroundColor !== 'rgba(0, 0, 0, 0)' ? cs.backgroundColor : '';
		const bw = ['Top', 'Right', 'Bottom', 'Left']
			.map((side) => {
				const w = cs.getPropertyValue(`border-${side.toLowerCase()}-width`);
				const st = cs.getPropertyValue(`border-${side.toLowerCase()}-style`);
				return st === 'none' || w === '0px' ? '' : `${w} ${st} ${cs.getPropertyValue(`border-${side.toLowerCase()}-color`)}`;
			})
			.join(',');
		const shadow = cs.boxShadow !== 'none' ? cs.boxShadow : '';
		const field = el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement;
		if (!bg && bw === ',,,' && !shadow && !field && el.tagName !== 'svg') continue;
		const [x, y, w, h] = box(el, o);
		const val = field ? `${(el as HTMLInputElement).type}=${(el as HTMLInputElement).value}|${(el as HTMLInputElement).placeholder}|${(el as HTMLInputElement).checked}` : '';
		out.push({ k: 'box:' + el.tagName.toLowerCase(), x, y, w, h, s: [bg, bw, cs.borderRadius, shadow, val].join('|') });
	}
	return out;
}

export function hash(items: Item[]): string {
	let h = 0x811c9dc5;
	for (const ch of JSON.stringify(items)) {
		h ^= ch.charCodeAt(0);
		h = Math.imul(h, 0x01000193);
	}
	return (h >>> 0).toString(16).padStart(8, '0');
}

/** Разница двух снимков: что пропало, что появилось (по ключу и месту). */
export function diff(before: Item[], after: Item[]): string[] {
	const key = (i: Item) => JSON.stringify(i);
	const a = new Map<string, number>();
	for (const i of after) a.set(key(i), (a.get(key(i)) ?? 0) + 1);
	const b = new Map<string, number>();
	for (const i of before) b.set(key(i), (b.get(key(i)) ?? 0) + 1);
	const lines: string[] = [];
	for (const [k, n] of b) if ((a.get(k) ?? 0) < n) lines.push('- ' + k);
	for (const [k, n] of a) if ((b.get(k) ?? 0) < n) lines.push('+ ' + k);
	return lines;
}
