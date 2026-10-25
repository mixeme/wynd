package archive

import (
	"encoding/json"
	"sort"
	"time"
)

// Метаданные читаемого архива (план 45, «Работа с архивами»; владелец,
// 2026-10-24): Мастерская объединяет и делит личные архивы, не разбирая
// страниц. Текстов здесь нет — они на страницах; здесь только где что лежит.
// Каждая запись в index.html — между MetaPostOpen и MetaPostClose: при
// объединении лента склеивается из этих кусков как есть.
//
// В файле нет времени сборки: холостая сборка (Size) и настоящая должны дать
// архив одной длины. Чей кусок свежее при объединении — решает CutoffDate.

// MetaName — файл метаданных в корне архива.
const MetaName = "wynd-archive.json"

// MetaFormat — формат файла метаданных; MetaFormatVersion растёт, когда
// старая Мастерская перестаёт понимать новые архивы.
const (
	MetaFormat        = "wynd-readable-archive"
	MetaFormatVersion = 1
)

// MetaPostOpen и MetaPostClose обрамляют запись в ленте index.html.
func MetaPostOpen(postID string) string { return "<!-- wynd-post " + postID + " -->" }

const MetaPostClose = "<!-- /wynd-post -->"

// MetaTocOpen и MetaTocClose обрамляют строку оглавления posts/index.html:
// объединённое оглавление склеивается так же, как лента.
func MetaTocOpen(postID string) string { return "<!-- wynd-toc " + postID + " -->" }

const MetaTocClose = "<!-- /wynd-toc -->"

// Meta — wynd-archive.json.
type Meta struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	CircleID      string `json:"circle_id"`
	CircleName    string `json:"circle_name"`
	Color         string `json:"color,omitempty"`
	// CutoffDate — до какого дня собран архив: из двух архивов одного круга
	// запись берётся из того, где этот день позже.
	CutoffDate string     `json:"cutoff_date"`
	Posts      []MetaPost `json:"posts"`
	// Avatars — лица авторов: id лица → файл в архиве.
	Avatars map[string]string `json:"avatars,omitempty"`
	// DayTitles — дни с названием: при делении по годам их год виден сразу.
	DayTitles []string `json:"day_titles,omitempty"`
}

// MetaPost — одна запись: где её страница, какие файлы её.
type MetaPost struct {
	ID        string    `json:"id"`
	EntryDate string    `json:"entry_date"`
	CreatedAt time.Time `json:"created_at"`
	AuthorID  string    `json:"author_id"`
	// AuthorName — имя автора: строка оглавления, если в архиве нет её меток.
	AuthorName string `json:"author_name,omitempty"`
	Page       string `json:"page"`
	// Files — файлы записи и её комментариев в архиве (media/…).
	Files    []string `json:"files,omitempty"`
	Comments int      `json:"comments"`
}

func buildMeta(in BuildInput, mediaNames map[string]string) ([]byte, error) {
	m := Meta{
		Format: MetaFormat, FormatVersion: MetaFormatVersion,
		CircleID: in.CircleID, CircleName: in.CircleName, Color: in.Color, CutoffDate: in.CutoffDate,
		Posts: make([]MetaPost, 0, len(in.Posts)),
	}
	for _, fp := range in.Posts {
		mp := MetaPost{
			ID: fp.Post.ID, EntryDate: fp.Post.EntryDate, CreatedAt: fp.Post.CreatedAt.UTC(),
			AuthorID: fp.Post.IdentityID, AuthorName: fp.Post.AuthorName, Page: "posts/" + fp.Post.ID + ".html", Comments: len(fp.Comments),
		}
		seen := map[string]bool{}
		add := func(blobID string) {
			if name, ok := mediaNames[blobID]; ok && !seen[name] {
				seen[name] = true
				mp.Files = append(mp.Files, name)
			}
		}
		for _, md := range fp.Media {
			add(md.BlobID)
			add(md.AudioCoverBlobID)
			add(md.VideoPosterBlobID)
		}
		for _, c := range fp.Comments {
			for _, md := range c.Media {
				add(md.BlobID)
			}
		}
		m.Posts = append(m.Posts, mp)
	}
	for identity, blobID := range in.Avatars {
		if name, ok := mediaNames[blobID]; ok {
			if m.Avatars == nil {
				m.Avatars = map[string]string{}
			}
			m.Avatars[identity] = name
		}
	}
	for day := range in.DayTitles {
		m.DayTitles = append(m.DayTitles, day)
	}
	sort.Strings(m.DayTitles)
	return json.Marshal(m)
}
