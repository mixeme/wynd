// Review fixture only. Render real Wynd UI components into the archive shell.
// node docs/visual/build-a26-card-proposal.mjs <fresh-archive-index.html>
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import assert from 'node:assert/strict';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const require = createRequire(path.join(repo, 'web/package.json'));
const { createServer } = await import(pathToFileURL(require.resolve('vite')));
const { svelte } = await import(pathToFileURL(path.join(repo, 'web/node_modules/@sveltejs/vite-plugin-svelte/src/index.js')));
const out = path.join(repo, 'docs/testing/a26-card-proposal-2026-11-15');
fs.mkdirSync(out, { recursive: true });
const source = fs.readFileSync(path.resolve(process.argv[2]), 'utf8');
const postMatches = [...source.matchAll(/<article\b[\s\S]*?<\/article>/g)];
assert.ok(postMatches.length >= 2);
const shell = source.slice(0, postMatches[0].index);
const first = postMatches.slice(0, 2).map(m => m[0]);
// The same source images and post content in both frames.
for (let i = 0; i < first.length; i++) {
  first[i] = first[i].replace(/<(img|video)([^>]*?)src="media\/[^\"]+"/g,
    (_, tag, attrs) => `<${tag}${attrs}src="${tag === 'video' ? 'sample.mp4' : 'dacha.jpg'}"`);
}
fs.copyFileSync(path.join(repo, 'dev/screens/dacha.jpg'), path.join(out, 'dacha.jpg'));
fs.copyFileSync(path.join(repo, 'dev/archive-frame/sample.mp4'), path.join(out, 'sample.mp4'));
fs.copyFileSync(path.join(repo, 'web/static/icons.svg'), path.join(out, 'icons.svg'));
const ui = fs.readFileSync(path.join(repo, 'web/src/lib/styles/ui.css'), 'utf8');
const rules = [...ui.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter(m => {
  const selector = m[1].replace(/\/\*[\s\S]*?\*\//g, '').trim();
  return selector === ':where(button)' || selector === '.ic' || selector === '.vid-mark' ||
    selector === '.under' || selector.startsWith('.thumbs');
}).map(m => m[0]).join('\n');
const server = await createServer({
  configFile: false, root: path.join(repo, 'web'),
  plugins: [svelte({ configFile: false })],
  resolve: { alias: { $lib: path.join(repo, 'web/src/lib'), $ui: path.join(repo, 'web/src/lib/components') } },
  server: { middlewareMode: true, hmr: false },
});
try {
  const { render } = await server.ssrLoadModule('svelte/server');
  const { createRawSnippet } = await server.ssrLoadModule('svelte');
  const raw = html => createRawSnippet(() => ({ render: () => html }));
  const { default: MediaTile } = await server.ssrLoadModule('/src/lib/components/data/MediaTile.svelte');
  const { default: ThumbStrip } = await server.ssrLoadModule('/src/lib/components/data/ThumbStrip.svelte');
  const { default: TextButton } = await server.ssrLoadModule('/src/lib/components/forms/TextButton.svelte');
  const tile = video => render(MediaTile, { props: {
    variant: 'compose', src: 'dacha.jpg', kind: video ? 'video' : 'photo', onclick() {},
  }}).body;
  const strip = items => render(ThumbStrip, { props: { children: raw(items.join('')) } }).body;
  const more = render(TextButton, { props: { children: raw('<span>Ещё 6 фото</span>'), onclick() {} } }).body;
  const blocks = [
    `<section class="archive-media">${strip([tile(false), tile(true)])}</section>`,
    `<section class="archive-media">${strip(Array.from({length: 6}, () => tile(false)))}<div class="extra" hidden>${strip(Array.from({length: 6}, () => tile(false)))}</div><div class="more">${more}</div></section>`,
  ];
  const cards = first.map((html, i) => {
    let inserted = false;
    return html.replace(/<div class="media">[\s\S]*?<\/div>/g, () => {
      if (inserted) return '';
      inserted = true;
      return blocks[i];
    });
  });
  const end = '</div></body></html>';
  const baseline = shell + first.join('') + end;
  fs.writeFileSync(path.join(out, 'baseline.html'), baseline);
  const css = `<style>${rules}\n.archive-media .thumbs{padding:0}\n.archive-media .extra{margin-top:8px}\n.archive-media .more{margin-top:11px}\n.archive-media [hidden]{display:none}</style>`;
  const sprite = `<div hidden>${fs.readFileSync(path.join(out, 'icons.svg'), 'utf8')}</div>`;
  const behavior = `<script>document.querySelector('.more button').addEventListener('click',function(){const extra=this.closest('section').querySelector('.extra');extra.hidden=!extra.hidden;this.querySelector('span').textContent=extra.hidden?'Ещё 6 фото':'Свернуть';this.setAttribute('aria-expanded',String(!extra.hidden));});</script>`;
  fs.writeFileSync(path.join(out, 'proposal.html'), shell.replace('</head>', css + '</head>') + sprite + cards.join('') + behavior + end);
  console.log(out);
} finally {
  await server.close();
}
