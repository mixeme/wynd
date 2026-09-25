package web_test

import (
	"io/fs"
	"testing"

	"gitea.mixdep.ru/mix/wynd/web"
)

func TestEmbed_includesChunkFiles(t *testing.T) {
	f, err := web.Build.Open("dist/index.html")
	if err != nil {
		t.Fatalf("open dist/index.html: %v", err)
	}
	f.Close()
	count := 0
	err = fs.WalkDir(web.Build, "dist/_app/immutable/chunks", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk chunks: %v", err)
	}
	if count == 0 {
		t.Fatal("no chunk files embedded")
	}
}
