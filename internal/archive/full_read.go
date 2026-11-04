package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// FullReader читает открытый (без шифрования) полный архив на сервере —
// перенос круга (план 53): новый сервер забрал файл у прежнего и поднимает
// круг сам. Оживление из файла читает его на устройстве (wyndfile.ts).
type FullReader struct {
	zr        *zip.ReadCloser
	Manifest  FullManifest
	media     map[string]*zip.File
	limits    ReadLimits
	jsonBytes int64
	rows      int
}

// FullTable — data/<таблица>.json: колонки и строки, числа — float64, как
// после разбора JSON от устройства.
type FullTable struct {
	Columns []string
	Rows    [][]any
}

// OpenFullArchive открывает файл и читает manifest.json.
func OpenFullArchive(path string) (*FullReader, error) {
	return OpenFullArchiveWithLimits(path, ReadLimits{})
}

func OpenFullArchiveWithLimits(path string, limits ReadLimits) (*FullReader, error) {
	limits = limits.defaults()
	if err := checkFullDirectory(path, limits); err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	r := &FullReader{zr: zr, media: map[string]*zip.File{}, limits: limits}
	seen := map[string]bool{}
	var mediaBytes uint64
	for _, f := range zr.File {
		if seen[f.Name] {
			zr.Close()
			return nil, fmt.Errorf("archive: duplicate ZIP entry")
		}
		seen[f.Name] = true
		if rest, ok := strings.CutPrefix(f.Name, "media/"); ok {
			id, _, _ := strings.Cut(rest, ".")
			if _, duplicate := r.media[id]; duplicate || f.UncompressedSize64 > limits.MediaBytes-mediaBytes {
				zr.Close()
				return nil, fmt.Errorf("archive: media exceeds limits or repeats an ID")
			}
			mediaBytes += f.UncompressedSize64
			r.media[id] = f
		}
	}
	if err := r.readJSON("manifest.json", &r.Manifest); err != nil {
		zr.Close()
		return nil, err
	}
	if r.Manifest.Format != FullFormat || !FullFormatKnown(r.Manifest.FormatVersion) || r.Manifest.Posts < 0 {
		zr.Close()
		return nil, fmt.Errorf("archive: not a circle archive (%q v%d)", r.Manifest.Format, r.Manifest.FormatVersion)
	}
	return r, nil
}

func (r *FullReader) Close() error { return r.zr.Close() }

func (r *FullReader) readJSON(name string, v any) error {
	limit := r.limits.TableBytes
	if name == "manifest.json" {
		limit = r.limits.ManifestBytes
	}
	limit = min(limit, r.limits.JSONBytes-r.jsonBytes)
	if limit <= 0 {
		return fmt.Errorf("archive: JSON budget exhausted")
	}
	f, err := r.zr.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	lr := &io.LimitedReader{R: f, N: limit + 1}
	decoder := json.NewDecoder(lr)
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var extra any
	err = decoder.Decode(&extra)
	if lr.N == 0 {
		return fmt.Errorf("archive: JSON exceeds limit")
	}
	if err != io.EOF {
		return fmt.Errorf("archive: trailing JSON or invalid ZIP data")
	}
	r.jsonBytes += limit + 1 - lr.N
	return nil
}

// Tables — все таблицы из manifest.tables.
func (r *FullReader) Tables() (map[string]FullTable, error) {
	out := map[string]FullTable{}
	for _, name := range r.Manifest.Tables {
		if _, duplicate := out[name]; duplicate || name == "" || strings.ContainsAny(name, "/\\.") {
			return nil, fmt.Errorf("archive: bad table name %q", name)
		}
		var raw rawFullTable
		if err := r.readJSON("data/"+name+".json", &raw); err != nil {
			return nil, fmt.Errorf("archive: table %s: %w", name, err)
		}
		t, err := decodeFullTable(raw, r.limits.Rows-r.rows)
		if err != nil {
			return nil, err
		}
		r.rows += len(t.Rows)
		if err := chronicle.ValidateArchiveTableShape(name, chronicle.FullArchiveTable{Columns: t.Columns, Rows: t.Rows}); err != nil {
			return nil, err
		}
		out[name] = FullTable{Columns: t.Columns, Rows: t.Rows}
	}
	return out, nil
}

// OpenMedia — файл круга по id блоба архива.
func (r *FullReader) OpenMedia(blobID string) (io.ReadCloser, error) {
	f, ok := r.media[blobID]
	if !ok {
		return nil, fmt.Errorf("archive: media %s missing", blobID)
	}
	return f.Open()
}
