package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Layout selects HTML export mode.
type Layout string

const (
	LayoutFeed  Layout = "feed"
	LayoutPosts Layout = "posts"
)

// BuildInput is passed to BuildPersonalArchive.
type BuildInput struct {
	// CircleID — в метаданных архива: архивы разных кругов Мастерская не
	// объединяет.
	CircleID   string
	CircleName string
	CutoffDate string
	// Layout не влияет на архив: с 0.30.1 в нём обе раскладки (кадр 6.23).
	// Поле осталось, чтобы старые клиенты с ?layout= получали тот же архив.
	Layout Layout
	Posts  []chronicle.FeedPost
	Blobs  *blob.Store
	// Avatars maps identity id to avatar blob id (faces are not anonymized).
	Avatars map[string]string
	// DayTitles maps entry date to the day title said before the cutoff.
	DayTitles map[string]string
	// Color — ключ цвета круга (circles.color): шапка и акценты архива.
	Color string
	// Fonts — сборка клиента (web/dist); из неё в ZIP кладётся Golos Text.
	// nil или файла нет — архив на системном шрифте.
	Fonts fs.FS
	// SizeOnly — холостая сборка: вместо медиа нули той же длины. Длина ZIP
	// от содержимого файлов не зависит (Store, заголовки без времени), так что
	// холостой проход заранее даёт точный размер для Content-Length — и диск
	// при этом не читается.
	SizeOnly bool
	// layout writes a persistent ZIP index with references instead of media bytes.
	layout *layoutWriter
}

// BuildPersonalArchive writes a self-contained ZIP with media/ and HTML to w.
//
// Пишет в поток, а не в память: на круге в несколько гигабайт архив целиком в
// bytes.Buffer занимал столько же ОЗУ на каждого качающего (ARC-1). Медиа
// кладутся без сжатия — JPEG, WebP и MP4 Deflate не уменьшает, а время
// сборки на нём уходило (ARC-2). Сборка прерывается отменой ctx.
func BuildPersonalArchive(ctx context.Context, w io.Writer, in BuildInput) error {
	if in.Blobs == nil {
		return fmt.Errorf("archive: nil blob store")
	}
	zw := zip.NewWriter(w)
	if err := buildReadable(ctx, zw, in, nil, style{hex: circleHex(in.Color)}); err != nil {
		return err
	}
	return zw.Close()
}

// buildReadable пишет в zw медиа и читаемую ленту — общее у архива участника
// и полного архива круга. extra — файлы, которых нет в ленте (обложки дней,
// аватары ушедших): полный архив кладёт все файлы круга.
func buildReadable(ctx context.Context, zw *zip.Writer, in BuildInput, extra []string, st style) error {
	mediaNames := make(map[string]string)
	addBlob := func(blobID string) error {
		if blobID == "" {
			return nil
		}
		if _, ok := mediaNames[blobID]; ok {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Недоступный файл пропускается, а не роняет архив: оценка его и так не
		// считает, а один потерянный на диске файл закрывал экспорт всему кругу
		// — перед самой чисткой (план 42, ARC-4). В HTML на его месте пометка.
		info, err := in.Blobs.OpenBlob(ctx, blobID)
		if errors.Is(err, blob.ErrNotFound) && in.layout == nil {
			log.Printf("archive: blob %s unavailable, skipped", blobID)
			return nil
		}
		if err != nil {
			return fmt.Errorf("archive: blob %s: %w", blobID, err)
		}
		name := "media/" + blobID + mediaExtension(info.MimeType, info.Filename)
		if in.layout != nil {
			if err := in.layout.addBlob(ctx, zw, in.Blobs, blobID, name); err != nil {
				return err
			}
			mediaNames[blobID] = name
			return nil
		}
		if in.SizeOnly {
			fi, err := os.Stat(info.Path)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("archive: stat blob %s: %w", blobID, err)
			}
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
			if err != nil {
				return err
			}
			if _, err := io.CopyN(fw, zeros{}, fi.Size()); err != nil {
				return err
			}
			mediaNames[blobID] = name
			return nil
		}
		f, err := openBlobFile(info.Path)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("archive: blob %s file missing, skipped", blobID)
			return nil
		}
		if err != nil {
			return fmt.Errorf("archive: open blob %s: %w", blobID, err)
		}
		defer f.Close()
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return err
		}
		if _, err := io.Copy(fw, f); err != nil {
			return err
		}
		mediaNames[blobID] = name
		return nil
	}
	for _, fp := range in.Posts {
		for _, m := range fp.Media {
			if err := addBlob(m.BlobID); err != nil {
				return err
			}
			if err := addBlob(m.AudioCoverBlobID); err != nil {
				return err
			}
			if err := addBlob(m.VideoPosterBlobID); err != nil {
				return err
			}
		}
		// Вложения комментариев — тоже в архив (4.21).
		for _, c := range fp.Comments {
			for _, m := range c.Media {
				if err := addBlob(m.BlobID); err != nil {
					return err
				}
			}
		}
	}
	avatarIDs := make([]string, 0, len(in.Avatars))
	for _, blobID := range in.Avatars {
		avatarIDs = append(avatarIDs, blobID)
	}
	sort.Strings(avatarIDs)
	for _, blobID := range avatarIDs {
		if err := addBlob(blobID); err != nil {
			return err
		}
	}
	for _, blobID := range extra {
		if err := addBlob(blobID); err != nil {
			return err
		}
	}

	for _, f := range archiveFonts {
		data, err := readFont(in.Fonts, f)
		if err != nil || data == nil {
			continue
		}
		if err := writeStoredFile(zw, "fonts/"+f, data); err != nil {
			return err
		}
		st.fonts = append(st.fonts, f)
	}

	// В архиве обе раскладки (кадр 6.23, решение 2026-10-13): единая лента в
	// index.html и страница на каждую запись в posts/ с оглавлением, медиа у
	// них общее. Выбирать заранее не нужно — и пересобирать ради другой тоже.
	meta, err := buildMeta(in, mediaNames)
	if err != nil {
		return err
	}
	if err := writeZipFile(zw, MetaName, meta); err != nil {
		return err
	}
	feed := renderFeedIndex(st, in.CircleName, in.CutoffDate, in.Posts, mediaNames, in.Avatars, in.DayTitles)
	if err := writeZipFile(zw, "index.html", []byte(feed)); err != nil {
		return err
	}
	toc := renderPostsIndex(st, in.CircleName, in.CutoffDate, in.Posts, in.DayTitles)
	if err := writeZipFile(zw, "posts/index.html", []byte(toc)); err != nil {
		return err
	}
	for _, fp := range in.Posts {
		body := renderPostPage(st, in.CircleName, fp, mediaNames, in.Avatars, in.DayTitles)
		fname := path.Join("posts", fp.Post.ID+".html")
		if err := writeZipFile(zw, fname, []byte(body)); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// mediaExtension — расширение файла в архиве. Архив открывают с диска, и
// браузер узнаёт тип файла только по расширению: звук и видео с «.bin» не
// играли (план 46, C39) — голосовые и видеосообщения из Firefox приходят WebM.
// Тип без своего расширения в таблице — расширение исходного имени файла.
func mediaExtension(mime, filename string) string {
	if ext := extensionForMime(mime); ext != ".bin" {
		return ext
	}
	ext := strings.ToLower(path.Ext(filename))
	if len(ext) >= 2 && len(ext) <= 6 && strings.Trim(ext[1:], "abcdefghijklmnopqrstuvwxyz0123456789") == "" {
		return ext
	}
	return ".bin"
}

func extensionForMime(mime string) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	if i := strings.Index(m, ";"); i >= 0 {
		m = strings.TrimSpace(m[:i])
	}
	switch m {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/heic", "image/heif":
		return ".heic"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/mp4", "audio/x-m4a", "audio/m4a":
		return ".m4a"
	case "audio/aac":
		return ".aac"
	case "audio/ogg":
		return ".ogg"
	case "audio/opus":
		return ".opus"
	case "audio/webm":
		return ".webm"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/flac", "audio/x-flac":
		return ".flac"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

// archiveFonts — файлы Golos Text из сборки клиента: кириллица и латиница.
// Расширенные наборы не нужны — архив читает русский текст.
var archiveFonts = []string{
	"GolosText-Variable-cyrillic.woff2",
	"GolosText-Variable-latin.woff2",
}

func readFont(fonts fs.FS, name string) ([]byte, error) {
	if fonts == nil {
		return nil, nil
	}
	data, err := fs.ReadFile(fonts, "fonts/"+name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func writeStoredFile(zw *zip.Writer, name string, data []byte) error {
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}

// circleHexes — палитра кругов, та же, что в клиенте
// (web/src/lib/theme/colors.ts). Неизвестный ключ — терракота.
var circleHexes = map[string]string{
	"terracotta": "#AF5839", "teal": "#357077", "olive": "#58673A", "ochre": "#70571B",
	"plum": "#7A4265", "indigo": "#3C4D83", "coffee": "#62452F", "slate": "#3D494F",
}

func circleHex(color string) string {
	if hex, ok := circleHexes[color]; ok {
		return hex
	}
	return circleHexes["terracotta"]
}

// style — оформление страниц одного архива: цвет круга и вшитые шрифты.
type style struct {
	hex   string
	fonts []string
	// full — полный архив круга: шапка «на дату», а не «до», и своя подпись.
	full    bool
	partial bool
}

func (st style) pageEnd() string {
	if st.partial {
		return "<p class=\"end\">Wynd · часть журнала круга</p></div></body></html>"
	}
	if st.full {
		return pageEndFull
	}
	return pageEnd
}

// Вёрстка — как в клиенте (wynd.html, «Квота и архив»): бумага и чернила из
// tokens.css, цвет круга в шапке и акцентах, карточка записи и комментарии
// как в ленте. Всё внутри файла: архив открывается офлайн и без Wynd.
// Стили сверяются с web/src/lib/styles/{tokens,ui}.css тестом
// TestArchiveStylesMatchApp — при смене значений в приложении тест укажет
// на архив, задокументированные отличия записаны в его таблице. Полосатая
// заглушка обложки без файла — как .pic (решение владельца 2026-11-12).
const baseCSS = `
:root{--paper:#F4F0E9;--card:#FCFAF6;--ink:#2B2724;--muted:#7A7269;--faint:#A8A096;--line:#DFD8CD}
*{box-sizing:border-box}
body{font-family:"Golos Text",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;background:var(--paper);color:var(--ink);margin:0;line-height:1.5;font-size:15px}
.wrap{max-width:640px;margin:0 auto}
.head{background:var(--c);color:#fff;padding:22px 20px 18px}
.head h1{font-size:22px;margin:0;font-weight:700}
.head p{margin:4px 0 0;opacity:.85;font-size:13.5px}
.head a{color:#fff}
.post{background:var(--card);border:1px solid var(--line);border-radius:16px;margin:14px 12px;padding:14px 16px}
.who{display:flex;align-items:center;gap:10px}
.author{font-weight:600}
.av{width:32px;height:32px;border-radius:50%;object-fit:cover;flex:none}
.avl{width:32px;height:32px;border-radius:50%;background:var(--c);color:#fff;display:inline-flex;align-items:center;justify-content:center;font-weight:700;font-size:14px;flex:none}
.meta{color:var(--muted);font-size:12.5px}
.body{margin:10px 0 0;white-space:pre-wrap}
.media img,.media video{width:100%;height:auto;display:block;border-radius:12px;margin:10px 0 0}
.gone{color:var(--faint);font-size:12.5px;margin:10px 0 0}
.reactions{margin-top:10px;font-size:13px;color:var(--muted)}
.comments{margin-top:12px;border-top:1px solid var(--line);padding-top:8px}
.comment{font-size:14px;margin:6px 0}
.comment .author{color:var(--c)}
.cmedia img{max-width:240px}
.cfile{margin:6px 0;font-size:14px}
.toc{list-style:none;margin:8px 0;padding:0}
.toc li{background:var(--card);border:1px solid var(--line);border-radius:14px;margin:10px 12px;padding:12px 16px}
.toc a{color:var(--ink);text-decoration:none;font-weight:600}
.toc a:hover{color:var(--c)}
.back{display:inline-block;margin:14px 12px 0;color:var(--c)}
.end{text-align:center;color:var(--faint);font-size:12.5px;padding:24px 16px 40px}
.voice{display:flex;align-items:center;gap:11px;margin:12px 0 0;width:100%;padding:0;border:0;background:none;color:inherit;font:inherit;text-align:left;cursor:pointer}
.voice-play{width:34px;height:34px;border-radius:50%;background:var(--c);display:grid;place-items:center;flex:0 0 auto;border:0;padding:0;cursor:pointer;color:#fff}
.voice-play .ic{width:17px;height:17px;color:#fff}
.vwave{display:flex;align-items:center;gap:1.5px;height:22px;flex:1;min-width:0;overflow:hidden}
.vwave i{flex:1 1 0;max-width:3px;min-height:3px;border-radius:2px;background:var(--line)}
.vwave i.on{background:var(--c)}
.sz{font-size:11.5px;color:var(--faint)}
.att{display:flex;align-items:center;gap:11px;border:1px solid var(--line);border-radius:10px;padding:11px 13px;margin:12px 0 0;min-width:0;box-sizing:border-box;width:100%;text-align:left}
.att-play{display:flex;align-items:center;gap:11px;flex:1;min-width:0;text-align:left;background:none;border:0;padding:0;color:inherit;font:inherit;cursor:pointer}
.att-cover{width:72px;height:72px;border-radius:8px;flex:0 0 auto;overflow:hidden;display:block;border:1px solid var(--line);background-color:#c3af95;background-image:repeating-linear-gradient(0deg,rgba(255,255,255,0.07) 0 7px,rgba(0,0,0,0.05) 7px 15px)}
.att-cover img{width:100%;height:100%;object-fit:cover;display:block;border-radius:inherit}
.att-body{flex:1;min-width:0;display:flex;flex-direction:column;justify-content:center;gap:11px}
.att-line{display:flex;align-items:center;gap:11px;min-width:0}
.att-line .ic{width:21px;height:21px;color:var(--c);flex:0 0 auto}
.att-name{font-size:12.5px;line-height:1.3;flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.att-bar{display:block;height:3px;border-radius:2px;background:var(--line);overflow:hidden}
.att-bar i{display:block;height:100%;background:var(--c);width:0}
.att .sz{line-height:1}
.dl{line-height:0;color:var(--muted);flex:none;border:0;background:none;padding:0}
.dl .ic{width:21px;height:21px}
.ic{width:21px;height:21px;fill:none;stroke:currentColor;stroke-width:1.6;stroke-linecap:round;stroke-linejoin:round}
.pause,.voice.playing .play,.att.playing .play{display:none}
.voice.playing .pause,.att.playing .pause{display:inline}
noscript audio{width:100%;margin:10px 0 0}
`

// head пишет <head> страницы: prefix — путь до корня архива ("" или "../").
func (st style) head(b *strings.Builder, title, prefix string) {
	b.WriteString("<!DOCTYPE html><html lang=\"ru\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>")
	for _, f := range st.fonts {
		b.WriteString("@font-face{font-family:\"Golos Text\";font-weight:400 900;src:url(\"")
		b.WriteString(prefix + "fonts/" + f)
		b.WriteString("\") format(\"woff2\")}")
	}
	b.WriteString(":root{--c:")
	b.WriteString(st.hex)
	b.WriteString("}")
	b.WriteString(baseCSS)
	b.WriteString("</style></head><body><div class=\"wrap\">")
}

func (st style) header(b *strings.Builder, circleName, cutoff string) {
	b.WriteString("<header class=\"head\"><h1>")
	b.WriteString(html.EscapeString(circleName))
	if st.partial {
		b.WriteString("</h1><p>Часть журнала круга на ")
	} else if st.full {
		b.WriteString("</h1><p>Полный архив круга на ")
	} else {
		b.WriteString("</h1><p>Архив круга до ")
	}
	b.WriteString(html.EscapeString(humanDate(cutoff)))
	b.WriteString("</p></header>")
}

const (
	pageEnd     = "<p class=\"end\">Wynd · персональный архив</p>" + audioScript + "</div></body></html>"
	pageEndFull = "<p class=\"end\">Wynd · полный архив круга</p>" + audioScript + "</div></body></html>"
)

// audioScript — один плеер на страницу: нажатие на строку играет её файл,
// полоса и волна идут по ходу, повторное нажатие — пауза. Всё внутри файла:
// архив остаётся офлайн, без JS играет нативный <audio> из <noscript>.
const audioScript = `<script>(function(){
var el=new Audio(),active=null,src="";
function mmss(s){s=Math.max(0,Math.floor(s||0));var m=Math.floor(s/60),r=s%60;return m+":"+(r<10?"0":"")+r}
function total(row){return Number(row&&row.getAttribute("data-ms")||0)/1000}
function setTime(row,s){var t=row&&row.querySelector(".sz");if(t)t.textContent=mmss(s)}
function setProg(row,f){if(!row)return;var bar=row.querySelector(".att-bar i");if(bar)bar.style.width=(f*100).toFixed(1)+"%";var wave=row.querySelector(".vwave");if(wave){var bars=wave.children,n=Math.round(f*bars.length);for(var i=0;i<bars.length;i++)bars[i].classList[i<n?"add":"remove"]("on")}}
function mark(row,on){if(!row)return;row.classList[on?"add":"remove"]("playing");var b=row.querySelector("button");if(b)b.setAttribute("aria-pressed",on?"true":"false")}
function idle(row){mark(row,false);setProg(row,0);setTime(row,total(row))}
el.addEventListener("timeupdate",function(){if(!active)return;setProg(active,el.duration?el.currentTime/el.duration:0);setTime(active,el.currentTime)});
el.addEventListener("loadedmetadata",function(){if(active&&isFinite(el.duration)&&el.duration>0){active.setAttribute("data-ms",String(Math.round(el.duration*1000)));if(el.currentTime===0)setTime(active,total(active))}});
el.addEventListener("ended",function(){if(active)idle(active)});
el.addEventListener("pause",function(){mark(active,false)});
el.addEventListener("play",function(){mark(active,true)});
document.addEventListener("click",function(ev){
var btn=ev.target&&ev.target.closest?ev.target.closest("button[data-src]"):null;if(!btn)return;
var row=btn.closest(".voice,.att"),next=btn.getAttribute("data-src");
if(src===next&&!el.paused){el.pause();return}
if(src!==next){if(active&&active!==row)idle(active);src=next;active=row;el.src=next}
else{active=row}
el.play().catch(function(){});
});
})()</script>`

var monthsGen = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// humanDate: "2026-08-30" → "30 августа 2026"; нераспознанное — как есть.
func humanDate(isoDay string) string {
	t, err := time.Parse("2006-01-02", isoDay)
	if err != nil {
		return isoDay
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthsGen[t.Month()-1], t.Year())
}

// snippetRunes — длина начала записи в оглавлении раскладки posts.
const snippetRunes = 60

// snippet режет текст по рунам, а не по байтам — срез байтов рвал
// кириллическую букву пополам, и оглавление показывало «�» (ARC-3); обрыв —
// на последнем пробеле, если он не слишком близко к началу.
func snippet(body string, limit int) string {
	runes := []rune(body)
	if len(runes) <= limit {
		return body
	}
	cut := runes[:limit]
	for i := len(cut) - 1; i > limit/2; i-- {
		if unicode.IsSpace(cut[i]) {
			cut = cut[:i]
			break
		}
	}
	return string(cut) + "…"
}

// entryDateHTML — дата отнесения и, если есть, название дня (ARC-6).
func entryDateHTML(entryDate string, dayTitles map[string]string) string {
	out := html.EscapeString(humanDate(entryDate))
	if title := dayTitles[entryDate]; title != "" {
		out += " · «" + html.EscapeString(title) + "»"
	}
	return out
}

func renderFeedIndex(st style, circleName, cutoff string, posts []chronicle.FeedPost, media, avatars, dayTitles map[string]string) string {
	var b strings.Builder
	st.head(&b, circleName, "")
	st.header(&b, circleName, cutoff)
	for _, fp := range posts {
		// Метки — для Мастерской: объединённая лента склеивается из кусков.
		b.WriteString(MetaPostOpen(fp.Post.ID))
		appendPostHTML(&b, fp, media, avatars, dayTitles, false)
		b.WriteString(MetaPostClose)
	}
	b.WriteString(st.pageEnd())
	return b.String()
}

func renderPostsIndex(st style, circleName, cutoff string, posts []chronicle.FeedPost, dayTitles map[string]string) string {
	var b strings.Builder
	st.head(&b, circleName+" — оглавление", "../")
	st.header(&b, circleName, cutoff)
	b.WriteString("<a class=\"back\" href=\"../index.html\">← единая лента</a>")
	b.WriteString("<ul class=\"toc\">")
	for _, fp := range posts {
		label := fp.Post.AuthorName
		if fp.Post.Body != "" {
			label += ": " + snippet(fp.Post.Body, snippetRunes)
		}
		b.WriteString(MetaTocOpen(fp.Post.ID))
		b.WriteString("<li><a href=\"")
		b.WriteString(html.EscapeString(fp.Post.ID))
		b.WriteString(".html\">")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</a><div class=\"meta\">")
		b.WriteString(entryDateHTML(fp.Post.EntryDate, dayTitles))
		b.WriteString("</div></li>")
		b.WriteString(MetaTocClose)
	}
	b.WriteString("</ul>")
	b.WriteString(st.pageEnd())
	return b.String()
}

func renderPostPage(st style, circleName string, fp chronicle.FeedPost, media, avatars, dayTitles map[string]string) string {
	var b strings.Builder
	st.head(&b, circleName, "../")
	b.WriteString("<a class=\"back\" href=\"index.html\">← оглавление</a>")
	appendPostHTML(&b, fp, media, avatars, dayTitles, true)
	b.WriteString(st.pageEnd())
	return b.String()
}

// appendAvatarHTML — аватар лица, а без него буква имени на цвете круга.
func appendAvatarHTML(b *strings.Builder, name, identityID string, media, avatars map[string]string, prefix string) {
	if blobID, ok := avatars[identityID]; ok {
		if src, ok := media[blobID]; ok {
			b.WriteString("<img class=\"av\" alt=\"\" src=\"")
			b.WriteString(html.EscapeString(prefix + src))
			b.WriteString("\">")
			return
		}
	}
	initial := "·"
	for _, r := range strings.TrimSpace(name) {
		initial = strings.ToUpper(string(r))
		break
	}
	b.WriteString("<span class=\"avl\">")
	b.WriteString(html.EscapeString(initial))
	b.WriteString("</span>")
}

func appendPostHTML(b *strings.Builder, fp chronicle.FeedPost, media, avatars, dayTitles map[string]string, relativeMedia bool) {
	prefix := ""
	if relativeMedia {
		prefix = "../"
	}
	b.WriteString("<article class=\"post\"><div class=\"who\">")
	appendAvatarHTML(b, fp.Post.AuthorName, fp.Post.IdentityID, media, avatars, prefix)
	b.WriteString("<div><div class=\"author\">")
	b.WriteString(html.EscapeString(fp.Post.AuthorName))
	b.WriteString("</div><div class=\"meta\">")
	b.WriteString(entryDateHTML(fp.Post.EntryDate, dayTitles))
	if !fp.Post.CreatedAt.IsZero() {
		b.WriteString(" · опубликовано ")
		b.WriteString(html.EscapeString(fp.Post.CreatedAt.UTC().Format("02.01.2006 15:04")))
		b.WriteString(" UTC")
	}
	b.WriteString("</div></div></div>")
	if fp.Post.Body != "" {
		b.WriteString("<div class=\"body\">")
		b.WriteString(html.EscapeString(fp.Post.Body))
		b.WriteString("</div>")
	}
	for _, m := range fp.Media {
		if playableAudio(m) {
			appendAudioHTML(b, m, media, prefix)
			continue
		}
		src, ok := media[m.BlobID]
		if !ok {
			b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
			continue
		}
		if m.Kind == chronicle.MediaAttachment {
			// Файл записи — ссылкой, как в комментарии: картинкой он не
			// показывался, на его месте была пустая рамка (C39).
			name := m.OriginalFilename
			if name == "" {
				name = "файл"
			}
			b.WriteString("<p class=\"cfile\"><a href=\"")
			b.WriteString(html.EscapeString(prefix + src))
			b.WriteString("\">")
			b.WriteString(html.EscapeString(name))
			b.WriteString("</a></p>")
			continue
		}
		b.WriteString("<div class=\"media\">")
		if m.Kind == chronicle.MediaVideo {
			b.WriteString("<video controls src=\"")
		} else {
			b.WriteString("<img alt=\"\" src=\"")
		}
		b.WriteString(html.EscapeString(prefix + src))
		if m.Kind == chronicle.MediaVideo {
			b.WriteString("\"></video>")
		} else {
			b.WriteString("\">")
		}
		b.WriteString("</div>")
	}
	if len(fp.Reactions) > 0 {
		b.WriteString("<div class=\"reactions\">")
		for i, r := range fp.Reactions {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(html.EscapeString(reactionMark(r.Emoji)))
			b.WriteString(" ")
			b.WriteString(html.EscapeString(r.AuthorName))
		}
		b.WriteString("</div>")
	}
	if len(fp.Comments) > 0 {
		b.WriteString("<div class=\"comments\">")
		for _, c := range fp.Comments {
			b.WriteString("<div class=\"comment\"><span class=\"author\">")
			b.WriteString(html.EscapeString(c.AuthorName))
			b.WriteString(":</span> ")
			b.WriteString(html.EscapeString(c.Body))
			appendCommentMediaHTML(b, c.Media, media, prefix)
			b.WriteString("</div>")
		}
		b.WriteString("</div>")
	}
	b.WriteString("</article>")
}

// appendCommentMediaHTML — вложения комментария: снимок картинкой, голосовое
// и звук плеером, остальное ссылкой на файл.
func appendCommentMediaHTML(b *strings.Builder, items []chronicle.PostMedia, media map[string]string, prefix string) {
	for _, m := range items {
		if playableAudio(m) {
			appendAudioHTML(b, m, media, prefix)
			continue
		}
		src, ok := media[m.BlobID]
		if !ok {
			b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
			continue
		}
		if m.Kind == chronicle.MediaPhoto {
			b.WriteString("<div class=\"media cmedia\"><img alt=\"\" src=\"")
			b.WriteString(html.EscapeString(prefix + src))
			b.WriteString("\"></div>")
			continue
		}
		name := m.OriginalFilename
		if name == "" {
			name = "файл"
		}
		b.WriteString("<p class=\"cfile\"><a href=\"")
		b.WriteString(html.EscapeString(prefix + src))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(name))
		b.WriteString("</a></p>")
	}
}

func playableAudio(m chronicle.PostMedia) bool {
	mime := strings.ToLower(strings.TrimSpace(m.MimeType))
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	if strings.HasPrefix(mime, "audio/") {
		return true
	}
	if mime != "" && mime != "application/octet-stream" {
		return false
	}
	switch strings.ToLower(path.Ext(m.OriginalFilename)) {
	case ".m4a", ".mp3", ".aac", ".ogg", ".opus", ".wav", ".flac":
		return true
	default:
		return false
	}
}

// appendAudioHTML — звук в записи или комментарии: голосовое — круглая кнопка
// цвета круга, волна и длительность (как 4.16), звук-файл — обложка, название,
// полоса хода и время (как 4.6/4.7). Раньше здесь стоял нативный <audio
// controls> и звук в архиве выглядел не так, как в приложении.
func appendAudioHTML(b *strings.Builder, m chronicle.PostMedia, media map[string]string, prefix string) {
	src, ok := media[m.BlobID]
	if !ok {
		b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
		return
	}
	href := html.EscapeString(prefix + src)
	if m.Voice {
		appendVoiceHTML(b, m, href)
	} else {
		appendSoundHTML(b, m, media, prefix, href)
	}
	// Без JS архив остаётся читаемым: играет нативный плеер.
	b.WriteString("<noscript><audio controls preload=\"none\" src=\"")
	b.WriteString(href)
	b.WriteString("\"></audio></noscript>")
}

// iconPlay, iconPause, iconDownload — те же значки, что в приложении
// (web/static/icons.svg): плеер архива рисуется теми же формами.
const (
	iconPlay     = `<svg class="ic play" viewBox="0 0 24 24" aria-hidden="true"><path d="M7.5 4.5 19.5 12l-12 7.5z"/></svg>`
	iconPause    = `<svg class="ic pause" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 5h3.5v14H7zM13.5 5H17v14h-3.5z"/></svg>`
	iconDownload = `<svg class="ic" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4v10.6M6.2 11.3 12 17l5.8-5.7"/><path d="M4.5 19.8h15"/></svg>`
)

// appendVoiceHTML — голосовое как в приложении (4.16): круглая кнопка цвета
// круга 34×34, волна из уровней записи и длительность.
func appendVoiceHTML(b *strings.Builder, m chronicle.PostMedia, href string) {
	b.WriteString(`<div class="voice" data-ms="`)
	b.WriteString(strconv.FormatInt(m.AudioDurationMs, 10))
	b.WriteString(`"><button type="button" class="voice-play" aria-label="Слушать голосовое" data-src="`)
	b.WriteString(href)
	b.WriteString(`">`)
	b.WriteString(iconPlay)
	b.WriteString(iconPause)
	b.WriteString(`</button><span class="vwave" aria-hidden="true">`)
	for _, peak := range wavePeaks(m.AudioPeaks) {
		b.WriteString(`<i style="height:`)
		b.WriteString(strconv.Itoa(peak))
		b.WriteString(`%"></i>`)
	}
	b.WriteString(`</span><span class="sz">`)
	b.WriteString(mmss(m.AudioDurationMs))
	b.WriteString(`</span></div>`)
}

// appendSoundHTML — звук-файл как в приложении (4.6/4.7): обложка 72×72,
// значок игры и название, полоса хода, время и кнопка скачивания.
func appendSoundHTML(b *strings.Builder, m chronicle.PostMedia, media map[string]string, prefix, href string) {
	b.WriteString(`<div class="att" data-ms="0"><button type="button" class="att-play" aria-label="Слушать" data-src="`)
	b.WriteString(href)
	b.WriteString(`"><span class="att-cover">`)
	if cover, ok := media[m.AudioCoverBlobID]; ok {
		b.WriteString(`<img alt="" src="`)
		b.WriteString(html.EscapeString(prefix + cover))
		b.WriteString(`">`)
	}
	b.WriteString(`</span><span class="att-body"><span class="att-line">`)
	b.WriteString(iconPlay)
	b.WriteString(iconPause)
	b.WriteString(`<span class="att-name">`)
	b.WriteString(html.EscapeString(audioLabel(m)))
	b.WriteString(`</span></span><span class="att-bar"><i></i></span><span class="sz">0:00</span></span></button><a class="dl" href="`)
	b.WriteString(href)
	b.WriteString(`" download aria-label="Скачать">`)
	b.WriteString(iconDownload)
	b.WriteString(`</a></div>`)
}

// audioLabel — подпись звука: «Исполнитель — Название» или имя файла.
func audioLabel(m chronicle.PostMedia) string {
	artist := strings.TrimSpace(m.AudioArtist)
	title := strings.TrimSpace(m.AudioTitle)
	if artist != "" && title != "" {
		return artist + " — " + title
	}
	if label := strings.TrimSpace(m.OriginalFilename); label != "" {
		return label
	}
	return "звук"
}

// wavePeaks — уровни волны 1…100: в записи их до 64, у старых голосовых может
// не быть — тогда ровная линия, чтобы строка не пустела.
func wavePeaks(peaks []int) []int {
	if len(peaks) == 0 {
		out := make([]int, 32)
		for i := range out {
			out[i] = 20
		}
		return out
	}
	out := make([]int, 0, len(peaks))
	for _, p := range peaks {
		if p < 4 {
			p = 4
		}
		if p > 100 {
			p = 100
		}
		out = append(out, p)
	}
	return out
}

// mmss — длительность «1:05» для подписи голосового.
func mmss(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	total := (ms + 500) / 1000
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// reactionMark — знак реакции как в ленте; ключи API: heart, laugh,
// surprise, anger. Незнакомое значение ранних версий — как есть.
func reactionMark(key string) string {
	switch key {
	case "heart":
		return "♥"
	case "laugh":
		return "😄"
	case "surprise":
		return "😮"
	case "anger":
		return "😠"
	default:
		return key
	}
}

// HasExternalLinks reports whether HTML contains http(s) references.
func HasExternalLinks(htmlDoc string) bool {
	lower := strings.ToLower(htmlDoc)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "//www.")
}

// zeros — бесконечный поток нулей для холостой сборки (SizeOnly).
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// Size — точная длина архива in в байтах: холостая сборка без чтения медиа.
func Size(ctx context.Context, in BuildInput) (int64, error) {
	in.SizeOnly = true
	var cw countWriter
	if err := BuildPersonalArchive(ctx, &cw, in); err != nil {
		return 0, err
	}
	return cw.n, nil
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
