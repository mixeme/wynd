package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант: оценка личного архива совпадает с тем, что на самом деле уйдёт
// в ZIP. Оценка считается запросами, срез — сборкой; сойтись они обязаны, и
// сверяются здесь на круге с двумя отрезками видимости, аватарами, чужими
// комментариями и реакциями (аудит 2026-09-22).
func TestEstimateArchivePersonalMatchesSnapshot(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())

	blob := func(id, account string, size int64) {
		e.seedAccount(account)
		if _, err := e.ch.DB().ExecContext(e.ctx, `
			INSERT INTO blobs (id, account_id, sha256, size_bytes, mime_type, storage_path, status, created_at)
			VALUES (?, ?, 'deadbeef', ?, 'image/jpeg', 'de/ad', 'complete', ?)
		`, id, account, size, e.t0.UTC().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
	}

	// До вступления Боба: запись с фото и аватар владельца.
	blob("photo-old", "owner", 100)
	blob("avatar-owner", "owner", 7)
	old := e.post(circle.ID, "owner", "до Боба", "2026-08-01", e.at(0))
	e.attachPhoto(old.ID, "photo-old")
	avatar := "avatar-owner"
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "owner", chronicle.UpdateIdentityInput{
		AvatarBlobID: &avatar,
	}, e.at(0)); err != nil {
		t.Fatal(err)
	}

	e.join(circle.ID, "bob", "Боб", e.at(2))

	// При Бобе: запись с двумя фото, его комментарий и реакция, его аватар.
	blob("photo-a", "owner", 200)
	blob("photo-b", "owner", 300)
	blob("avatar-bob", "bob", 11)
	fresh := e.post(circle.ID, "owner", "при Бобе", "2026-08-03", e.at(3))
	e.attachPhoto(fresh.ID, "photo-a")
	e.attachPhoto(fresh.ID, "photo-b")
	bobAvatar := "avatar-bob"
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "bob", chronicle.UpdateIdentityInput{
		AvatarBlobID: &bobAvatar,
	}, e.at(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "bob", PostID: fresh.ID, Body: "ага", Now: e.at(3).Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
		CircleID: circle.ID, AccountID: "bob", PostID: fresh.ID, Emoji: "heart", Now: e.at(3).Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// После отсечки — не считается вовсе.
	blob("photo-after", "owner", 999)
	after := e.post(circle.ID, "owner", "после отсечки", "2026-08-10", e.at(10))
	e.attachPhoto(after.ID, "photo-after")

	const cutoff = "2026-08-06"
	for _, account := range []string{"owner", "bob"} {
		want := estimateBySnapshot(t, e, circle.ID, account, cutoff)
		got, err := e.ch.EstimateArchivePersonal(e.ctx, circle.ID, account, cutoff)
		if err != nil {
			t.Fatalf("%s: %v", account, err)
		}
		if got != want {
			t.Fatalf("%s: оценка %+v, срез даёт %+v", account, got, want)
		}
	}
}

// estimateBySnapshot считает то же самое по собранному срезу — прежним
// способом, чтобы сверить с запросами.
func estimateBySnapshot(t *testing.T, e *testEnv, circleID, accountID, cutoff string) chronicle.ArchivePersonalStats {
	t.Helper()
	posts, err := e.ch.ArchiveSnapshot(e.ctx, circleID, accountID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	blobs := map[string]bool{}
	for _, fp := range posts {
		for _, m := range fp.Media {
			blobs[m.BlobID] = true
		}
	}
	avatars, err := e.ch.IdentityAvatarBlobIDs(e.ctx, chronicle.IdentityIDsFromFeed(posts))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range avatars {
		blobs[id] = true
	}
	stats := chronicle.ArchivePersonalStats{PostCount: len(posts)}
	for id := range blobs {
		var size int64
		var status string
		err := e.ch.DB().QueryRowContext(e.ctx,
			`SELECT size_bytes, status FROM blobs WHERE id = ?`, id).Scan(&size, &status)
		if err != nil || status != "complete" {
			continue
		}
		stats.MediaFiles++
		stats.MediaBytes += size
	}
	return stats
}
