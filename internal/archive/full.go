package archive

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Полный архив круга (план 51, A1): файл «.wynd» — ZIP с машинной частью
// (manifest.json, data/<таблица>.json), всеми файлами круга (media/) и
// читаемой лентой (index.html, posts/), как у архива участника. Без
// шифрования файл открывается любым архиватором; оживляет круг Wynd.

// FullFormat и FullFormatVersion — что за файл и какой версии его разметка.
// Версия растёт, когда меняется сама разметка архива; схема базы — отдельно
// (manifest.schema_version), её переводит оживление. Версия 2 — та же
// разметка, но номер схемы по новой нумерации миграций (с 0.34.0, план 46,
// C45): в версии 1 он по прежней (34–36), в версии 2 — с 1.
const (
	FullFormat        = "wynd-circle"
	FullFormatVersion = 2
)

// FullFormatKnown — разметка, которую этот сервер читает.
func FullFormatKnown(v int) bool { return v == 1 || v == 2 }

// FullManifest — manifest.json полного архива.
type FullManifest struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"format_version"`
	WyndVersion   string    `json:"wynd_version"`
	SchemaVersion int       `json:"schema_version"`
	Server        string    `json:"server"`
	CircleID      string    `json:"circle_id"`
	CircleName    string    `json:"circle_name"`
	SnapshotAt    time.Time `json:"snapshot_at"`
	Posts         int       `json:"posts"`
	Tables        []string  `json:"tables"`
}

// FullInput — всё для сборки полного архива.
type FullInput struct {
	Manifest FullManifest
	Data     *chronicle.FullArchiveData
	// Readable — лента для глаз: записи, аватары, названия дней, шрифты.
	// CutoffDate в ней — день снимка (шапка «на дату»).
	Readable BuildInput
}

// tableJSON — data/<таблица>.json: колонки и строки как в схеме базы.
type tableJSON struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// BuildFullArchive пишет полный архив круга в w потоком: память не растёт с
// размером круга, медиа — без сжатия.
func BuildFullArchive(ctx context.Context, w io.Writer, in FullInput) error {
	if in.Data == nil || in.Readable.Blobs == nil {
		return fmt.Errorf("archive: full archive without data")
	}
	zw := zip.NewWriter(w)
	m := in.Manifest
	m.Format, m.FormatVersion = FullFormat, FullFormatVersion
	m.CircleID, m.CircleName, m.SnapshotAt = in.Data.CircleID, in.Data.CircleName, in.Data.SnapshotAt
	m.Posts = in.Data.Posts
	m.Tables = m.Tables[:0]
	for _, t := range in.Data.Tables {
		m.Tables = append(m.Tables, t.Name)
	}
	if err := writeJSON(zw, "manifest.json", m); err != nil {
		return err
	}
	for _, t := range in.Data.Tables {
		rows := t.Rows
		if rows == nil {
			rows = [][]any{}
		}
		if err := writeJSON(zw, "data/"+t.Name+".json", tableJSON{Columns: t.Columns, Rows: rows}); err != nil {
			return err
		}
	}
	st := style{hex: circleHex(in.Data.Color), full: true}
	if err := buildReadable(ctx, zw, in.Readable, in.Data.BlobIDs, st); err != nil {
		return err
	}
	return zw.Close()
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeZipFile(zw, name, data)
}

// FullSize — длина полного архива без шифрования. Холостая сборка: медиа не
// читаются, но JSON и HTML сжимаются по-настоящему — длина точная.
func FullSize(ctx context.Context, in FullInput) (int64, error) {
	in.Readable.SizeOnly = true
	var cw countWriter
	if err := BuildFullArchive(ctx, &cw, in); err != nil {
		return 0, err
	}
	return cw.n, nil
}
