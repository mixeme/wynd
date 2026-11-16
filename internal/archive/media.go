package archive

// Медиа записи в читаемом архиве (A26): плитки 88 × 88 как в приложении,
// первые шесть видны, остальные — кнопкой «Ещё N» на месте; просмотр поверх
// страницы — штатный тёмный .lb приложения. Вёрстка принята владельцем
// 2026-11-16 (кадры a26-card-proposal и a26-viewer-proposal), подробности —
// docs/testing/a26-card-proposal-2026-11-15 и a26-viewer-proposal-2026-11-15.
// Без JS архив остаётся читаемым: плитка — ссылка на сам файл, а спрятанные
// плитки раскрывает <noscript>; просмотрщик и «Ещё N» — улучшение поверх.

import (
	"fmt"
	"html"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// feedTilesVisible — сколько плиток видно сразу (принятый кадр: шесть).
const feedTilesVisible = 6

// mediaTile — фото или ролик записи в сетке превью.
type mediaTile struct {
	src string
	// poster — обложка ролика (кадр, который приложение сняло при загрузке);
	// у фото пусто — плитка показывает сам снимок.
	poster string
	video  bool
}

// appendMediaSectionHTML пишет секцию медиа записи: все фото и ролики
// записи одной сеткой (порядок записи), первые шесть видны, остальные —
// за «Ещё N». Файлы и звук в сетку не входят — их рисует вызывающий.
func appendMediaSectionHTML(b *strings.Builder, fp chronicle.FeedPost, media map[string]string, prefix string) {
	var tiles []mediaTile
	missing := 0
	for _, m := range fp.Media {
		if playableAudio(m) || m.Kind == chronicle.MediaAttachment {
			continue
		}
		if m.Kind != chronicle.MediaPhoto && m.Kind != chronicle.MediaVideo {
			continue
		}
		src, ok := media[m.BlobID]
		if !ok {
			missing++
			continue
		}
		t := mediaTile{src: prefix + src, video: m.Kind == chronicle.MediaVideo}
		if t.video {
			if p, ok := media[m.VideoPosterBlobID]; ok {
				t.poster = prefix + p
			}
		}
		tiles = append(tiles, t)
	}
	if len(tiles) == 0 {
		for i := 0; i < missing; i++ {
			b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
		}
		return
	}
	cap := captionText(fp)
	visible, extra := tiles, []mediaTile(nil)
	if len(tiles) > feedTilesVisible {
		visible, extra = tiles[:feedTilesVisible], tiles[feedTilesVisible:]
	}
	b.WriteString("<section class=\"archive-media\"><div class=\"thumbs\">")
	for i, t := range visible {
		appendMediaTileHTML(b, i+1, t, cap)
	}
	b.WriteString("</div>")
	if len(extra) > 0 {
		b.WriteString("<div class=\"extra\" hidden><div class=\"thumbs\">")
		for i, t := range extra {
			appendMediaTileHTML(b, feedTilesVisible+i+1, t, cap)
		}
		label := html.EscapeString(moreLabel(extra))
		b.WriteString("</div></div><div class=\"more\"><button type=\"button\" class=\"under\" aria-expanded=\"false\" data-more=\"")
		b.WriteString(label)
		b.WriteString("\"><span>")
		b.WriteString(label)
		b.WriteString("</span></button></div>")
		// Без JS «Ещё N» не нажать — тогда прятать нечего.
		b.WriteString("<noscript><style>.archive-media .extra{display:block!important}</style></noscript>")
	}
	b.WriteString("</section>")
	for i := 0; i < missing; i++ {
		b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
	}
}

// appendMediaTileHTML — одна плитка: ссылка на файл (работает без JS),
// с данными для просмотрщика поверх.
func appendMediaTileHTML(b *strings.Builder, n int, t mediaTile, cap string) {
	src := html.EscapeString(t.src)
	kind, label := "photo", "Открыть фото "+fmt.Sprint(n)
	if t.video {
		kind, label = "video", "Открыть видео "+fmt.Sprint(n)
	}
	b.WriteString("<div class=\"thumb\"><a class=\"thumb-body\" href=\"")
	b.WriteString(src)
	b.WriteString("\" data-src=\"")
	b.WriteString(src)
	b.WriteString("\" data-kind=\"")
	b.WriteString(kind)
	b.WriteString("\" data-cap=\"")
	b.WriteString(html.EscapeString(cap))
	if t.poster != "" {
		b.WriteString("\" data-poster=\"")
		b.WriteString(html.EscapeString(t.poster))
	}
	b.WriteString("\" aria-label=\"")
	b.WriteString(label)
	b.WriteString("\">")
	if t.video {
		if t.poster != "" {
			// Плитка ролика — кадр обложки со значком, как в приложении
			// и в принятом кадре (2026-11-16).
			b.WriteString("<img alt=\"\" src=\"")
			b.WriteString(html.EscapeString(t.poster))
			b.WriteString("\">")
		} else {
			// Постера нет — браузер покажет первый кадр ролика.
			b.WriteString("<video src=\"")
			b.WriteString(src)
			b.WriteString("#t=0.1\" playsinline muted preload=\"metadata\"></video>")
		}
		b.WriteString(`<svg class="ic vid-mark" viewBox="0 0 24 24" aria-hidden="true"><path d="M7.5 4.5 19.5 12l-12 7.5z"/></svg>`)
	} else {
		b.WriteString("<img alt=\"\" src=\"")
		b.WriteString(src)
		b.WriteString("\">")
	}
	b.WriteString("</a></div>")
}

// moreLabel — текст кнопки раскрытия: «Ещё 6 фото», «Ещё 3 видео»,
// «Ещё 5 вложений» (смешанные).
func moreLabel(extra []mediaTile) string {
	photos, videos := 0, 0
	for _, t := range extra {
		if t.video {
			videos++
		} else {
			photos++
		}
	}
	switch {
	case videos == 0:
		return "Ещё " + fmt.Sprint(len(extra)) + " фото"
	case photos == 0:
		return "Ещё " + fmt.Sprint(len(extra)) + " видео"
	default:
		return "Ещё " + plural(len(extra), "вложение", "вложения", "вложений")
	}
}

// plural — русские формы числительного: 1 вложение, 2 вложения, 5 вложений.
func plural(n int, one, few, many string) string {
	mod10, mod100 := n%10, n%100
	switch {
	case mod10 == 1 && mod100 != 11:
		return fmt.Sprint(n) + " " + one
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
		return fmt.Sprint(n) + " " + few
	default:
		return fmt.Sprint(n) + " " + many
	}
}

// captionText — подпись под медиа: автор и абсолютные дата/время UTC
// (принятый кадр: «Аня · 10 ноября 2026, 21:05 UTC»).
func captionText(fp chronicle.FeedPost) string {
	if fp.Post.CreatedAt.IsZero() {
		return fp.Post.AuthorName
	}
	t := fp.Post.CreatedAt.UTC()
	return fmt.Sprintf("%s · %d %s %d, %02d:%02d UTC", fp.Post.AuthorName,
		t.Day(), monthsGen[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// mediaCSS — сетка превью и просмотр поверх. Значения плиток и .lb взяты
// из web/src/lib/styles/ui.css (тест паритета); внутри карточки архива у
// .thumbs снят отступ 16 px — карточка уже имеет свои поля (принятый кадр).
const mediaCSS = `
:where(button){font:inherit;color:inherit;background:none;border:0;padding:0;margin:0;text-align:inherit;cursor:pointer;-webkit-tap-highlight-color:transparent;appearance:none}
.thumbs{display:flex;flex-wrap:wrap;gap:8px;padding:16px 16px 0;align-items:flex-start}
.thumbs .thumb{width:88px;height:88px;border-radius:8px;overflow:hidden;position:relative;flex:0 0 auto;background:var(--tint)}
.thumbs .thumb .thumb-body,.thumbs .thumb-body{display:block;width:100%;height:100%;border:0;background:transparent}
.thumbs .thumb img,.thumbs .thumb video,.thumbs .thumb-body img,.thumbs .thumb-body video{width:100%;height:100%;object-fit:cover;display:block}
.vid-mark{position:absolute;left:50%;top:50%;width:26px;height:26px;transform:translate(-50%,-50%);color:rgba(255,255,255,0.9);fill:currentColor;filter:drop-shadow(0 0 3px rgba(0,0,0,0.45));pointer-events:none}
.under{border-bottom:1px solid;padding-bottom:1px}
.archive-media{margin:11px 0 0}
.archive-media .thumbs{padding:0}
.archive-media .extra{margin-top:8px}
.archive-media .more{margin-top:11px}
.archive-media [hidden]{display:none}
.lb{position:absolute;inset:0;background:#171513;display:flex;flex-direction:column;color:#efe9e0}
.lb.fixed{position:fixed;inset:0;z-index:50}
.lb .top{display:flex;align-items:center;gap:14px;padding:10px 16px}
.lb .ic{color:#efe9e0}
.lb .mid{flex:1;min-height:0;min-width:0;position:relative;display:grid;place-items:center;touch-action:pan-y;overflow:hidden}
.lb .mid img,.lb .mid video{display:block;min-width:0;min-height:0;width:100%;height:100%;object-fit:contain}
.lb .nav{position:absolute;top:0;bottom:0;width:64px;display:flex;align-items:center;color:rgba(239,233,224,0.55);cursor:pointer;border:none;background:none;padding:0;font:inherit}
.lb .nav:hover{color:#efe9e0}
.lb .nav.prev{left:0;justify-content:flex-start;padding-left:4px}
.lb .nav.next{right:0;justify-content:flex-end;padding-right:4px}
.lb .cap{padding:14px 16px 22px;font-size:12.5px;color:#b9b1a8}
.lb .dots{display:flex;gap:5px;justify-content:center;padding-bottom:14px}
.lb .dots u,.lb .dots button{width:5px;height:5px;border-radius:50%;background:#5a534c;border:0;padding:0;cursor:pointer}
.lb .dots u.on,.lb .dots .on{background:#efe9e0}
.ib{line-height:0}
`

// viewerScript — просмотр поверх страницы: одна оболочка .lb на страницу,
// листание внутри записи, видео с нативным управлением без автозапуска,
// остановка ролика при смене и закрытии, возврат фокуса к исходной плитке.
// Без JS плитки остаются ссылками на файлы.
const viewerScript = `<script>(function(){
var wrap=document.querySelector('.wrap'),opener=null,prevOverflow='',gi=0,ii=0;
var icons={
x:'<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M5.5 5.5 18.5 18.5M18.5 5.5 5.5 18.5"/></svg>',
down:'<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4v10.6M6.2 11.3 12 17l5.8-5.7"/><path d="M4.5 19.8h15"/></svg>',
chev:'<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M8.5 5.5 15 12l-6.5 6.5"/></svg>'
};
function sections(){return [].slice.call(document.querySelectorAll('.archive-media'))}
function tilesOf(g){var s=sections()[g];return s?[].slice.call(s.querySelectorAll('.thumb-body')):[]}
function pauseVideo(lb){var v=lb&&lb.querySelector('video');if(v)v.pause()}
function dismiss(){
var lb=document.querySelector('.lb');if(!lb)return;
pauseVideo(lb);lb.remove();
if(wrap)wrap.inert=false;
document.body.style.overflow=prevOverflow;
if(opener)opener.focus({preventScroll:true});
}
function open(g,i,from){
var old=document.querySelector('.lb');
if(old){pauseVideo(old);old.remove()}else{opener=from||document.activeElement;prevOverflow=document.body.style.overflow}
var tiles=tilesOf(g);if(!tiles.length)return;
gi=g;ii=i;
var t=tiles[i],kind=t.getAttribute('data-kind'),src=t.getAttribute('data-src'),cap=t.getAttribute('data-cap');
var lb=document.createElement('div');lb.className='lb fixed';lb.setAttribute('role','dialog');lb.setAttribute('aria-modal','true');
lb.setAttribute('aria-label',kind==='video'?'Просмотр видео':'Просмотр фото');
var top=document.createElement('div');top.className='top';top.innerHTML='<button type="button" class="ib" aria-label="Закрыть">'+icons.x+'</button><span style="font-size:13.5px">'+(i+1)+' из '+tiles.length+'</span><button type="button" class="ib" style="margin-left:auto;color:#EFE9E0" aria-label="Скачать">'+icons.down+'</button>';
var mid=document.createElement('div');mid.className='mid';mid.setAttribute('role','region');mid.setAttribute('aria-label','Просмотр');
function nav(k,back){var b=document.createElement('button');b.type='button';b.className='nav '+k;b.setAttribute('aria-label',back?'Предыдущее вложение':'Следующее вложение');b.innerHTML=(back?'<svg class="ic" viewBox="0 0 24 24" style="transform:rotate(180deg)" aria-hidden="true"><path d="M8.5 5.5 15 12l-6.5 6.5"/></svg>':icons.chev);b.onclick=function(){open(gi,ii+(back?-1:1))};return b}
if(i>0)mid.appendChild(nav('prev',true));
var media;
if(kind==='video'){media=document.createElement('video');media.src=src;media.controls=true;media.playsInline=true;media.preload='metadata';var po=t.getAttribute('data-poster');if(po)media.poster=po}
else{media=document.createElement('img');media.src=src;media.alt=''}
mid.appendChild(media);
if(i+1<tiles.length)mid.appendChild(nav('next',false));
lb.appendChild(top);lb.appendChild(mid);
if(tiles.length>1){
var dots=document.createElement('div');dots.className='dots';
for(var k=0;k<tiles.length;k++){
var d=document.createElement('button');d.type='button';d.setAttribute('aria-label','Вложение '+(k+1));
if(k===i)d.className='on';
d.onclick=(function(k){return function(){open(gi,k)}})(k);
dots.appendChild(d);
}
lb.appendChild(dots);
}
var capEl=document.createElement('div');capEl.className='cap';
var capSpan=document.createElement('span');capSpan.textContent=cap||'';
var sizeEl=document.createElement('span');sizeEl.setAttribute('data-size','');
capSpan.appendChild(sizeEl);capEl.appendChild(capSpan);lb.appendChild(capEl);
function size(){var w=media.naturalWidth||media.videoWidth,h=media.naturalHeight||media.videoHeight;if(w&&h)sizeEl.textContent=' · '+w+' × '+h}
media.addEventListener('load',size);media.addEventListener('loadedmetadata',size);size();
lb.querySelector('[aria-label="Закрыть"]').onclick=dismiss;
lb.querySelector('[aria-label="Скачать"]').onclick=function(){
var a=document.createElement('a');a.href=src;a.download=src.split('/').pop()||'media';a.click();
};
document.body.appendChild(lb);
if(wrap)wrap.inert=true;
document.body.style.overflow='hidden';
lb.querySelector('[aria-label="Закрыть"]').focus({preventScroll:true});
}
document.querySelectorAll('.archive-media').forEach(function(section,g){
section.querySelectorAll('.thumb-body').forEach(function(tile,i){
tile.addEventListener('click',function(ev){ev.preventDefault();open(g,i,tile)});
});
});
document.querySelectorAll('.archive-media .more button').forEach(function(button){
button.addEventListener('click',function(){
var extra=button.closest('section').querySelector('.extra');
extra.hidden=!extra.hidden;
button.querySelector('span').textContent=extra.hidden?(button.getAttribute('data-more')||'Ещё'):'Свернуть';
button.setAttribute('aria-expanded',String(!extra.hidden));
});
});
document.addEventListener('keydown',function(e){
var lb=document.querySelector('.lb');if(!lb)return;
if(e.key==='Escape'){e.preventDefault();dismiss()}
else if(e.key==='ArrowLeft'&&ii>0&&e.target.tagName!=='VIDEO'){e.preventDefault();open(gi,ii-1)}
else if(e.key==='ArrowRight'&&ii+1<tilesOf(gi).length&&e.target.tagName!=='VIDEO'){e.preventDefault();open(gi,ii+1)}
else if(e.key==='Tab'){
var buttons=[].slice.call(lb.querySelectorAll('button,video[controls]')),i=buttons.indexOf(document.activeElement);
if(e.shiftKey&&i===0){e.preventDefault();buttons[buttons.length-1].focus()}
else if(!e.shiftKey&&i===buttons.length-1){e.preventDefault();buttons[0].focus()}
}
});
})();</script>`
