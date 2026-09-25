package chronicle_test

import (
	"errors"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestCreatePostRejectsWhitespaceWithoutMedia(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	_, err := e.ch.CreatePost(e.ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "   ",
		EntryDate: "2026-08-05", Now: e.at(0),
	})
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestCreatePostTrimsBody(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p, err := e.ch.CreatePost(e.ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "  текст  ",
		EntryDate: "2026-08-05", Now: e.at(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Body != "текст" {
		t.Fatalf("body: %q", p.Body)
	}
}

func TestCreatePostRejectsInvalidEntryDate(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	_, err := e.ch.CreatePost(e.ctx, chronicle.PostInput{
		CircleID: circle.ID, AccountID: "owner", Body: "ok",
		EntryDate: "2026-02-30", Now: e.at(0),
	})
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestCreateCommentRejectsWhitespace(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	_, err := e.ch.CreateComment(e.ctx, chronicle.CommentInput{
		CircleID: circle.ID, AccountID: "owner", PostID: p.ID, Body: "   ", Now: e.at(1),
	})
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestEditPostRejectsInvalidEntryDate(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	err := e.ch.EditPost(e.ctx, circle.ID, "owner", p.ID, "правка", "2026-02-30", e.at(0))
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestEditPostRejectsWhitespaceWithoutMedia(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.UnlimitedWindow())
	p := e.post(circle.ID, "owner", "запись", "2026-08-05", e.at(0))
	err := e.ch.EditPost(e.ctx, circle.ID, "owner", p.ID, "   ", "", e.at(0))
	if !errors.Is(err, chronicle.ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}
