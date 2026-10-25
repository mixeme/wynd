package installer

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

// 10а: предлагается свежий выпуск Мастерской, вышедший не меньше
// UpdateDelay назад; черновики, пробные и чужие метки — нет.
func TestPickRelease(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)
	assets := []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{{"wynd-workshop.exe", "https://x/exe"}, {"wynd-workshop.exe.sig", "https://x/sig"}}
	rels := []ghRelease{
		{TagName: "v0.44.0", PublishedAt: old},
		{TagName: "workshop-v0.3.0", PublishedAt: now.Add(-time.Hour), Assets: assets},
		{TagName: "workshop-v0.2.1", PublishedAt: old, Draft: true, Assets: assets},
		{TagName: "workshop-v0.2.0", PublishedAt: old, Assets: assets, Body: "- Обновляет Wynd 0.43 и новее\n- Бэкап с паролем\n"},
	}
	up := pickRelease(rels, "0.1.0", now)
	if !up.Available || up.Version != "0.2.0" || len(up.Notes) != 2 {
		t.Fatalf("выпуск: %+v", up)
	}
	if up := pickRelease(rels, "0.2.0", now); up.Available {
		t.Fatalf("новее нет, а предложено: %+v", up)
	}
}

func TestVerifySignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	key := base64.StdEncoding.EncodeToString(pub)
	exe := []byte("MZ new workshop")
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, exe)))
	if err := VerifySignature(key, exe, sig); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(key, append(exe, '!'), sig); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("подменённый exe принят: %v", err)
	}
}
