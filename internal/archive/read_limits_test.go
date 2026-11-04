package archive

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func limitTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFullArchiveDecompressionBudgets(t *testing.T) {
	manifest := `{"format":"wynd-circle","format_version":2,"tables":["posts","accounts"]}`
	valid := `{"columns":[],"rows":[]}`
	for name, tc := range map[string]struct {
		files  map[string]string
		limits ReadLimits
	}{
		"compressed table": {map[string]string{"manifest.json": manifest, "data/posts.json": `{"columns":["body"],"rows":[["` + strings.Repeat("x", 8192) + `"]]}`}, ReadLimits{TableBytes: 128}},
		"aggregate JSON":   {map[string]string{"manifest.json": manifest, "data/posts.json": valid, "data/accounts.json": valid}, ReadLimits{JSONBytes: int64(len(manifest) + 2*len(valid) - 1)}},
		"manifest":         {map[string]string{"manifest.json": manifest}, ReadLimits{ManifestBytes: 16}},
		"media":            {map[string]string{"manifest.json": manifest, "media/id.bin": strings.Repeat("x", 8192)}, ReadLimits{MediaBytes: 32}},
		"trailing JSON":    {map[string]string{"manifest.json": manifest, "data/posts.json": valid + ` {"extra":true}`}, ReadLimits{}},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := OpenFullArchiveWithLimits(limitTestZip(t, tc.files), tc.limits)
			if err == nil {
				defer r.Close()
				_, err = r.Tables()
			}
			if err == nil {
				t.Fatal("untrusted archive exceeded its budget")
			}
		})
	}
}

func TestFullArchiveDirectoryLimitsBeforeAllocation(t *testing.T) {
	files := map[string]string{"manifest.json": `{"format":"wynd-circle","format_version":2}`, "a": "a", "b": "b"}
	path := limitTestZip(t, files)
	if _, err := OpenFullArchiveWithLimits(path, ReadLimits{Entries: 2}); err == nil {
		t.Fatal("accepted too many entries")
	}
	if _, err := OpenFullArchiveWithLimits(path, ReadLimits{DirectoryBytes: 8}); err == nil {
		t.Fatal("accepted oversized directory")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(data[len(data)-12:len(data)-10], 1) // forged advertised count
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFullArchive(path); err == nil {
		t.Fatal("trusted a forged entry count")
	}
}

func TestFullArchiveZIP64Directory(t *testing.T) {
	path := limitTestZip(t, map[string]string{"manifest.json": `{"format":"wynd-circle","format_version":2}`})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	end := append([]byte(nil), data[len(data)-22:]...)
	count := binary.LittleEndian.Uint16(end[10:12])
	size, offset := binary.LittleEndian.Uint32(end[12:16]), binary.LittleEndian.Uint32(end[16:20])
	var record [56]byte
	binary.LittleEndian.PutUint32(record[:4], 0x06064b50)
	binary.LittleEndian.PutUint64(record[4:12], 44)
	binary.LittleEndian.PutUint16(record[12:14], 45)
	binary.LittleEndian.PutUint16(record[14:16], 45)
	binary.LittleEndian.PutUint64(record[24:32], uint64(count))
	binary.LittleEndian.PutUint64(record[32:40], uint64(count))
	binary.LittleEndian.PutUint64(record[40:48], uint64(size))
	binary.LittleEndian.PutUint64(record[48:56], uint64(offset))
	var locator [20]byte
	binary.LittleEndian.PutUint32(locator[:4], 0x07064b50)
	binary.LittleEndian.PutUint64(locator[8:16], uint64(len(data)-22))
	binary.LittleEndian.PutUint32(locator[16:20], 1)
	binary.LittleEndian.PutUint16(end[8:10], 0xffff)
	binary.LittleEndian.PutUint16(end[10:12], 0xffff)
	binary.LittleEndian.PutUint32(end[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(end[16:20], 0xffffffff)
	data = append(data[:len(data)-22], record[:]...)
	data = append(data, locator[:]...)
	data = append(data, end...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := OpenFullArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}

func FuzzFullArchiveReadLimits(f *testing.F) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, _ := z.Create("manifest.json")
	_, _ = w.Write([]byte(`{"format":"wynd-circle","format_version":2,"tables":["posts"]}`))
	w, _ = z.Create("data/posts.json")
	_, _ = w.Write([]byte(`{"columns":["body"],"rows":[["sample"]]}`))
	_ = z.Close()
	f.Add(buf.Bytes())
	f.Add([]byte("PK\x05\x06"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128<<10 {
			return
		}
		path := filepath.Join(t.TempDir(), "input.zip")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		r, err := OpenFullArchiveWithLimits(path, ReadLimits{Entries: 16, DirectoryBytes: 4096, ManifestBytes: 1024, TableBytes: 1024, JSONBytes: 2048, MediaBytes: 4096})
		if err == nil {
			defer r.Close()
			_, _ = r.Tables()
		}
	})
}

func TestFullTableRejectsNestedCellsAndExcessRows(t *testing.T) {
	for _, rows := range []string{`[[{}]]`, `[[[]]]`, `[[]]`, `[["one","extra"]]`, `[["one"],["two"]]`} {
		if _, err := decodeFullTable(rawFullTable{Columns: []byte(`["body"]`), Rows: []byte(rows)}, 1); err == nil {
			t.Fatalf("accepted %s", rows)
		}
	}
	table, err := decodeFullTable(rawFullTable{Columns: []byte(`["body"]`), Rows: []byte(`[["original"]]`)}, 1)
	if err != nil || len(table.Rows) != 1 || table.Rows[0][0] != "original" {
		t.Fatalf("valid row rejected: %+v %v", table, err)
	}
}
