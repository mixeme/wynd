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
		}
	}
	for _, blobID := range in.Avatars {
		if err := addBlob(blobID); err != nil {
			return err
		}
	}

	switch layout {
	case LayoutPosts:
		index := renderPostsIndex(in.CircleName, in.CutoffDate, in.Posts, in.DayTitles)
		if err := writeZipFile(zw, "index.html", []byte(index)); err != nil {
			return err
		}
		for _, fp := range in.Posts {
			body := renderPostPage(in.CircleName, fp, mediaNames, in.Avatars, in.DayTitles)
			fname := path.Join("posts", fp.Post.ID+".html")
			if err := writeZipFile(zw, fname, []byte(body)); err != nil {
				return err
			}
		}
	default:
		feed := renderFeedIndex(in.CircleName, in.CutoffDate, in.Posts, mediaNames, in.Avatars, in.DayTitles)
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

const baseCSS = `
body{font-family:system-ui,sans-serif;background:#f5f0eb;color:#1a1a1a;margin:0;padding:16px;line-height:1.5}
h1{font-size:1.25rem;margin:0 0 8px}
.meta{color:#666;font-size:0.85rem;margin-bottom:16px}
.post{border-top:1px solid #ddd;padding:12px 0}
.author{font-weight:600;color:#c4725a}
.av{width:24px;height:24px;border-radius:50%;vertical-align:middle;margin-right:6px;object-fit:cover}
.body{margin:8px 0;white-space:pre-wrap}
.media img,video{max-width:100%;height:auto;display:block;margin:8px 0}
.comments{margin-top:8px;padding-left:12px;border-left:3px solid #e8ddd4}
.comment{font-size:0.9rem;margin:4px 0}
.reactions{font-size:0.85rem;color:#666}
a{color:#c4725a}
ul{list-style:none;padding:0}
li{margin:8px 0}
`

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
	out := html.EscapeString(entryDate)
	if title := dayTitles[entryDate]; title != "" {
		out += " · «" + html.EscapeString(title) + "»"
	}
	return out
}

func renderFeedIndex(circleName, cutoff string, posts []chronicle.FeedPost, media, avatars, dayTitles map[string]string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString("</title><style>")
	b.WriteString(baseCSS)
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString("</h1><p class=\"meta\">Архив до ")
	b.WriteString(html.EscapeString(cutoff))
	b.WriteString("</p>")
	for _, fp := range posts {
		appendPostHTML(&b, fp, media, avatars, dayTitles, false)
	}
	b.WriteString("</body></html>")
	return b.String()
}

func renderPostsIndex(circleName, cutoff string, posts []chronicle.FeedPost, dayTitles map[string]string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString(" — оглавление</title><style>")
	b.WriteString(baseCSS)
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString("</h1><p class=\"meta\">Архив до ")
	b.WriteString(html.EscapeString(cutoff))
	b.WriteString("</p><ul>")
	for _, fp := range posts {
		label := fp.Post.AuthorName
		if fp.Post.Body != "" {
			label += ": " + snippet(fp.Post.Body, snippetRunes)
		}
		b.WriteString("<li><a href=\"posts/")
		b.WriteString(html.EscapeString(fp.Post.ID))
		b.WriteString(".html\">")
		b.WriteString(html.EscapeString(label))
		b.WriteString("</a> <span class=\"meta\">")
		b.WriteString(entryDateHTML(fp.Post.EntryDate, dayTitles))
		b.WriteString("</span></li>")
	}
	b.WriteString("</ul></body></html>")
	return b.String()
}

func renderPostPage(circleName string, fp chronicle.FeedPost, media, avatars, dayTitles map[string]string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(circleName))
	b.WriteString("</title><style>")
	b.WriteString(baseCSS)
	b.WriteString("</style></head><body><p class=\"meta\"><a href=\"../index.html\">← К оглавлению</a></p>")
	appendPostHTML(&b, fp, media, avatars, dayTitles, true)
	b.WriteString("</body></html>")
	return b.String()
}

func appendAuthorHTML(b *strings.Builder, name, identityID string, media, avatars map[string]string, prefix string) {
	if avatars != nil {
		if blobID, ok := avatars[identityID]; ok {
			if src, ok := media[blobID]; ok {
				b.WriteString("<img class=\"av\" alt=\"\" src=\"")
				b.WriteString(html.EscapeString(prefix + src))
				b.WriteString("\">")
			}
		}
	}
	b.WriteString(html.EscapeString(name))
}

func appendPostHTML(b *strings.Builder, fp chronicle.FeedPost, media, avatars, dayTitles map[string]string, relativeMedia bool) {
	prefix := ""
	if relativeMedia {
		prefix = "../"
	}
	b.WriteString("<article class=\"post\"><div class=\"author\">")
	appendAuthorHTML(b, fp.Post.AuthorName, fp.Post.IdentityID, media, avatars, prefix)
	b.WriteString("</div><div class=\"meta\">")
	b.WriteString(entryDateHTML(fp.Post.EntryDate, dayTitles))
	if !fp.Post.CreatedAt.IsZero() {
		b.WriteString(" · ")
		b.WriteString(html.EscapeString(fp.Post.CreatedAt.UTC().Format(time.RFC3339)))
	}
	b.WriteString("</div>")
	if fp.Post.Body != "" {
		b.WriteString("<div class=\"body\">")
		b.WriteString(html.EscapeString(fp.Post.Body))
		b.WriteString("</div>")
	}
	for _, m := range fp.Media {
		src, ok := media[m.BlobID]
		if !ok {
			b.WriteString("<p class=\"meta\">Файл недоступен на сервере</p>")
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
	if len(fp.Comments) > 0 {
		b.WriteString("<div class=\"comments\">")
		for _, c := range fp.Comments {
			b.WriteString("<div class=\"comment\"><span class=\"author\">")
			appendAuthorHTML(b, c.AuthorName, c.IdentityID, media, avatars, prefix)
			b.WriteString(":</span> ")
			b.WriteString(html.EscapeString(c.Body))
			b.WriteString("</div>")
		}
		b.WriteString("</div>")
	}
	if len(fp.Reactions) > 0 {
		b.WriteString("<div class=\"reactions\">")
		for _, r := range fp.Reactions {
			appendAuthorHTML(b, r.AuthorName, r.IdentityID, media, avatars, prefix)
			b.WriteString(" ")
			b.WriteString(html.EscapeString(r.Emoji))
			b.WriteString(" ")
		}
		b.WriteString("</div>")
	}
	b.WriteString("</article>")
}

// HasExternalLinks reports whether HTML contains http(s) references.
func HasExternalLinks(htmlDoc string) bool {
	lower := strings.ToLower(htmlDoc)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "//www.")
}
