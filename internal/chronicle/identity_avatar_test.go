package chronicle_test

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func TestRenameIdentityKeepsAvatar(t *testing.T) {
	e := newTestEnv(t)
	circle := e.createCircle("owner", "Аня", chronicle.DurationWindow(24*time.Hour))
	mem, err := e.ch.MembershipForAccount(e.ctx, circle.ID, "owner")
	if err != nil {
		t.Fatalf("MembershipForAccount: %v", err)
	}
	e.seedBlob("face-blob", "owner")
	blob := "face-blob"
	if err := e.ch.UpdateIdentity(e.ctx, circle.ID, "owner", chronicle.UpdateIdentityInput{
		AvatarBlobID: &blob,
	}, e.at(0)); err != nil {
		t.Fatalf("UpdateIdentity avatar: %v", err)
	}
	if err := e.ch.RenameIdentity(e.ctx, circle.ID, "owner", "Анна", e.at(1)); err != nil {
		t.Fatalf("RenameIdentity: %v", err)
	}
	got, err := e.ch.ResolveIdentityAvatar(e.ctx, mem.IdentityID)
	if err != nil {
		t.Fatalf("ResolveIdentityAvatar: %v", err)
	}
	if got != "face-blob" {
		t.Fatalf("avatar after rename = %q", got)
	}
}
