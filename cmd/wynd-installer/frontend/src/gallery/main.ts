// Стенд окна Мастерской (gallery.html, только `vite` — в сборку не входит):
// окно с подставным Go, проведённое щелчками до каждого экрана.
//   ?scene=7a-server   — одна сцена, как живое окно;
//   ?run               — все сцены: снимок каждой, сумма и сверка с сохранённым;
//   ?run&save          — то же и запомнить снимки как эталон (localStorage).
import { mount, tick, unmount } from 'svelte';
import '$lib/styles/tokens.css';
import '$lib/styles/ui.css';
import '$lib/styles/workshop.css';
import '../app.css';
import Window from '../Window.svelte';
import { capture, diff, hash, type Item } from './capture';
import { makeBackend } from './fake';
import { type Act, SCENES, type Scene } from './scenes';

const KEY = 'wynd-workshop-gallery';
const frame = document.getElementById('frame') as HTMLElement;
const report = document.getElementById('report') as HTMLElement;

const settle = async () => {
	for (let i = 0; i < 4; i++) {
		await tick();
		await new Promise((r) => setTimeout(r, 0));
	}
};

function find(text: string, nth = 0): HTMLElement {
	const want = text.replace(/\s+/g, ' ').trim();
	const hits = [...frame.querySelectorAll<HTMLElement>('button, label, [role="tab"]')].filter(
		(e) => (e.textContent ?? '').replace(/\s+/g, ' ').trim() === want
	);
	if (!hits[nth]) throw new Error(`нет «${text}» (#${nth})`);
	return hits[nth];
}

async function act(a: Act) {
	if ('tab' in a) find('Архивы').click();
	else if ('click' in a) find(a.click, a.nth).click();
	else {
		const input = frame.querySelector<HTMLInputElement>(`input[placeholder="${a.fill}"]`);
		if (!input) throw new Error(`нет поля «${a.fill}»`);
		input.value = a.value;
		input.dispatchEvent(new Event('input', { bubbles: true }));
	}
	await settle();
}

let current: ReturnType<typeof mount> | undefined;

async function show(scene: Scene) {
	if (current) await unmount(current);
	frame.innerHTML = '';
	(window as unknown as { go: unknown }).go = { main: { App: makeBackend(scene.fake ?? {}) } };
	current = mount(Window, { target: frame });
	await settle();
	for (const a of scene.acts ?? []) await act(a);
}

const params = new URLSearchParams(location.search);

if (params.has('run')) {
	const saved: Record<string, Item[]> = (() => {
		try {
			return JSON.parse(localStorage.getItem(KEY) ?? '{}');
		} catch {
			return {};
		}
	})();
	const now: Record<string, Item[]> = {};
	const lines: string[] = [];
	for (const scene of SCENES) {
		try {
			await show(scene);
			now[scene.name] = capture(frame);
			const was = saved[scene.name];
			const d = was ? diff(was, now[scene.name]) : [];
			lines.push(`${scene.name} ${hash(now[scene.name])} «${frame.querySelector('h2')?.textContent?.trim() ?? ''}» ${!was ? 'нет эталона' : d.length ? `РАЗНИЦА ${d.length}` : 'как эталон'}`);
			lines.push(...d.slice(0, 40).map((l) => '    ' + l));
		} catch (e) {
			lines.push(`${scene.name} ОШИБКА ${(e as Error).message}`);
		}
	}
	if (params.has('save')) {
		try {
			localStorage.setItem(KEY, JSON.stringify(now));
			lines.push('эталон сохранён');
		} catch (e) {
			lines.push('эталон не сохранён: ' + (e as Error).message);
		}
	}
	report.textContent = lines.join('\n');
	(window as unknown as { galleryDone: string; gallerySnap: unknown }).galleryDone = lines.join('\n');
	(window as unknown as { gallerySnap: unknown }).gallerySnap = now;
} else {
	const name = params.get('scene') ?? SCENES[0].name;
	const scene = SCENES.find((s) => s.name === name) ?? SCENES[0];
	report.innerHTML = SCENES.map((s) => `<a href="?scene=${s.name}"${s === scene ? ' class="on"' : ''}>${s.name}</a>`).join(' ');
	await show(scene);
}
