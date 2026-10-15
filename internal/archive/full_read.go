package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FullReader читает открытый (без шифрования) полный архив на сервере —
// перенос круга (план 53): новый сервер забрал файл у прежнего и поднимает
// круг сам. Оживление из файла читает его на устройстве (wyndfile.ts).
type FullReader struct {
	zr       *zip.ReadCloser
	Manifest FullManifest
	media    map[string]*zip.File
}

// FullTable — data/<таблица>.json: колонки и строки, числа — float64, как
// после разбора JSON от устройства.
type FullTable struct {
	Columns []string
	Rows    [][]any
}

// OpenFullArchive открывает файл и читает manifest.json.
func OpenFullArchive(path string) (*FullReader, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	r := &FullReader{zr: zr, media: map[string]*zip.File{}}
	for _, f := range zr.File {
		if rest, ok := strings.CutPrefix(f.Name, "media/"); ok {
			id, _, _ := strings.Cut(rest, ".")
			r.media[id] = f
		}
	}
	if err := r.readJSON("manifest.json", &r.Manifest); err != nil {
		zr.Close()
		return nil, err
	}
	if r.Manifest.Format != FullFormat || r.Manifest.FormatVersion != FullFormatVersion {
		zr.Close()
		return nil, fmt.Errorf("archive: not a circle archive (%q v%d)", r.Manifest.Format, r.Manifest.FormatVersion)
	}
	return r, nil
}

func (r *FullReader) Close() error { return r.zr.Close() }

func (r *FullReader) readJSON(name string, v any) error {
	f, err := r.zr.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}

// Tables — все таблицы из manifest.tables.
func (r *FullReader) Tables() (map[string]FullTable, error) {
	out := map[string]FullTable{}
	for _, name := range r.Manifest.Tables {
		if strings.ContainsAny(name, "/\\.") {
			return nil, fmt.Errorf("archive: bad table name %q", name)
		}
		var t tableJSON
		if err := r.readJSON("data/"+name+".json", &t); err != nil {
			return nil, fmt.Errorf("archive: table %s: %w", name, err)
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
