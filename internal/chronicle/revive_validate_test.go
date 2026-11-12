package chronicle_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestReviveRejectsMalformedTablesBeforeWrites(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "original", "2026-08-01", e.at(1))
	e.seedBlob("photo", "owner")
	e.attachPhoto(p.ID, "photo")
	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	set := func(tables map[string]chronicle.FullArchiveTable, table, column string, value any) {
		tb := tables[table]
		i := slices.Index(tb.Columns, column)
		if i < 0 || len(tb.Rows) == 0 {
			t.Fatalf("fixture lacks %s.%s", table, column)
		}
		tb.Rows[0][i] = value
	}
	cases := map[string]func(map[string]chronicle.FullArchiveTable){
		"short row": func(m map[string]chronicle.FullArchiveTable) {
			tb := m["blobs"]
			tb.Rows[0] = []any{"short"}
			m["blobs"] = tb
		},
		"missing column": func(m map[string]chronicle.FullArchiveTable) {
			tb := m["accounts"]
			tb.Columns[1] = "created_at"
			m["accounts"] = tb
		},
		"duplicate column": func(m map[string]chronicle.FullArchiveTable) {
			tb := m["blobs"]
			tb.Columns[1] = tb.Columns[0]
			m["blobs"] = tb
		},
		"wrong type": func(m map[string]chronicle.FullArchiveTable) {
			set(m, "posts", "body", map[string]any{"unexpected": true})
		},
		"negative size":       func(m map[string]chronicle.FullArchiveTable) { set(m, "blobs", "size_bytes", float64(-1)) },
		"fractional size":     func(m map[string]chronicle.FullArchiveTable) { set(m, "blobs", "size_bytes", 1.5) },
		"overflow size":       func(m map[string]chronicle.FullArchiveTable) { set(m, "blobs", "size_bytes", float64(math.MaxInt64)) },
		"bad digest":          func(m map[string]chronicle.FullArchiveTable) { set(m, "blobs", "sha256", "not-a-hash") },
		"broken reference":    func(m map[string]chronicle.FullArchiveTable) { set(m, "post_media", "post_id", "missing") },
		"wrong event id type": func(m map[string]chronicle.FullArchiveTable) { set(m, "events", "id", float64(1)) },
		"duplicate primary key": func(m map[string]chronicle.FullArchiveTable) {
			tb := m["posts"]
			tb.Rows = append(tb.Rows, tb.Rows[0])
			m["posts"] = tb
		},
		"unknown table": func(m map[string]chronicle.FullArchiveTable) { m["admin_credentials"] = chronicle.FullArchiveTable{} },
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			tables, accounts, blobs := archiveOverJSON(t, data)
			corrupt(tables)
			_, err := e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{FormatVersion: 2, SchemaVersion: 15, Tables: tables, Accounts: accounts, BlobMap: blobs, ReviverID: "owner"})
			if !errors.Is(err, chronicle.ErrInvalid) {
				t.Fatalf("expected invalid archive, got %v", err)
			}
			// C81: отказ по данным круга несёт отдельный код — экран не должен
			// показывать подсказку про адрес сервера.
			if !errors.Is(err, chronicle.ErrArchiveTables) {
				t.Fatalf("expected archive-tables code, got %v", err)
			}
			var circles, blobCount int
			if err := e.ch.DB().QueryRow(`SELECT (SELECT COUNT(*) FROM circles), (SELECT COUNT(*) FROM blobs)`).Scan(&circles, &blobCount); err != nil {
				t.Fatal(err)
			}
			if circles != 1 || blobCount != 1 {
				t.Fatalf("side effects: circles=%d blobs=%d", circles, blobCount)
			}
		})
	}
}
