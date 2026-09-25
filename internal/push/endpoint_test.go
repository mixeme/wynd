package push_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

func newPushService(t *testing.T) *push.Service {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc, err := push.New(st)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB().ExecContext(t.Context(), `
		INSERT INTO accounts (id, email, created_at) VALUES ('acc', 'a@test.local', ?)
	`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	return svc
}

// Инвариант (SEC-4): сервер не ходит по адресу подписки во внутреннюю сеть.
// Без проверки endpoint подписки был оракулом сканирования сети: 404/410
// удаляет подписку, остальное — нет.
func TestSubscribeRejectsNonPublicEndpoints(t *testing.T) {
	svc := newPushService(t)
	bad := []string{
		"http://127.0.0.1/ep",          // не https
		"https://127.0.0.1/ep",         // loopback
		"https://10.0.0.1/ep",          // частная сеть
		"https://192.168.1.10/ep",      // частная сеть
		"https://169.254.169.254/meta", // link-local
		"https://[::1]/ep",             // loopback v6
		"https://0.0.0.0/ep",           // unspecified
		"https://100.64.0.1/ep",        // CGNAT
		"ftp://push.example/ep",        // не https
		"https:///ep",                  // без хоста
	}
	for _, endpoint := range bad {
		err := svc.Subscribe(t.Context(), push.SubscribeInput{
			AccountID: "acc", Endpoint: endpoint, P256dh: "k", Auth: "a",
		})
		if !errors.Is(err, push.ErrInvalid) {
			t.Fatalf("%s: err = %v, want invalid", endpoint, err)
		}
	}

	// Обычный адрес push-сервиса принимается.
	if err := svc.Subscribe(t.Context(), push.SubscribeInput{
		AccountID: "acc", Endpoint: "https://push.example/ep1", P256dh: "k", Auth: "a",
	}); err != nil {
		t.Fatalf("публичный endpoint отвергнут: %v", err)
	}
}

// Инвариант (SEC-4, AUTH-5): свой клиент доставки не ходит по редиректам,
// а VAPID `sub` — mailto:, иначе строгие сервисы отвечают 400 и подписка
// молча не работает.
func TestDeliveryClientRefusesRedirectAndSendsMailtoSubscriber(t *testing.T) {
	if !strings.HasPrefix(push.SubscriberMailto, "mailto:") {
		t.Fatalf("VAPID sub = %q, want mailto:", push.SubscriberMailto)
	}

	var gotSub string
	hops := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		if auth := r.Header.Get("Authorization"); auth != "" {
			gotSub = auth
		}
		if r.URL.Path == "/ep" {
			// Редирект на loopback: по нему ходить нельзя.
			http.Redirect(w, r, "http://127.0.0.1:1/internal", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	svc := newPushService(t)
	if err := svc.EnsureKeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Ключи браузера: настоящая точка на кривой, иначе шифрование падает
	// раньше запроса. Подписку кладём мимо Subscribe: httptest живёт на
	// loopback, а проверяем здесь доставку, не приём адреса.
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p256dh := base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB().ExecContext(t.Context(), `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
		VALUES ('s1', 'acc', ?, ?, ?, '', ?)
	`, srv.URL+"/ep", p256dh, base64.RawURLEncoding.EncodeToString(secret),
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	err = svc.SendTest(t.Context(), "acc")
	if err == nil {
		t.Fatal("редирект должен считаться неуспешной доставкой")
	}
	if hops != 1 {
		t.Fatalf("клиент прошёл по редиректу: hops = %d", hops)
	}
	var left int
	if err := svc.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("подписка удалена не по 404/410: осталось %d", left)
	}
	if !strings.Contains(gotSub, "vapid") {
		t.Fatalf("Authorization без VAPID: %q", gotSub)
	}
}
