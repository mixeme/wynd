// Реальный Svelte/WebView с явно синтетической сессией и замкнутым API fixture.
// Ни один API-запрос этой проверки не уходит на сервер; публикация запрещена.
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { AndroidDevice, waitFor, sleep } from "./android-device.mjs";
const [serial, outputDirectory] = process.argv.slice(2),
  device = new AndroidDevice(serial);
const cdp = await device.connect();
const id = "88888888-8888-4888-8888-888888888888";
const draft = "Сохранённый черновик";
const out = outputDirectory ?? "dev/screens/shots/android-share";
mkdirSync(out, { recursive: true });
let backup, injected;
async function spa(target) {
  assert(target === "/share");
  await cdp.evaluate(
    "Array.from(document.querySelectorAll('button')).find(button=>button.textContent.trim()==='Отмена').click()",
  );
  await waitFor(
    () => cdp.evaluate(`location.pathname===${JSON.stringify(target)}`),
    "SPA route did not open",
  );
}
const openDB = `const db=await new Promise((resolve,reject)=>{const r=indexedDB.open('wynd');r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});const req=r=>new Promise((resolve,reject)=>{r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});`;
try {
  const initial = await cdp.evaluate(
    `new Promise((resolve,reject)=>{
    const port=window.__wyndIncomingShare,previous=port?.onmessage;
    if(!port){reject(Error('Missing incoming bridge'));return;}
    const timer=setTimeout(()=>{port.onmessage=previous;reject(Error('Incoming bridge timeout'))},5000);
    port.onmessage=event=>{const value=JSON.parse(event.data);if(value.id===2000000000){clearTimeout(timer);port.onmessage=previous;resolve(value.pending)}else previous?.(event)};
    port.postMessage(JSON.stringify({id:2000000000,op:'peek'}));
  })`,
    true,
  );
  assert(
    initial &&
      !initial.text &&
      initial.files.length === 2 &&
      initial.files.every((file) =>
        ["wynd-share-fixture.jpg", "wynd-share-fixture.webm"].includes(
          file.name,
        ),
      ),
    "UI probe accepts only the own synthetic mixed share",
  );
  backup = await cdp.evaluate(
    `(async()=>{${openDB}
    const sessions=await req(db.transaction('sessions').objectStore('sessions').getAll());
    const admins=await req(db.transaction('admin_session').objectStore('admin_session').getAll());
    if(sessions.length||admins.length)throw Error('Use only the unauthenticated dev profile');
    const stores={};for(const name of ['sessions','settings','snapshots','cursors','groups','street_ops','pins']){const store=db.transaction(name).objectStore(name);stores[name]={keys:await req(store.getAllKeys()),values:await req(db.transaction(name).objectStore(name).getAll())};}
    const storage={local:Object.fromEntries(Object.entries(localStorage)),session:Object.fromEntries(Object.entries(sessionStorage))};
    await req(db.transaction('sessions','readwrite').objectStore('sessions').put({origin:'',name:'Сервер проверки',email:'fixture@example.invalid',token:'wynd-share-fixture-only',account_id:'share-fixture'}));
    sessionStorage.setItem('wynd.compose.${id}',${JSON.stringify(draft)});db.close();return {stores,storage};})()`,
    true,
  );
  const source = `(()=>{
    const original=window.fetch.bind(window);window.__wyndFixtureRequests=[];
    const circles=[{id:'${id}',name:'Семья — проверка',color:'terracotta',status:'active',unread:0,last_read_seq:0},{id:'readonly-fixture',name:'Только чтение',color:'teal',status:'left_with_access',unread:0,last_read_seq:0},{id:'full-fixture',name:'Нет места',color:'olive',status:'active',space_full:'circle',unread:0,last_read_seq:0}];
    window.fetch=async(input,init={})=>{
      const url=new URL(typeof input==='string'?input:input.url,location.origin);
      if(url.origin!==location.origin)throw Error('Fixture forbids foreign requests');
      if(!url.pathname.startsWith('/api/'))return original(input,init);
      const method=init.method||input.method||'GET',p=url.pathname.slice('/api/v1'.length);
      __wyndFixtureRequests.push({method,path:p});
      const json=value=>new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}});
      if(method!=='GET')return new Response(JSON.stringify({error:'fixture_no_writes'}),{status:409,headers:{'Content-Type':'application/json'}});
      if(p==='/circles')return json({circles,groups:[]});
      if(p==='/instance')return json({name:'Сервер проверки',registration_mode:'invite',compression:{image_max_px:2048,image_quality:80,attachment_max_bytes:33554432}});
      if(p==='/appearance')return json({avatar_shape:'circle'});
      if(p==='/pay/status')return json({});
      if(p==='/circles/${id}')return json({identity_name:'Участник проверки',identity_id:'fixture-identity',share_place:true,has_others:false,edit_window_sec:3600});
      if(p==='/circles/${id}/members')return json({members:[]});
      if(p.startsWith('/sync'))return new Response(':fixture\\n\\n',{headers:{'Content-Type':'text/event-stream'}});
      return new Response(JSON.stringify({error:'fixture_not_found'}),{status:404,headers:{'Content-Type':'application/json'}});
    };
  })();`;
  injected = (
    await cdp.send("Page.addScriptToEvaluateOnNewDocument", { source })
  ).identifier;
  await cdp.navigate(device.origin + `/circles/${id}/compose`);
  await waitFor(
    () =>
      cdp.evaluate(
        `document.querySelector('textarea')?.value===${JSON.stringify(draft)}`,
      ),
    "Real compose did not load",
  );
  await sleep(1800);
  assert(
    await cdp.evaluate(`location.pathname==='/circles/${id}/compose'`),
    "Incoming share replaced the open editor",
  );
  await cdp.evaluate("document.activeElement?.blur()");
  device.screenshot(path.join(out, "compose-before.png"));
  // Выбор круга открывается настоящим маршрутом, без рисованного макета.
  await spa("/share");
  await waitFor(
    () =>
      cdp.evaluate(
        "document.body.textContent.includes('Файлов: 2.') && !!Array.from(document.querySelectorAll('button')).find(b=>b.textContent.includes('Семья — проверка'))",
      ),
    "Real share chooser did not load",
  );
  assert(
    await cdp.evaluate(
      "!document.body.textContent.includes('Только чтение') && !document.body.textContent.includes('Нет места')",
    ),
  );
  await cdp.evaluate("document.fonts.ready", true);
  device.screenshot(path.join(out, "choose-circle.png"));
  await cdp.evaluate(
    "Array.from(document.querySelectorAll('button')).find(b=>b.textContent.includes('Семья — проверка')).click()",
  );
  await waitFor(
    () =>
      cdp.evaluate(
        `location.pathname==='/circles/${id}/compose' && document.querySelector('textarea')?.value===${JSON.stringify(draft)} && document.querySelectorAll('.thumbs > .thumb:not(.busy)').length>=2`,
      ),
    "Shared files did not reach real compose",
  );
  await cdp.evaluate("document.activeElement?.blur()");
  await sleep(1000);
  device.screenshot(path.join(out, "compose-media.png"));
  const media = await cdp.evaluate(
    "({tiles:document.querySelectorAll('.thumbs > .thumb').length,body:document.querySelector('textarea')?.value,requests:__wyndFixtureRequests,width:innerWidth,height:innerHeight,scale:devicePixelRatio,overflow:document.documentElement.scrollWidth>innerWidth})",
  );
  assert(!media.overflow);
  assert(
    !media.requests.some((r) => /\/uploads|\/posts/.test(r.path)),
    "Import attempted an upload/publication",
  );
  device.adb(
    "shell",
    "am",
    "start",
    "-n",
    "mixeme.wynd.sharefixture/.Sender",
    "--es",
    "phase",
    "text",
  );
  await sleep(2200);
  assert(
    await cdp.evaluate(
      `location.pathname==='/circles/${id}/compose' && document.querySelector('textarea')?.value===${JSON.stringify(draft)}`,
    ),
    "Warm SEND disturbed the open editor",
  );
  await spa("/share");
  await waitFor(
    () => cdp.evaluate("document.body.textContent.includes('Получен текст.')"),
    "Text share did not appear",
  );
  await cdp.evaluate(
    "Array.from(document.querySelectorAll('button')).find(b=>b.textContent.includes('Семья — проверка')).click()",
  );
  await waitFor(
    () =>
      cdp.evaluate(
        "document.querySelector('textarea')?.value.includes('<b>без HTML</b>')",
      ),
    "Shared plain text did not reach real compose",
  );
  const text = await cdp.evaluate(
    "({body:document.querySelector('textarea').value,htmlElementPresent:Array.from(document.querySelectorAll('b')).some(b=>b.textContent==='без HTML'),requests:__wyndFixtureRequests})",
  );
  assert(text.body.startsWith(draft + "\n\n"));
  assert(!text.htmlElementPresent);
  assert(!text.requests.some((r) => /\/uploads|\/posts/.test(r.path)));
  await cdp.evaluate("document.activeElement?.blur()");
  device.screenshot(path.join(out, "compose-text.png"));
  device.adb(
    "shell",
    "am",
    "start",
    "-n",
    "mixeme.wynd.sharefixture/.Sender",
    "--es",
    "phase",
    "text",
  );
  await sleep(1800);
  await spa("/share");
  await waitFor(
    () => cdp.evaluate("document.body.textContent.includes('Получен текст.')"),
    "Cancellation chooser did not appear",
  );
  device.adb("shell", "input", "keyevent", "4");
  try {
    await waitFor(
      () => cdp.evaluate("location.pathname==='/circles'"),
      "System Back did not cancel incoming share",
    );
  } catch (error) {
    const debug = await cdp.evaluate(
      "({url:location.href,visibility:document.visibilityState,body:document.body.textContent.slice(0,1500),state:history.state})",
    );
    debug.history = await cdp.send("Page.getNavigationHistory");
    writeFileSync(
      path.join(out, "back-debug.json"),
      JSON.stringify(debug, null, 2),
    );
    device.screenshot(path.join(out, "back-failed.png"));
    throw error;
  }
  await sleep(1800);
  assert(
    await cdp.evaluate("location.pathname==='/circles'"),
    "Cancelled share reopened",
  );
  device.screenshot(path.join(out, "circles-after-cancel.png"));
  writeFileSync(
    path.join(out, "result.json"),
    JSON.stringify({ fixture: true, media, text }, null, 2),
  );
  console.log(
    JSON.stringify(
      {
        checks: [
          "real-chooser",
          "writable-circles-only",
          "photo-and-video-in-compose",
          "draft-preserved",
          "warm-send-preserves-editor",
          "plain-text-in-compose",
          "system-back-cancels-share",
          "no-uploads-or-publication",
          "no-horizontal-overflow",
        ],
        runtime: {
          width: media.width,
          height: media.height,
          scale: media.scale,
        },
      },
      null,
      2,
    ),
  );
} finally {
  try {
    if (injected)
      await cdp.send("Page.removeScriptToEvaluateOnNewDocument", {
        identifier: injected,
      });
    if (backup)
      await cdp.evaluate(
        `(async()=>{${openDB}const backup=${JSON.stringify(backup)};
      for(const [name,data]of Object.entries(backup.stores)){const tx=db.transaction(name,'readwrite'),store=tx.objectStore(name);store.clear();data.values.forEach((value,i)=>{if(store.keyPath)store.put(value);else store.put(value,data.keys[i])});await new Promise((resolve,reject)=>{tx.oncomplete=resolve;tx.onerror=()=>reject(tx.error)});}
      for(const [name,values]of Object.entries(backup.storage)){const storage=name==='local'?localStorage:sessionStorage;storage.clear();for(const [key,value]of Object.entries(values))storage.setItem(key,value);}db.close();})()`,
        true,
      );
    if (backup) await cdp.navigate(device.origin + "/");
  } finally {
    cdp.close();
  }
}
