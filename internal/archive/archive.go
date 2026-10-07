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
	"path"
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
	CircleName string
	CutoffDate string
	Layout     Layout
	Posts      []chronicle.FeedPost
	Blobs      *blob.Store
	// Avatars maps identity id to avatar blob id (faces are not anonymized).
	Avatars map[string]string
	// DayTitles maps entry date to the day title said before the cutoff.
	DayTitles map[string]string
	// Color — ключ цвета круга (circles.color): шапка и акценты архива.
	Color string
	// Fonts — сборка клиента (web/dist); из неё в ZIP кладётся Golos Text.
	// nil или файла нет — архив на системном шрифте.
	Fonts fs.FS
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
	layout := in.Layout
	if layout == "" {
		layout = LayoutFeed
	}
	zw := zip.NewWriter(w)

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
		if errors.Is(err, blob.ErrNotFound) {
			log.Printf("archive: blob %s unavailable, skipped", blobID)
			return nil
		}
		if err != nil {
			return fmt.Errorf("archive: blob %s: %w", blobID, err)
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
		name := "media/" + blobID + extensionForMime(info.MimeType)
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
		// Вложения комментариев — тоже в архив (4.29).
		for _, c := range fp.Comments {
			for _, m := range c.Media {
				if err := addBlob(m.BlobID); err != nil {
					return err
				}
			}
		}
	}
	for _, blobID := range in.Avatars {
		if err := addBlob(blobID); err != nil {
			return err
		}
	}

	st := style{hex: circleHex(in.Color)}
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

	switch layout {
	case LayoutPosts:
		index := renderPostsIndex(st, in.CircleName, in.CutoffDate, in.Posts, in.DayTitles)
		if err := writeZipFile(zw, "index.html", []byte(index)); err != nil {
			return err
		}
		for _, fp := range in.Posts {
			body := renderPostPage(st, in.CircleName, fp, mediaNames, in.Avatars, in.DayTitles)
			fname := path.Join("posts", fp.Post.ID+".html")
			if err := writeZipFile(zw, fname, []byte(body)); err != nil {
				return err
			}
		}
	default:
		feed := renderFeedIndex(st, in.CircleName, in.CutoffDate, in.Posts, mediaNames, in.Avatars, in.DayTitles)
		if err := writeZipFile(zw, "index.html", []byte(feed)); err != nil {
			return err
		}
	}

	return zw.Close()
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
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
	case "video/mp4":
		return ".mp4"
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
}

// Вёрстка — как в клиенте (wynd.html, «Квота и архив»): бумага и чернила из
// tokens.css, цвет круга в шапке и акцентах, карточка записи и комментарии
// как в ленте. Всё внутри файла: архив открывается офлайн и без Wynd.
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
	b.WriteString("</h1><p>Архив круга до ")
	b.WriteString(html.EscapeString(humanDate(cutoff)))
	b.WriteString("</p></header>")
}

const pageEnd = "<p class=\"end\">Wynd · персональный архив</p></div></body></html>"

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
		appendPostHTML(&b, fp, media, avatars, dayTitles, false)
	}
	b.WriteString(pageEnd)
	return b.String()
}

func renderPostsIndex(st style, circleName, cutoff string, posts []chronicle.FeedPost, dayTitles map[string]string) string {
	var b strings.Builder
	st.head(&b, circleName+" — оглавление", "")
	st.header(&b, circleName, cutoff)
	b.WriteString("<ul class=\"toc\">")
	for _, fp := range posts {
		label := fp.Post.AuthorName
		if fp.Post.Body != "" {
			label += ": " + snippet(fp.Post.Body, snippetRunes)
		}
		b.WriteString("<li><a href=\"posts/")
		b.WriteString(html.EscapeString(fp.Post.ID))
		b.WriteString(".html\">")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</a><div class=\"meta\">")
		b.WriteString(entryDateHTML(fp.Post.EntryDate, dayTitles))
		b.WriteString("</div></li>")
	}
	b.WriteString("</ul>")
	b.WriteString(pageEnd)
	return b.String()
}

func renderPostPage(st style, circleName string, fp chronicle.FeedPost, media, avatars, dayTitles map[string]string) string {
	var b strings.Builder
	st.head(&b, circleName, "../")
	b.WriteString("<a class=\"back\" href=\"../index.html\">← ")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString("</a>")
	appendPostHTML(&b, fp, media, avatars, dayTitles, true)
	b.WriteString(pageEnd)
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

func appendAudioHTML(b *strings.Builder, m chronicle.PostMedia, media map[string]string, prefix string) {
	src, ok := media[m.BlobID]
	if !ok {
		b.WriteString("<p class=\"gone\">Файл недоступен на сервере</p>")
		return
	}
	b.WriteString("<div class=\"media\">")
	if cover, ok := media[m.AudioCoverBlobID]; ok {
		b.WriteString("<img alt=\"\" src=\"")
		b.WriteString(html.EscapeString(prefix + cover))
		b.WriteString("\">")
	}
	b.WriteString("<audio controls src=\"")
	b.WriteString(html.EscapeString(prefix + src))
	b.WriteString("\"></audio>")
	label := strings.TrimSpace(m.OriginalFilename)
	artist := strings.TrimSpace(m.AudioArtist)
	title := strings.TrimSpace(m.AudioTitle)
	if artist != "" && title != "" {
		label = artist + " — " + title
	}
	if label != "" {
		b.WriteString("<div>")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</div>")
	}
	b.WriteString("</div>")
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
