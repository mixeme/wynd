package push_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"io"
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
	// Боевой клиент не набирает loopback (SEC-7), а httptest живёт на нём.
	svc.AllowLoopbackDeliveryForTest()
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

// Инвариант (SEC-7, аудит 2026-09-22): непубличный адрес отвергается на
// каждой доставке, а не только при подписке. Подписка принята, когда имя
// смотрело наружу; после этого имя перевели на loopback — запрос не должен
// уйти вовсе (DNS rebinding).
func TestDeliveryRefusesEndpointResolvingToLoopback(t *testing.T) {
	hops := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	svc := newPushService(t)
	if err := svc.EnsureKeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Тот же порт, но по имени: localhost резолвится в loopback, как имя,
	// переведённое туда после подписки.
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://localhost:" + port + "/secret-endpoint-token"
	seedSubscription(t, svc, endpoint)

	err = svc.SendTest(t.Context(), "acc")
	if err == nil {
		t.Fatal("доставка на loopback должна падать")
	}
	if !errors.Is(err, push.ErrInvalid) {
		t.Fatalf("ошибка %v, ожидался отказ по адресу", err)
	}
	if hops != 0 {
		t.Fatalf("запрос всё-таки ушёл: hops = %d", hops)
	}
	var left int
	if err := svc.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("подписка снята не по 404/410: осталось %d", left)
	}
}

// Инвариант (SEC-7): путь endpoint — ключ от устройства, в ошибке остаётся
// только хост.
func TestDeliveryErrorKeepsHostWithoutPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	srvHost := strings.TrimPrefix(srv.URL, "http://")

	svc := newPushService(t)
	svc.AllowLoopbackDeliveryForTest()
	if err := svc.EnsureKeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	seedSubscription(t, svc, srv.URL+"/secret-endpoint-token")

	err := svc.SendTest(t.Context(), "acc")
	if err == nil {
		t.Fatal("статус 500 — неуспешная доставка")
	}
	if strings.Contains(err.Error(), "secret-endpoint-token") {
		t.Fatalf("в ошибке путь endpoint: %v", err)
	}
	if !strings.Contains(err.Error(), srvHost) {
		t.Fatalf("в ошибке нет хоста: %v", err)
	}

	// Транспортная ошибка приходит из *url.Error с полным адресом — она
	// тоже должна дойти до лога без пути.
	srv.Close()
	err = svc.SendTest(t.Context(), "acc")
	if err == nil {
		t.Fatal("закрытый сервер — неуспешная доставка")
	}
	if strings.Contains(err.Error(), "secret-endpoint-token") {
		t.Fatalf("в ошибке путь endpoint: %v", err)
	}
}

// seedSubscription кладёт подписку мимо Subscribe: проверяется доставка, а
// не приём адреса. Ключи браузера — настоящая точка на кривой, иначе
// шифрование падает раньше запроса.
func seedSubscription(t *testing.T, svc *push.Service, endpoint string) {
	t.Helper()
	seedSubscriptionID(t, svc, "s-seed", endpoint)
}

func seedSubscriptionID(t *testing.T, svc *push.Service, id, endpoint string) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB().ExecContext(t.Context(), `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
		VALUES (?, 'acc', ?, ?, ?, '', ?)
	`, id, endpoint, base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(secret),
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

// Сервис пушей Mozilla для телефонов принимает тело не длиннее 3070 байт
// после base64 («limited to 3070 bytes») и на большее отвечает 413.
// webpush-go добивает запись до RecordSize — при 4096 и при 3070 каждый пуш в
// Firefox на Android отбивался, каким бы коротким ни был сигнал.
func TestDeliveryFitsMozillaPayloadLimit(t *testing.T) {
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotLen = len(body)
		if base64.RawURLEncoding.EncodedLen(len(body)) > 3070 {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	svc := newPushService(t)
	svc.AllowLoopbackDeliveryForTest()
	if err := svc.EnsureKeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	err = svc.SendTestTo(t.Context(), srv.URL+"/ep",
		base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(secret))
	// SendTestTo проверяет адрес как подписку, а httptest — loopback: доставку
	// ведём через сохранённую подписку, как в боевом notify.
	if err == nil {
		t.Fatal("loopback endpoint must be refused by SendTestTo")
	}
	if _, err := svc.DB().ExecContext(t.Context(), `
		INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
		VALUES ('s1', 'acc', ?, ?, ?, '', ?)
	`, srv.URL+"/ep", base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(secret), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := svc.SendTest(t.Context(), "acc"); err != nil {
		t.Fatalf("delivery of %d bytes: %v", gotLen, err)
	}
}
