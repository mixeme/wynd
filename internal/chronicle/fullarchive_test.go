package chronicle_test

import (
	"slices"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (план 51): полный архив берёт круг целиком — все таблицы круга
// в порядке оживления, почту участников, все файлы — и ленту без отбора по
// видимости: запись, сделанная до прихода участника, в ней есть.
func TestFullArchiveDataTakesWholeCircle(t *testing.T) {
	e := newTestEnv(t)
	e.seedAccount("owner")
	e.seedAccount("bob")
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	early := e.post(circle.ID, "owner", "до Боба", "2026-08-01", e.at(0))
	e.seedBlob("photo1", "owner")
	e.attachPhoto(early.ID, "photo1")
	e.join(circle.ID, "bob", "Боб", e.at(1))
	e.post(circle.ID, "bob", "я пришёл", "2026-08-02", e.at(2))

	data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	rows := map[string]int{}
	for _, tb := range data.Tables {
		names = append(names, tb.Name)
		rows[tb.Name] = len(tb.Rows)
	}
	want := []string{"circles", "identities", "identity_names", "memberships", "membership_spans", "events",
		"posts", "post_media", "comments", "comment_media", "reactions", "days", "day_titles", "day_covers", "accounts", "blobs"}
	if !slices.Equal(names, want) {
		t.Fatalf("таблицы: %v", names)
	}
	if rows["circles"] != 1 || rows["posts"] != 2 || rows["identities"] != 2 || rows["accounts"] != 2 || rows["blobs"] != 1 || data.Posts != 2 {
		t.Fatalf("строки: %v", rows)
	}
	for _, tb := range data.Tables {
		if tb.Name != "accounts" {
			continue
		}
		email := slices.Index(tb.Columns, "email")
		var got []string
		for _, r := range tb.Rows {
			got = append(got, r[email].(string))
		}
		slices.Sort(got)
		if !slices.Equal(got, []string{"bob@test.local", "owner@test.local"}) {
			t.Fatalf("почта: %v", got)
		}
	}
	if !slices.Contains(data.BlobIDs, "photo1") {
		t.Fatalf("файлы круга: %v", data.BlobIDs)
	}
	feed, err := e.ch.FullArchiveFeed(e.ctx, circle.ID, e.at(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 2 {
		t.Fatalf("лента полного архива: %d записей", len(feed))
	}
	// Снимок до второй записи — её в ленте нет.
	feed, _ = e.ch.FullArchiveFeed(e.ctx, circle.ID, e.at(1))
	if len(feed) != 1 || feed[0].Post.ID != early.ID {
		t.Fatalf("лента до момента: %+v", feed)
	}
}
