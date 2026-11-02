import fs from 'node:fs';
import { createRequire } from 'node:module';

const root = new URL('../', import.meta.url);
const require = createRequire(new URL('web/package.json', root));
const { JSDOM } = require('jsdom');
const doc = new JSDOM(fs.readFileSync(new URL('docs/visual/screens.html', root), 'utf8')).window
	.document;

// Accepted frames stay in screens.html; this gallery contains only the review queue.
// Keep section anchors stable when earlier groups leave the queue.
const groups = [
	{
		id: 'review-2',
		title: 'Перенос круга',
		note: '3.15 — круг принят новым сервером.',
		frames: ['e3-15']
	},
	{
		id: 'review-3',
		title: 'Завершение переноса',
		note: 'Подтверждение удаления круга с прежнего сервера.',
		frames: ['e6-35a']
	},
	{
		id: 'review-4',
		title: 'Оживление: файл загружается',
		note: 'Промежуточный экран загрузки с устройства.',
		frames: ['e7-29d']
	},
	{
		id: 'review-5',
		title: 'Открытая ссылка переноса',
		note: 'Страница ссылки, предназначенной для вставки на принимающем сервере.',
		frames: ['e7-20c']
	},
	{
		id: 'review-6',
		title: 'Журнал без рода',
		note: 'Образцы разных событий при неизвестном поле лица, а не последовательность одной операции.',
		frames: ['e3-16b']
	}
];

const esc = (value) =>
	String(value)
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;');
const sections = groups
	.map(({ id, title, note, frames }) => {
		const rendered = frames
			.map((frameId) => {
				const frame = doc.getElementById(frameId);
				if (!frame) throw new Error(`Missing frame: ${frameId}`);
				return frame.outerHTML;
			})
			.join('\n');
		return `<section id="${id}"><h2>${esc(title)}</h2><p class="review-note">${esc(note)}</p><div class="review-grid">${rendered}</div></section>`;
	})
	.join('\n');

const fixtures = JSON.parse(
	fs.readFileSync(new URL('docs/plans/46-a18-letters.json', root), 'utf8')
);
const letters = fixtures.letters
	.map(({ template, title, subject, body }) => {
		const needsEmail = fixtures.login_email_templates.includes(template);
		if (
			needsEmail &&
			!body.includes(`Войдите с почтой ${fixtures.recipient} — попадёте в своё имя и свои записи.`)
		) {
			throw new Error(`Missing login email: ${template}`);
		}
		if (!needsEmail && body.includes(fixtures.recipient))
			throw new Error(`Unnecessary login email: ${template}`);
		return `<article class="letter"><h3>${esc(title)}</h3><p class="letter-subject">Тема: ${esc(subject)}</p><pre>${esc(body)}</pre><p class="review-note">Предлагаемый текст ${esc(template)}. Письмо не отправлено.</p></article>`;
	})
	.join('\n');

const style = `<style>
body{padding:24px}main{max-width:1250px;margin:auto}header{padding:0 0 20px;margin-bottom:24px}
header h1{font-size:25px;color:var(--ink);letter-spacing:0;text-transform:none}
h2{font-size:20px;color:var(--ink);letter-spacing:0;text-transform:none;margin:42px 0 10px}
.review-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,390px),1fr));align-items:start;gap:32px 26px}
.review-grid>figure.s{display:flex;flex-direction:column;align-items:stretch;width:100%;max-width:390px;min-width:0;height:auto;margin:0}
/* Override the document's narrow-screen scale AND negative margin together.
   Each phone reserves its full height, including SSR layouts with fixed overlays. */
.review-grid>figure.s>.ph{position:relative;width:100%;max-width:100%;min-width:0;height:800px;min-height:800px;max-height:800px;margin:0;transform:translateZ(0);overflow:hidden;flex:0 0 800px}
.review-grid>figure.s>.num{position:static;margin:14px 0 0}
.review-grid>figure.s>b,.review-grid>figure.s>i{position:static;overflow-wrap:anywhere}
.review-note{color:var(--muted);font-size:14px;max-width:850px}nav{display:flex;flex-wrap:wrap;gap:8px 20px}
.letters{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,350px),1fr));gap:20px}
.letter{padding:20px;background:var(--card);border:1px solid var(--line);border-radius:12px;min-width:0}
.letter h3{margin:0 0 12px}.letter-subject{font-weight:600}
.letter pre{white-space:pre-wrap;overflow-wrap:anywhere;font-family:var(--ui);font-size:14px;line-height:1.6;margin:0 0 16px}
@media(max-width:450px){body{padding:12px}}
</style>`;
const symbols = [...doc.querySelectorAll('body > svg')].map((el) => el.outerHTML).join('\n');
const head = doc.head.innerHTML.replace(
	/<title>[\s\S]*?<\/title>/,
	'<title>Wynd · Осталось утвердить</title>'
);
const html = `<!doctype html><html lang="ru"><head>${head}${style}</head><body>${symbols}<main>
<header><h1>Осталось утвердить</h1><p>Только непринятые кадры и тексты писем. Кнопки в макетах не выполняют действий, письма не отправляются.</p>
<nav>${groups.map(({ id, title }) => `<a href="#${id}">${esc(title)}</a>`).join('')}<a href="#letters">Письма</a></nav></header>
${sections}<section id="letters"><h2>Письма</h2><p class="review-note">Тексты для отдельного просмотра. Правило указания почты уже принято; сами письма ещё не утверждены.</p><div class="letters">${letters}</div></section>
</main></body></html>`;

const output = new URL('dev/desktop-mockup/', root);
fs.mkdirSync(output, { recursive: true });
fs.writeFileSync(new URL('a18-review.html', output), html);
fs.writeFileSync(new URL('transition-review.html', output), html);
console.log(
	`Review ready: ${groups.flatMap((group) => group.frames).length} pending frames, ${fixtures.letters.length} letters.`
);
