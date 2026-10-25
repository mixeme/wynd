package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Работа с архивами в Мастерской (план 45, кадры 11а–11г): объединить
// архивы одного круга, разделить архив по годам или сроку, сделать из
// полного архива круга (.wynd) ленту для чтения. Сервер не нужен: всё
// собирается из того, что лежит в самих архивах, — по wynd-archive.json и
// кускам ленты между метками (meta.go). Страницы и файлы копируются как
// есть; заново пишутся только лента, оглавление и метаданные.

var (
	// ErrNoMeta — файл не читается как архив Wynd с метаданными: чужой ZIP,
	// повреждённый файл или архив старше метаданных.
	ErrNoMeta = errors.New("archive: not a Wynd archive with metadata")
	// ErrOtherCircle — архивы разных кругов вместе не объединяются.
	ErrOtherCircle = errors.New("archive: archive of another circle")
)

// Source — открытый архив с метаданными.
type Source struct {
	Path  string
	Meta  Meta
	zr    *zip.ReadCloser
	files map[string]*zip.File
	feed  map[string]string
	toc   map[string]string
}

// OpenSource открывает архив; без wynd-archive.json — ErrNoMeta.
func OpenSource(path string) (*Source, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, ErrNoMeta
	}
	s := &Source{Path: path, zr: zr, files: map[string]*zip.File{}}
	for _, f := range zr.File {
		s.files[f.Name] = f
	}
	raw, err := s.read(MetaName)
	if err != nil || json.Unmarshal(raw, &s.Meta) != nil || s.Meta.Format != MetaFormat || s.Meta.FormatVersion > MetaFormatVersion {
		zr.Close()
		return nil, ErrNoMeta
	}
	return s, nil
}

// Close закрывает файл архива.
func (s *Source) Close() error { return s.zr.Close() }

func (s *Source) read(name string) ([]byte, error) {
	f, ok := s.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

var (
	feedBlockRe = regexp.MustCompile(`(?s)<!-- wynd-post (\S+) -->(.*?)<!-- /wynd-post -->`)
	tocBlockRe  = regexp.MustCompile(`(?s)<!-- wynd-toc (\S+) -->(.*?)<!-- /wynd-toc -->`)
)

func (s *Source) blocks() error {
	if s.feed != nil {
		return nil
	}
	s.feed, s.toc = map[string]string{}, map[string]string{}
	index, err := s.read("index.html")
	if err != nil {
		return ErrNoMeta
	}
	for _, m := range feedBlockRe.FindAllSubmatch(index, -1) {
		s.feed[string(m[1])] = string(m[2])
	}
	if toc, err := s.read("posts/index.html"); err == nil {
		for _, m := range tocBlockRe.FindAllSubmatch(toc, -1) {
			s.toc[string(m[1])] = string(m[2])
		}
	}
	return nil
}

func (s *Source) size(names ...string) int64 {
	var n int64
	for _, name := range names {
		if f, ok := s.files[name]; ok {
			n += int64(f.UncompressedSize64)
		}
	}
	return n
}

// Pick — запись, взятая из одного из архивов.
type Pick struct {
	Post MetaPost
	Src  *Source
}

// Summary — что получится (11б, 11в): даты по дню записи, записи, размер.
type Summary struct {
	CircleName string `json:"circle_name"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Posts      int    `json:"posts"`
	// Duplicates — записи, что были в нескольких архивах; взята та, где
	// архив собран позже (её могли поправить между архивациями).
	Duplicates int   `json:"duplicates"`
	Bytes      int64 `json:"bytes"`
}

// PlanMerge — записи объединения: по одной на id, из архива с поздней отсечкой.
func PlanMerge(srcs []*Source) ([]Pick, Summary, error) {
	if len(srcs) == 0 {
		return nil, Summary{}, ErrNoMeta
	}
	circle := srcs[0].Meta.CircleID
	byID := map[string]Pick{}
	dup := 0
	for _, s := range srcs {
		if s.Meta.CircleID != circle {
			return nil, Summary{}, fmt.Errorf("%w: %s", ErrOtherCircle, filepath.Base(s.Path))
		}
		for _, p := range s.Meta.Posts {
			prev, seen := byID[p.ID]
			if seen {
				dup++
				if prev.Src.Meta.CutoffDate > s.Meta.CutoffDate {
					continue
				}
			}
			byID[p.ID] = Pick{Post: p, Src: s}
		}
	}
	picks := make([]Pick, 0, len(byID))
	for _, p := range byID {
		picks = append(picks, p)
	}
	return orderPicks(picks), summarize(picks, newest(srcs), dup), nil
}

// PlanSplit — записи архива, чей день записи в [from, to] (ГГГГ-ММ-ДД, включительно).
func PlanSplit(src *Source, from, to string) ([]Pick, Summary) {
	var picks []Pick
	for _, p := range src.Meta.Posts {
		if p.EntryDate >= from && p.EntryDate <= to {
			picks = append(picks, Pick{Post: p, Src: src})
		}
	}
	return orderPicks(picks), summarize(picks, src, 0)
}

// Years — годы записей архива по дню записи, по возрастанию (11в).
func Years(src *Source) []int {
	seen := map[int]bool{}
	for _, p := range src.Meta.Posts {
		if y, err := strconv.Atoi(firstN(p.EntryDate, 4)); err == nil {
			seen[y] = true
		}
	}
	var out []int
	for y := range seen {
		out = append(out, y)
	}
	sort.Ints(out)
	return out
}

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func orderPicks(picks []Pick) []Pick {
	sort.SliceStable(picks, func(i, j int) bool {
		if !picks[i].Post.CreatedAt.Equal(picks[j].Post.CreatedAt) {
			return picks[i].Post.CreatedAt.After(picks[j].Post.CreatedAt)
		}
		return picks[i].Post.ID > picks[j].Post.ID
	})
	return picks
}

func newest(srcs []*Source) *Source {
	best := srcs[0]
	for _, s := range srcs[1:] {
		if s.Meta.CutoffDate >= best.Meta.CutoffDate {
			best = s
		}
	}
	return best
}

func summarize(picks []Pick, base *Source, dup int) Summary {
	sum := Summary{CircleName: base.Meta.CircleName, Posts: len(picks), Duplicates: dup}
	seen := map[string]bool{}
	for _, p := range picks {
		if sum.From == "" || p.Post.EntryDate < sum.From {
			sum.From = p.Post.EntryDate
		}
		if p.Post.EntryDate > sum.To {
			sum.To = p.Post.EntryDate
		}
		for _, name := range append([]string{p.Post.Page}, p.Post.Files...) {
			key := p.Src.Path + "\x00" + name
			if !seen[key] {
				seen[key] = true
				sum.Bytes += p.Src.size(name)
			}
		}
	}
	return sum
}

// Assemble пишет архив из выбранных записей: страницы и файлы — как есть,
// лента, оглавление и метаданные — заново. base даёт круг, цвет и шрифты.
func Assemble(w io.Writer, picks []Pick, base *Source) error {
	zw := zip.NewWriter(w)
	written := map[string]bool{}
	copyFile := func(src *Source, name string) error {
		if written[name] {
			return nil
		}
		f, ok := src.files[name]
		if !ok {
			return nil
		}
		written[name] = true
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, rc)
		return err
	}

	st := style{hex: circleHex(base.Meta.Color)}
	for _, f := range archiveFonts {
		if _, ok := base.files["fonts/"+f]; ok {
			if err := copyFile(base, "fonts/"+f); err != nil {
				return err
			}
			st.fonts = append(st.fonts, f)
		}
	}

	meta := Meta{
		Format: MetaFormat, FormatVersion: MetaFormatVersion,
		CircleID: base.Meta.CircleID, CircleName: base.Meta.CircleName, Color: base.Meta.Color,
		CutoffDate: base.Meta.CutoffDate, Posts: make([]MetaPost, 0, len(picks)),
	}
	days := map[string]bool{}
	used := map[*Source]bool{base: true}
	var feed, toc strings.Builder
	for _, p := range picks {
		if err := p.Src.blocks(); err != nil {
			return err
		}
		used[p.Src] = true
		if p.Src.Meta.CutoffDate > meta.CutoffDate {
			meta.CutoffDate = p.Src.Meta.CutoffDate
		}
		for _, name := range append([]string{p.Post.Page}, p.Post.Files...) {
			if err := copyFile(p.Src, name); err != nil {
				return err
			}
		}
		block, ok := p.Src.feed[p.Post.ID]
		if !ok {
			return fmt.Errorf("%w: %s has no feed block for %s", ErrNoMeta, filepath.Base(p.Src.Path), p.Post.ID)
		}
		feed.WriteString(MetaPostOpen(p.Post.ID))
		feed.WriteString(block)
		feed.WriteString(MetaPostClose)
		toc.WriteString(MetaTocOpen(p.Post.ID))
		if line, ok := p.Src.toc[p.Post.ID]; ok {
			toc.WriteString(line)
		} else {
			// Архив без меток оглавления — строка по метаданным.
			toc.WriteString("<li><a href=\"" + html.EscapeString(p.Post.ID) + ".html\">" +
				html.EscapeString(p.Post.AuthorName) + "</a><div class=\"meta\">" +
				entryDateHTML(p.Post.EntryDate, nil) + "</div></li>")
		}
		toc.WriteString(MetaTocClose)
		meta.Posts = append(meta.Posts, p.Post)
		days[p.Post.EntryDate] = true
	}
	// Лица и дни с названием — из тех архивов, откуда взяты записи.
	for s := range used {
		for identity, name := range s.Meta.Avatars {
			if meta.Avatars == nil {
				meta.Avatars = map[string]string{}
			}
			if _, have := meta.Avatars[identity]; have && s != base {
				continue
			}
			if err := copyFile(s, name); err != nil {
				return err
			}
			meta.Avatars[identity] = name
		}
		for _, d := range s.Meta.DayTitles {
			if days[d] {
				meta.DayTitles = appendOnce(meta.DayTitles, d)
			}
		}
	}
	sort.Strings(meta.DayTitles)

	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := writeZipFile(zw, MetaName, raw); err != nil {
		return err
	}
	var index bytes.Buffer
	{
		var b strings.Builder
		st.head(&b, meta.CircleName, "")
		st.header(&b, meta.CircleName, meta.CutoffDate)
		b.WriteString(feed.String())
		b.WriteString(st.pageEnd())
		index.WriteString(b.String())
	}
	if err := writeZipFile(zw, "index.html", index.Bytes()); err != nil {
		return err
	}
	var b strings.Builder
	st.head(&b, meta.CircleName+" — оглавление", "../")
	st.header(&b, meta.CircleName, meta.CutoffDate)
	b.WriteString("<a class=\"back\" href=\"../index.html\">← единая лента</a><ul class=\"toc\">")
	b.WriteString(toc.String())
	b.WriteString("</ul>")
	b.WriteString(st.pageEnd())
	if err := writeZipFile(zw, "posts/index.html", []byte(b.String())); err != nil {
		return err
	}
	return zw.Close()
}

func appendOnce(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// AssembleFile пишет архив в файл: сначала рядом, потом переименованием —
// оборванная сборка не оставит половину файла под нужным именем.
func AssembleFile(path string, picks []Pick, base *Source) error {
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := Assemble(f, picks, base); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// readableNames — что из полного архива нужно ленте: страницы, файлы,
// шрифты и метаданные; машинная часть (manifest.json, data/) — нет.
func readableName(name string) bool {
	return name == "index.html" || name == MetaName ||
		strings.HasPrefix(name, "posts/") || strings.HasPrefix(name, "media/") || strings.HasPrefix(name, "fonts/")
}

// ErrPassword — пароль зашифрованного архива не подошёл.
var ErrPassword = errors.New("archive: wrong password")

// ReadableFromFull делает из полного архива круга (.wynd) ленту для чтения
// (11г): ZIP того же вида, что личный архив, — весь круг. Зашифрованный
// файл расшифровывается паролем во временный рядом с результатом.
func ReadableFromFull(fullPath, password, outPath string) error {
	src := fullPath
	if !isZip(fullPath) {
		if password == "" {
			return ErrPassword
		}
		in, err := os.Open(fullPath)
		if err != nil {
			return err
		}
		defer in.Close()
		plain, err := Decrypt(in, password)
		if err != nil {
			return ErrPassword
		}
		tmp, err := os.CreateTemp(filepath.Dir(outPath), ".wynd-plain-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, plain); err != nil {
			tmp.Close()
			return ErrPassword
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		src = tmp.Name()
	}
	full, err := OpenFullArchive(src)
	if err != nil {
		return ErrNoMeta
	}
	full.Close()
	zr, err := zip.OpenReader(src)
	if err != nil {
		return ErrNoMeta
	}
	defer zr.Close()
	part := outPath + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		if !readableName(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			os.Remove(part)
			return err
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Store})
		if err == nil {
			_, err = io.Copy(fw, rc)
		}
		rc.Close()
		if err != nil {
			out.Close()
			os.Remove(part)
			return err
		}
	}
	if err := zw.Close(); err != nil {
		out.Close()
		os.Remove(part)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(part)
		return err
	}
	return os.Rename(part, outPath)
}

// IsEncryptedFull — нужен ли пароль: зашифрованный .wynd — не ZIP.
func IsEncryptedFull(path string) bool { return !isZip(path) }

func isZip(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return bytes.Equal(head, []byte("PK\x03\x04"))
}
