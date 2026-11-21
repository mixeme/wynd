package store

import (
	"fmt"
	"sort"
	"testing"
)

// Форма базовой схемы равна форме прежней цепочки 0001–0017 (C115): те же
// таблицы, индексы и триггеры, у каждой таблицы те же столбцы. Иначе свежая
// установка и переведённая база расходились бы в поведении.
func TestSquashSchemaMatchesFormerChain(t *testing.T) {
	fresh := freshBase(t)
	former := legacyBaseAt(t, squashedTop)

	objects := func(s *SQLite) []string {
		t.Helper()
		rows, err := s.db.Query(`SELECT type, name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var typ, name string
			if err := rows.Scan(&typ, &name); err != nil {
				t.Fatal(err)
			}
			out = append(out, typ+" "+name)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		sort.Strings(out)
		return out
	}

	got, want := objects(fresh), objects(former)
	if len(got) != len(want) {
		t.Fatalf("объектов в базовой схеме %d, в прежней цепочке %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("объект %d: базовая схема %q, прежняя цепочка %q", i, got[i], want[i])
		}
	}

	columns := func(s *SQLite, table string) []string {
		t.Helper()
		rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY name`, table)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatal(err)
			}
			out = append(out, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	tables := []string{}
	for _, obj := range got {
		var typ, name string
		if _, err := fmt.Sscanf(obj, "%s %s", &typ, &name); err != nil {
			t.Fatal(err)
		}
		if typ == "table" {
			tables = append(tables, name)
		}
	}
	for _, table := range tables {
		a, b := columns(fresh, table), columns(former, table)
		if len(a) != len(b) {
			t.Fatalf("%s: столбцов %d, в прежней цепочке %d", table, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s: столбец %d — %q против %q", table, i, a[i], b[i])
			}
		}
	}

	// Засев тоже часть формы: в поднятой цепочке есть строка настроек
	// сервера — свежая база обязана получить ту же (C115; пропуск INSERT
	// из 0001 лишил бы сервер строки bootstrapped=0).
	counts := func(s *SQLite, table string) int {
		t.Helper()
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM "` + table + `"`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, table := range tables {
		if table == "schema_migrations" {
			continue // отметки миграций — их свёртка как раз переписывает
		}
		a, b := counts(fresh, table), counts(former, table)
		if a != b {
			t.Fatalf("%s: строк %d, в прежней цепочке %d", table, a, b)
		}
	}
}
