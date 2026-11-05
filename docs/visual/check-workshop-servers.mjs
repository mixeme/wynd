import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { checkWorkshopBaselines } from './workshop-baseline-guard.mjs';
const require=createRequire(new URL('../../web/package.json', import.meta.url));
const {JSDOM,VirtualConsole}=require('jsdom');
const dir=path.dirname(fileURLToPath(import.meta.url));
const errors=[];
const saved=[];
const vc=new VirtualConsole();
vc.on('jsdomError',error=>errors.push(error.message));
const dom=new JSDOM(fs.readFileSync(path.join(dir,'workshop-servers.fragment.html'),'utf8'),{runScripts:'dangerously',virtualConsole:vc,beforeParse(window){window.openai={setWidgetState:state=>{saved.push(state);return Promise.resolve();}};}});
const {document:doc,Event,MouseEvent}=dom.window;
const frames=JSON.parse(doc.getElementById('workshop-frame-data').textContent);
const baselineResult=checkWorkshopBaselines(frames,doc.getElementById('workshop-frame-styles').textContent,JSON.parse(fs.readFileSync(path.join(dir,'workshop-review-baselines.json'),'utf8')));
const ids=new Set(frames.map(f=>f.id));
assert.equal(ids.size,frames.length);
const stage=()=>doc.querySelector('[data-stage]');
function choose(id){const frame=frames.find(f=>f.id===id);assert.ok(frame,id);const group=doc.querySelector('[data-review-group]');group.value=frame.group;group.dispatchEvent(new Event('change',{bubbles:true}));const select=doc.querySelector('[data-review-frame]');select.value=id;select.dispatchEvent(new Event('change',{bubbles:true}));}
function click(selector){const el=stage().querySelector(selector);assert.ok(el,selector);el.dispatchEvent(new MouseEvent('click',{bubbles:true}));}
for(const frame of frames){
  choose(frame.id);
  assert.equal(doc.querySelector('[data-review-frame]').value,frame.id);
  assert.equal(stage().querySelectorAll('.win').length,1);
  assert.ok(stage().querySelector('.server-rail'));
  assert.equal(stage().querySelectorAll('.side').length,0);
  const header=stage().querySelector('.server-context');
  if(header){assert.ok(header.querySelector('.server-heading h3 + .connection-badge'),frame.id);assert.equal(header.querySelectorAll('.connection-row .connection-state,.connection-row .connection-badge').length,0);}
  assert.equal(stage().querySelectorAll('.body .foot').length,0,frame.id);
  for(const button of stage().querySelectorAll('button')){
    if(button.textContent.trim()==='Отмена'){assert.ok(button.closest('.pane-footer .footer-actions'),frame.id);assert.ok(button.classList.contains('footer-dismiss'),frame.id);}
  }
  for(const input of stage().querySelectorAll('[data-auto-profile]'))assert.equal(input.disabled,false,frame.id);
  for(const action of stage().querySelectorAll('[data-frame]'))assert.ok(ids.has(action.dataset.frame),frame.id+' -> '+action.dataset.frame);
}
for(const id of ['s0a','s0b','s0c','s1c','7а','7в','8б','9в','s6c']){
  choose(id);
  const input=stage().querySelector('[data-auto-profile="family"]');assert.ok(input,id);
  input.checked=false;input.dispatchEvent(new Event('change',{bubbles:true}));
  assert.equal(doc.querySelector('[data-review-frame]').value,id,'Toggling preference must not navigate');
  assert.equal(stage().querySelector('[data-auto-profile="family"]').disabled,false);
  choose('6');assert.equal(stage().querySelector('[data-auto-profile="family"]').checked,false);
}
choose('7а');click('[data-frame="8а"]');click('[data-frame="8б"]');
assert.equal(doc.querySelector('[data-review-frame]').value,'8б');
click('[data-server="new"]');assert.equal(doc.querySelector('[data-review-frame]').value,'s6a');
assert.ok(stage().textContent.includes('56 из 140 МБ'));
assert.equal(stage().querySelectorAll('.rail-progress').length,1);
assert.equal(stage().querySelector('[data-server="new"] .rail-progress'),null);
assert.equal(stage().querySelector('.connection-row button').disabled,false);
click('[data-server="family"]');assert.equal(doc.querySelector('[data-review-frame]').value,'8б');
for(const id of ['1','s1b','s9a2']){
  choose(id);
  const hint=stage().querySelector('[data-credential-hint]');
  const footer=stage().querySelector('.profile-form > .pane-footer');
  const fields=stage().querySelector('[data-password-fields]');
  const stableText=hint.textContent;
  assert.equal(hint.dataset.inactive,'true');assert.equal(hint.getAttribute('aria-hidden'),'true');
  assert.equal(hint.hidden,false);assert.equal(hint.closest('.pane-footer'),null);
  assert.equal(fields.hidden,false);assert.equal(fields.dataset.inactive,'true');
  assert.ok(!stage().textContent.includes('По SSH-ключу или сохранённому паролю.'));
  const auth=stage().querySelector('[name="auth"]');auth.value='password';auth.dispatchEvent(new Event('change',{bubbles:true}));
  assert.equal(stage().querySelector('[name="autoConnect"]').disabled,true);assert.equal(hint.dataset.inactive,'false');assert.equal(fields.dataset.inactive,'false');assert.equal(hint.textContent,stableText);
  assert.equal(footer.querySelector('[data-credential-hint]'),null);
  const remember=stage().querySelector('[name="remember"]');remember.checked=true;remember.dispatchEvent(new Event('change',{bubbles:true}));
  assert.equal(stage().querySelector('[name="autoConnect"]').disabled,false);assert.equal(hint.dataset.inactive,'true');assert.equal(hint.textContent,stableText);
}
choose('c71-build');
const buildFrameId=doc.querySelector('[data-review-frame]').value;
assert.equal(stage().querySelector('[data-c71-log]').hidden,true);
click('[data-c71-toggle]');
assert.equal(stage().querySelector('[data-c71-log]').hidden,false);
assert.equal(stage().querySelector('[data-c71-toggle]').getAttribute('aria-expanded'),'true');
assert.equal(doc.querySelector('[data-review-frame]').value,buildFrameId);
click('[data-c71-toggle]');
assert.equal(stage().querySelector('[data-c71-log]').hidden,true);
assert.ok(saved.length>0);
assert.ok(JSON.stringify(saved).length>0);
assert.equal(errors.length,0,errors.join('\n'));
assert.ok(Buffer.byteLength(fs.readFileSync(path.join(dir,'workshop-servers.fragment.html')))<1000000);
console.log(JSON.stringify({frames:frames.length,groups:new Set(frames.map(f=>f.group)).size,errors,baseline:baselineResult,checks:'Prototype DOM, transitions and pinned shell pass. Pending/rejected layouts remain unapproved. Pixel geometry and screenshot provenance are not verified by this check.'}));
