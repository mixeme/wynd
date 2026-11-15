// Review only: actual Lightbox SSR over the accepted archive card fixture.
// node docs/visual/build-a26-viewer-proposal.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const require = createRequire(path.join(repo, 'web/package.json'));
const { createServer } = await import(pathToFileURL(require.resolve('vite')));
const { svelte } = await import(pathToFileURL(path.join(repo, 'web/node_modules/@sveltejs/vite-plugin-svelte/src/index.js')));
const baseDir = path.join(repo, 'docs/testing/a26-card-proposal-2026-11-15');
const out = path.join(repo, 'docs/testing/a26-viewer-proposal-2026-11-15');
fs.mkdirSync(out, { recursive: true });
const approved = fs.readFileSync(path.join(baseDir, 'proposal.html'), 'utf8');
const hashes = JSON.parse(fs.readFileSync(path.join(baseDir, 'sha256.json'), 'utf8'));
assert.equal(crypto.createHash('sha256').update(approved).digest('hex'), hashes.find(x => x.file === 'proposal.html').sha256);
// Preserve the approved card byte-for-byte apart from relative asset paths.
const asset = '../a26-card-proposal-2026-11-15/';
const base = approved.replaceAll('src="dacha.jpg"', `src="${asset}dacha.jpg"`);
const ui = fs.readFileSync(path.join(repo, 'web/src/lib/styles/ui.css'), 'utf8');
const rules = [...ui.matchAll(/([^{}]+)\{([^{}]*)\}/g)].filter(m => {
  const selector = m[1].replace(/\/\*[\s\S]*?\*\//g, '').trim();
  return selector === '.ib' || selector.startsWith('.lb');
}).map(m => m[0]).join('\n');
const server = await createServer({
  configFile: false, root: path.join(repo, 'web'),
  plugins: [
    // Router is never called during SSR; retain the real modal and Lightbox modules.
    { name: 'review-ssr-navigation',
      resolveId(id) { if (id === '$app/navigation') return '\0review-navigation'; },
      load(id) { if (id === '\0review-navigation') return 'export function goto(){throw new Error("SSR only")} export function pushState(){throw new Error("SSR only")}'; },
    },
    svelte({ configFile: false }),
  ],
  resolve: { alias: { $lib: path.join(repo, 'web/src/lib'), $ui: path.join(repo, 'web/src/lib/components') } },
  server: { middlewareMode: true, hmr: false },
});
try {
  const { render } = await server.ssrLoadModule('svelte/server');
  const { createRawSnippet } = await server.ssrLoadModule('svelte');
  const { default: Lightbox } = await server.ssrLoadModule('/src/lib/components/overlays/Lightbox.svelte');
  const { default: BylineParts } = await server.ssrLoadModule('/src/lib/components/data/BylineParts.svelte');
  const raw = html => createRawSnippet(() => ({ render: () => html }));
  const groups = [
    { author: 'Петя', date: '11 ноября 2026, 09:40 UTC', items: ['photo', 'video'] },
    { author: 'Аня', date: '10 ноября 2026, 21:05 UTC', items: Array(12).fill('photo') },
  ];
  let templates = '';
  for (const [group, data] of groups.entries()) {
    for (const [index, kind] of data.items.entries()) {
      const caption = render(BylineParts, { props: { parts: [{ text: data.author }, { text: data.date }] } }).body;
      const media = kind === 'video'
        ? `<video src="${asset}sample.mp4" poster="${asset}dacha.jpg" controls playsinline preload="metadata"></video>`
        : `<img src="${asset}dacha.jpg" alt="Дача" />`;
      const html = render(Lightbox, { props: {
        fixed: true, counter: `${index + 1} из ${data.items.length}`,
        caption: raw(`<span>${caption}<span data-size></span></span>`), media: raw(media),
        dotCount: data.items.length, dotIndex: index, onDotSelect() {},
        onclose() {}, ondownload() {},
        onprev: index > 0 ? () => {} : undefined,
        onnext: index < data.items.length - 1 ? () => {} : undefined,
      }}).body;
      templates += `<template id="viewer-${group}-${index}">${html}</template>`;
    }
  }
  // Small fixture adapter: all visible overlay HTML and styles above come from Wynd UI.
  // This is not the production archive viewer or the Svelte modal action.
  const behavior = `<script>
let viewerGroup=0, viewerIndex=0, opener=null, previousOverflow='';
const counts=[2,12];
function dismissViewer(){
  const lb=document.querySelector('.lb'); if(!lb)return;
  lb.querySelector('video')?.pause(); lb.remove();
  document.querySelector('.wrap').inert=false;
  document.body.style.overflow=previousOverflow; opener?.focus({preventScroll:true});
}
function openViewer(group,index,from){
  const old=document.querySelector('.lb');
  if(!old){opener=from||document.activeElement;previousOverflow=document.body.style.overflow;}
  old?.querySelector('video')?.pause(); old?.remove();
  viewerGroup=group;viewerIndex=index;
  document.body.append(document.querySelector('#viewer-'+group+'-'+index).content.cloneNode(true));
  const lb=document.querySelector('.lb'); const media=lb.querySelector('img,video');
  lb.setAttribute('aria-label',media.tagName==='VIDEO'?'Просмотр видео':'Просмотр фото');
  lb.querySelectorAll('.nav,.dots button').forEach(b=>b.setAttribute('aria-label',b.getAttribute('aria-label').replace('фото','вложение').replace('Фото','Вложение')));
  const size=()=>{const w=media.naturalWidth||media.videoWidth,h=media.naturalHeight||media.videoHeight;if(w&&h)lb.querySelector('[data-size]').textContent=' · '+w+' × '+h;};
  media.addEventListener('load',size);media.addEventListener('loadedmetadata',size);size();
  lb.querySelector('[aria-label="Закрыть"]').onclick=dismissViewer;
  lb.querySelector('[aria-label="Скачать"]').onclick=()=>{const a=document.createElement('a');a.href=media.src;a.download=media.tagName==='VIDEO'?'video.mp4':'photo.jpg';a.click();};
  const prev=lb.querySelector('.prev'),next=lb.querySelector('.next');
  if(prev)prev.onclick=()=>openViewer(group,index-1);
  if(next)next.onclick=()=>openViewer(group,index+1);
  lb.querySelectorAll('.dots button').forEach((b,i)=>b.onclick=()=>openViewer(group,i));
  document.querySelector('.wrap').inert=true;document.body.style.overflow='hidden';
  lb.querySelector('[aria-label="Закрыть"]').focus({preventScroll:true});
}
document.querySelectorAll('.archive-media').forEach((section,group)=>{
  section.querySelectorAll('.thumb-body').forEach((button,index)=>{button.setAttribute('aria-label',group===0&&index===1?'Открыть видео':'Открыть фото '+(index+1));button.onclick=()=>openViewer(group,index,button);});
});
document.addEventListener('keydown',e=>{
  const lb=document.querySelector('.lb');if(!lb)return;
  if(e.key==='Escape'){e.preventDefault();dismissViewer();}
  else if(e.key==='ArrowLeft'&&viewerIndex>0&&e.target.tagName!=='VIDEO'){e.preventDefault();openViewer(viewerGroup,viewerIndex-1);}
  else if(e.key==='ArrowRight'&&viewerIndex+1<counts[viewerGroup]&&e.target.tagName!=='VIDEO'){e.preventDefault();openViewer(viewerGroup,viewerIndex+1);}
  else if(e.key==='Tab'){
    const buttons=[...lb.querySelectorAll('button,video[controls]')],i=buttons.indexOf(document.activeElement);
    if(e.shiftKey&&i===0){e.preventDefault();buttons.at(-1).focus();}
    else if(!e.shiftKey&&i===buttons.length-1){e.preventDefault();buttons[0].focus();}
  }
});
</script>`;
  fs.writeFileSync(path.join(out, 'review.html'), base.replace('</head>', `<style>${rules}</style></head>`).replace('</body>', templates + behavior + '</body>'));
  fs.writeFileSync(path.join(out, 'source.json'), JSON.stringify({
    approvedCardCommit: '03ef72ce', approvedCardFrame: 'a26-card-proposal',
    approvedCardSha256: hashes.find(x=>x.file==='proposal.html').sha256,
    library: ['web/src/lib/components/overlays/Lightbox.svelte', 'web/src/lib/components/data/BylineParts.svelte', 'web/src/lib/styles/ui.css'].map(file=>({file,sha256:crypto.createHash('sha256').update(fs.readFileSync(path.join(repo,file))).digest('hex')})),
    referenceFrame: 'docs/visual/screens.html#e4-30',
  },null,2)+'\n');
  console.log(out);
} finally { await server.close(); }
