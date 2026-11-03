package archive_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// A21: the index keeps fixed metadata, not another copy of media; ZIP CRCs,
// arbitrary boundaries and a reloaded index produce exactly the same bytes.
func TestFullLayoutRandomAccessAndReload(t *testing.T) {
	in := fullInput(t)
	payload := bytes.Repeat([]byte("media-source-unchanged-0123456789"), 40000)
	blob, err := in.Readable.Blobs.PutGenerated(t.Context(), "acc1", "application/octet-stream", "file.bin", payload)
	if err != nil {
		t.Fatal(err)
	}
	in.Data.BlobIDs = []string{blob.ID}
	in.Data.Tables = append(in.Data.Tables, chronicle.FullArchiveTable{Name: "blobs", Columns: []string{"id", "size_bytes", "sha256"}, Rows: [][]any{{blob.ID, blob.SizeBytes, blob.SHA256}}})
	plan, err := archive.BuildFullLayout(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > len(payload)/8 {
		t.Fatalf("index copied media: %d vs %d", len(raw), len(payload))
	}
	reader, err := archive.OpenFullLayout(t.Context(), in.Readable.Blobs, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	whole, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(whole)) != plan.Size {
		t.Fatalf("length: %d vs %d", len(whole), plan.Size)
	}
	z, err := zip.NewReader(bytes.NewReader(whole), plan.Size)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range z.File {
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("%s CRC: %v", entry.Name, err)
		}
		if entry.Name == "media/"+blob.ID+".bin" {
			found = true
			if !bytes.Equal(b, payload) {
				t.Fatal("media changed")
			}
		}
	}
	if !found {
		t.Fatal("missing media entry")
	}
	var restored archive.FullLayout
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	resumed, err := archive.OpenFullLayout(t.Context(), in.Readable.Blobs, &restored)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	for _, part := range restored.Parts {
		for _, offset := range []int64{max(0, part.Offset-1), part.Offset, part.Offset + max(0, part.Size-1)} {
			got := make([]byte, min(int64(71), restored.Size-offset))
			if _, err := resumed.ReadAt(got, offset); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, whole[offset:offset+int64(len(got))]) {
				t.Fatalf("boundary at %d", offset)
			}
		}
	}
	if _, err := resumed.Seek(-17, io.SeekEnd); err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(resumed)
	if err != nil || !bytes.Equal(tail, whole[len(whole)-17:]) {
		t.Fatalf("end seek: %v", err)
	}
	if _, err := resumed.Seek(-1, io.SeekStart); err == nil {
		t.Fatal("negative seek accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled, err := archive.OpenFullLayout(ctx, in.Readable.Blobs, &restored)
	if err != nil {
		t.Fatal(err)
	}
	defer canceled.Close()
	if _, err := canceled.Read(make([]byte, 1)); err == nil {
		t.Fatal("cancellation ignored")
	}
	info, err := in.Readable.Blobs.OpenBlob(t.Context(), blob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(info.Path, bytes.Repeat([]byte{'x'}, len(payload)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.BuildFullLayout(t.Context(), in); err == nil {
		t.Fatal("changed source checksum accepted")
	}
}

func TestFullLayoutRejectsBrokenBoundaries(t *testing.T) {
	in := fullInput(t)
	for _, plan := range []*archive.FullLayout{
		{Version: 2},
		{Version: 1, Size: 3, Parts: []archive.LayoutPart{{Offset: 1, Size: 3, Data: []byte("abc")}}},
		{Version: 1, Size: 3, Parts: []archive.LayoutPart{{Size: 3, Data: []byte("ab")}}},
		{Version: 1, Size: 3, Parts: []archive.LayoutPart{{Size: 3, BlobID: "b", Data: []byte("abc")}}},
	} {
		if _, err := archive.OpenFullLayout(t.Context(), in.Readable.Blobs, plan); err == nil {
			t.Fatalf("invalid index accepted: %+v", plan)
		}
	}
}
