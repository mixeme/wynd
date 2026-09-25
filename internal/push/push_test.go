package push_test

import (
	"context"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func TestEnsureKeysGeneratesP256VAPIDKeys(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := push.New(st)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if err := svc.EnsureKeys(ctx); err != nil {
		t.Fatal(err)
	}
	pub1, err := svc.PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pub1 == "" {
		t.Fatal("want public key")
	}

	// Second call must be idempotent.
	if err := svc.EnsureKeys(ctx); err != nil {
		t.Fatal(err)
	}
	pub2, err := svc.PublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pub1 != pub2 {
		t.Fatalf("public key changed: %q -> %q", pub1, pub2)
	}
}

func TestPublicKeyNotConfiguredBeforeEnsure(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc, err := push.New(st)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_, err = svc.PublicKey(ctx)
	if err != push.ErrNotConfigured {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}
