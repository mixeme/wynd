package chronicle_test

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Инвариант (план 42, раздел B «Право писать»): править и удалять своё —
// запись, комментарий, реакцию, обложку — может только тот, кто сейчас может
// писать. Исключённый и вышедший с доступом получают forbidden, иначе
// исключённый участник стирает свою ветку вместе с чужими комментариями.
func TestAuthorMutatorsRequireWriteAccess(t *testing.T) {
	cases := []struct {
		name  string
		leave func(e *testEnv, circleID string, when time.Time)
	}{
		{
			name: "excluded",
			leave: func(e *testEnv, circleID string, when time.Time) {
				if err := e.ch.Exclude(e.ctx, circleID, "owner", "bob", when); err != nil {
					e.t.Fatalf("Exclude: %v", err)
				}
			},
		},
		{
			name: "left_with_access",
			leave: func(e *testEnv, circleID string, when time.Time) {
				if err := e.ch.LeaveWithAccess(e.ctx, circleID, "bob", when); err != nil {
					e.t.Fatalf("LeaveWithAccess: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t)
			// Окно правки шире отрезка членства: отказ должен приходить
			// от прав, а не от истёкшего окна.
			circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(30*24*time.Hour))
			e.join(circle.ID, "bob", "Боб", e.after(time.Hour))

			post := e.post(circle.ID, "bob", "моё", "2026-08-01", e.after(2*time.Hour))
			e.seedBlob("blob-a", "bob")
			e.seedBlob("blob-b", "bob")
			e.attachPhoto(post.ID, "blob-a")
			if err := e.ch.AttachMedia(e.ctx, post.ID, []chronicle.MediaInput{{
				BlobID: "blob-b", Kind: chronicle.MediaPhoto,
			}}); err != nil {
				t.Fatalf("AttachMedia: %v", err)
			}
			comment, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
				CircleID: circle.ID, AccountID: "bob", PostID: post.ID,
				Body: "мой комментарий", Now: e.after(3 * time.Hour),
			})
			if err != nil {
				t.Fatalf("CreateComment: %v", err)
			}
			reaction, err := e.ch.SetReaction(e.ctx, chronicle.ReactionInput{
				CircleID: circle.ID, AccountID: "bob", PostID: post.ID,
				Emoji: "heart", Now: e.after(4 * time.Hour),
			})
			if err != nil {
				t.Fatalf("SetReaction: %v", err)
			}

			tc.leave(e, circle.ID, e.after(5*time.Hour))
			now := e.after(6 * time.Hour)

			checks := []struct {
				what string
				err  error
			}{
				{"EditPost", e.ch.EditPost(e.ctx, circle.ID, "bob", post.ID, "переписал", "", now)},
				{"SetPostCover", e.ch.SetPostCover(e.ctx, circle.ID, "bob", post.ID, "blob-b", now)},
				{"EditComment", e.ch.EditComment(e.ctx, circle.ID, "bob", comment.ID, "переписал", now)},
				{"DeleteComment", e.ch.DeleteComment(e.ctx, circle.ID, "bob", comment.ID, now)},
				{"DeleteReaction", e.ch.DeleteReaction(e.ctx, circle.ID, "bob", reaction.ID, now)},
				{"DeletePost", e.ch.DeletePost(e.ctx, circle.ID, "bob", post.ID, now)},
			}
			for _, c := range checks {
				if !errors.Is(c.err, chronicle.ErrForbidden) {
					t.Fatalf("%s: err = %v, want forbidden", c.what, c.err)
				}
			}

			// Состояние, а не только код ответа: ветка цела.
			var body string
			var deleted int
			if err := e.ch.DB().QueryRowContext(e.ctx,
				`SELECT body, deleted FROM posts WHERE id = ?`, post.ID).Scan(&body, &deleted); err != nil {
				t.Fatalf("load post: %v", err)
			}
			if body != "моё" || deleted != 0 {
				t.Fatalf("post changed: body=%q deleted=%d", body, deleted)
			}
			var comments int
			if err := e.ch.DB().QueryRowContext(e.ctx,
				`SELECT count(*) FROM comments WHERE post_id = ? AND deleted = 0`, post.ID).Scan(&comments); err != nil {
				t.Fatalf("count comments: %v", err)
			}
			if comments != 1 {
				t.Fatalf("comments left = %d, want 1", comments)
			}
		})
	}
}

// Автор, который остался писателем, по-прежнему правит своё: шлюз не должен
// закрывать обычный путь.
func TestAuthorMutatorsStillWorkForActiveMember(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(30*24*time.Hour))
	e.join(circle.ID, "bob", "Боб", e.after(time.Hour))
	post := e.post(circle.ID, "bob", "моё", "2026-08-01", e.after(2*time.Hour))

	if err := e.ch.EditPost(e.ctx, circle.ID, "bob", post.ID, "поправил", "", e.after(3*time.Hour)); err != nil {
		t.Fatalf("EditPost: %v", err)
	}
	if err := e.ch.DeletePost(e.ctx, circle.ID, "bob", post.ID, e.after(4*time.Hour)); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}
}

// Инвариант (CHR-1): владелец может отозвать доступ у вышедшего с доступом.
// Открытого отрезка у него нет, поэтому исключение снимает can_read со всех
// его отрезков; раньше Exclude возвращал invalid и единственным способом
// отозвать чтение было удаление круга.
func TestExcludeRevokesAccessOfLeftWithAccess(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	e.join(circle.ID, "bob", "Боб", e.after(time.Hour))
	e.post(circle.ID, "owner", "видно бобу", "2026-08-01", e.after(2*time.Hour))

	if err := e.ch.LeaveWithAccess(e.ctx, circle.ID, "bob", e.after(3*time.Hour)); err != nil {
		t.Fatalf("LeaveWithAccess: %v", err)
	}
	canRead, err := e.ch.CanReadEvent(e.ctx, circle.ID, "bob", e.after(2*time.Hour))
	if err != nil {
		t.Fatalf("CanReadEvent: %v", err)
	}
	if !canRead {
		t.Fatal("вышедший с доступом должен видеть прошлое до выхода")
	}

	if err := e.ch.Exclude(e.ctx, circle.ID, "owner", "bob", e.after(4*time.Hour)); err != nil {
		t.Fatalf("Exclude: %v", err)
	}
	canRead, err = e.ch.CanReadEvent(e.ctx, circle.ID, "bob", e.after(2*time.Hour))
	if err != nil {
		t.Fatalf("CanReadEvent: %v", err)
	}
	if canRead {
		t.Fatal("после исключения доступ должен быть отозван")
	}
	var status string
	if err := e.ch.DB().QueryRowContext(e.ctx,
		`SELECT status FROM memberships WHERE circle_id = ? AND account_id = 'bob'`,
		circle.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(chronicle.StatusGone) {
		t.Fatalf("status = %q, want gone", status)
	}
}
