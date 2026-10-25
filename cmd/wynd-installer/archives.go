//go:build dev || desktop || bindings

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
)

// Вкладка «Архивы» (план 45, кадры 11а–11г): объединить, разделить, лента
// из полного архива круга. Сервер не нужен — всё на этом компьютере.

// ArchiveFile — архив в списке 11б и 11в.
type ArchiveFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	// Error — строка у самого файла: «Мастерская не может прочитать этот
	// файл как архив Wynd» (владелец, 2026-10-24) или «Это архив другого круга».
	Error   string          `json:"error,omitempty"`
	Summary archive.Summary `json:"summary"`
}

// MergeInfo — список 11б и что получится.
type MergeInfo struct {
	Files   []ArchiveFile   `json:"files"`
	OK      bool            `json:"ok"`
	Summary archive.Summary `json:"summary"`
	// Suggest — имя файла по умолчанию: «Семья 2024–2025.zip».
	Suggest string `json:"suggest,omitempty"`
}

// YearPart — год архива на 11в.
type YearPart struct {
	Year    int             `json:"year"`
	Summary archive.Summary `json:"summary"`
}

// SplitInfo — архив на 11в и его годы.
type SplitInfo struct {
	File  ArchiveFile `json:"file"`
	Years []YearPart  `json:"years"`
}

// ArchiveResult — чем кончилась сборка.
type ArchiveResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message,omitempty"`
	Files   []string `json:"files,omitempty"`
}

const unreadable = "Мастерская не может прочитать этот файл как архив Wynd"

func openSources(paths []string) ([]*archive.Source, []ArchiveFile) {
	var srcs []*archive.Source
	var files []ArchiveFile
	for _, p := range paths {
		f := ArchiveFile{Path: p, Name: filepath.Base(p)}
		s, err := archive.OpenSource(p)
		if err != nil {
			f.Error = unreadable
		} else {
			f.OK = true
			_, f.Summary = archive.PlanSplit(s, "0000-00-00", "9999-99-99")
			srcs = append(srcs, s)
		}
		files = append(files, f)
	}
	return srcs, files
}

func closeAll(srcs []*archive.Source) {
	for _, s := range srcs {
		_ = s.Close()
	}
}

func yearsLabel(from, to string) string {
	if len(from) < 4 || len(to) < 4 {
		return ""
	}
	if from[:4] == to[:4] {
		return from[:4]
	}
	return from[:4] + "–" + to[:4]
}

// PickArchives — выбор архивов для объединения.
func (a *App) PickArchives() []string {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Архивы одного круга",
		Filters: []runtime.FileFilter{{DisplayName: "Архивы Wynd (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil {
		return nil
	}
	return paths
}

// PickArchive — один файл: архив (zip) или полный архив круга (wynd).
func (a *App) PickArchive(full bool) string {
	filter := runtime.FileFilter{DisplayName: "Архивы Wynd (*.zip)", Pattern: "*.zip"}
	if full {
		filter = runtime.FileFilter{DisplayName: "Полный архив круга (*.wynd)", Pattern: "*.wynd"}
	}
	path, _ := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Filters: []runtime.FileFilter{filter}})
	return path
}

// PickSave — куда сохранить ZIP.
func (a *App) PickSave(name string) string {
	path, _ := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: name,
		Filters:         []runtime.FileFilter{{DisplayName: "ZIP", Pattern: "*.zip"}},
	})
	return path
}

// PickFolder — папка для частей деления.
func (a *App) PickFolder() string {
	path, _ := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
	return path
}

// MergeInfo — что получится из выбранных архивов (11б).
func (a *App) MergeInfo(paths []string) MergeInfo {
	srcs, files := openSources(paths)
	defer closeAll(srcs)
	info := MergeInfo{Files: files}
	if len(srcs) < 2 {
		return info
	}
	// Архив другого круга — строка у него, остальные объединяются.
	circle := srcs[0].Meta.CircleID
	var same []*archive.Source
	for i, f := range files {
		if !f.OK {
			continue
		}
		s := srcByPath(srcs, f.Path)
		if s.Meta.CircleID != circle {
			files[i].OK = false
			files[i].Error = "Это архив другого круга"
			continue
		}
		same = append(same, s)
	}
	info.Files = files
	if len(same) < 2 {
		return info
	}
	_, sum, err := archive.PlanMerge(same)
	if err != nil {
		return info
	}
	info.OK = allOK(files)
	info.Summary = sum
	info.Suggest = sum.CircleName + " " + yearsLabel(sum.From, sum.To) + ".zip"
	return info
}

func srcByPath(srcs []*archive.Source, path string) *archive.Source {
	for _, s := range srcs {
		if s.Path == path {
			return s
		}
	}
	return nil
}

func allOK(files []ArchiveFile) bool {
	for _, f := range files {
		if !f.OK {
			return false
		}
	}
	return true
}

// Merge объединяет архивы в один файл out.
func (a *App) Merge(paths []string, out string) ArchiveResult {
	srcs, files := openSources(paths)
	defer closeAll(srcs)
	if !allOK(files) || len(srcs) < 2 {
		return ArchiveResult{Message: unreadable}
	}
	picks, _, err := archive.PlanMerge(srcs)
	if err != nil {
		if errors.Is(err, archive.ErrOtherCircle) {
			return ArchiveResult{Message: "Это архив другого круга"}
		}
		return ArchiveResult{Message: err.Error()}
	}
	if err := archive.AssembleFile(out, picks, newestSource(srcs)); err != nil {
		return ArchiveResult{Message: "Не получилось записать файл: " + err.Error()}
	}
	return ArchiveResult{OK: true, Files: []string{out}}
}

func newestSource(srcs []*archive.Source) *archive.Source {
	best := srcs[0]
	for _, s := range srcs[1:] {
		if s.Meta.CutoffDate >= best.Meta.CutoffDate {
			best = s
		}
	}
	return best
}

// SplitInfo — годы архива (11в).
func (a *App) SplitInfo(path string) SplitInfo {
	srcs, files := openSources([]string{path})
	defer closeAll(srcs)
	info := SplitInfo{File: files[0]}
	if len(srcs) == 0 {
		return info
	}
	for _, y := range archive.Years(srcs[0]) {
		_, sum := archive.PlanSplit(srcs[0], fmt.Sprintf("%d-01-01", y), fmt.Sprintf("%d-12-31", y))
		info.Years = append(info.Years, YearPart{Year: y, Summary: sum})
	}
	return info
}

// Split делит архив: по годам (years) или за срок (from–to), в папку dir.
func (a *App) Split(path string, years []int, from, to, dir string) ArchiveResult {
	srcs, _ := openSources([]string{path})
	defer closeAll(srcs)
	if len(srcs) == 0 {
		return ArchiveResult{Message: unreadable}
	}
	src := srcs[0]
	type part struct{ from, to, label string }
	var parts []part
	if len(years) > 0 {
		for _, y := range years {
			parts = append(parts, part{fmt.Sprintf("%d-01-01", y), fmt.Sprintf("%d-12-31", y), fmt.Sprint(y)})
		}
	} else if from != "" && to != "" {
		parts = append(parts, part{from, to, from + "–" + to})
	}
	res := ArchiveResult{OK: true}
	for _, p := range parts {
		picks, _ := archive.PlanSplit(src, p.from, p.to)
		if len(picks) == 0 {
			continue
		}
		out := filepath.Join(dir, src.Meta.CircleName+" "+p.label+".zip")
		if err := archive.AssembleFile(out, picks, src); err != nil {
			return ArchiveResult{Message: "Не получилось записать файл: " + err.Error(), Files: res.Files}
		}
		res.Files = append(res.Files, out)
	}
	return res
}

// FullInfo — нужен ли пароль полному архиву (11г).
func (a *App) FullInfo(path string) map[string]any {
	st, err := os.Stat(path)
	if err != nil {
		return map[string]any{"ok": false}
	}
	return map[string]any{"ok": true, "name": filepath.Base(path), "bytes": st.Size(), "encrypted": archive.IsEncryptedFull(path)}
}

// Readable делает из полного архива ленту для чтения (11г).
func (a *App) Readable(path, password, out string) ArchiveResult {
	err := archive.ReadableFromFull(path, password, out)
	switch {
	case errors.Is(err, archive.ErrPassword):
		return ArchiveResult{Message: "Пароль не подошёл"}
	case errors.Is(err, archive.ErrNoMeta):
		return ArchiveResult{Message: "Мастерская не может прочитать этот файл как полный архив круга"}
	case err != nil:
		return ArchiveResult{Message: "Не получилось записать файл: " + err.Error()}
	}
	return ArchiveResult{OK: true, Files: []string{out}}
}

// ShowInFolder открывает папку с файлом.
func (a *App) ShowInFolder(path string) {
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(filepath.Dir(path)))
}
