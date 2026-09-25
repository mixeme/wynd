package push_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.mixdep.ru/mix/wynd/internal/push"
)

// Инвариант: подписку снимает только ответ «этого адреса больше нет» (404 и
// 410). Всё остальное — временная беда чужого сервиса: подписка остаётся,
// иначе одна пятисотка push-службы отписывала бы всех её пользователей.
func TestDeliveryDropsSubscriptionOnlyOnGone(t *testing.T) {
	cases := []struct {
		name   string
		status int
		kept   bool
		wantOK bool
	}{
		{"принято", http.StatusCreated, true, true},
		{"адреса нет", http.StatusNotFound, false, false},
		{"адрес снят", http.StatusGone, false, false},
		{"неверный запрос", http.StatusBadRequest, true, false},
		{"сервис лежит", http.StatusInternalServerError, true, false},
		{"слишком часто", http.StatusTooManyRequests, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			svc := newPushService(t)
			svc.AllowLoopbackDeliveryForTest()
			if err := svc.EnsureKeys(t.Context()); err != nil {
				t.Fatal(err)
			}
			seedSubscription(t, svc, srv.URL+"/ep")

			err := svc.SendTest(t.Context(), "acc")
			if tc.wantOK && err != nil {
				t.Fatalf("доставка: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("статус %d принят за успех", tc.status)
			}
			if hits != 1 {
				t.Fatalf("запросов к сервису: %d", hits)
			}

			var left int
			if err := svc.DB().QueryRowContext(t.Context(),
				`SELECT count(*) FROM push_subscriptions`).Scan(&left); err != nil {
				t.Fatal(err)
			}
			if tc.kept && left != 1 {
				t.Fatalf("подписка снята по статусу %d", tc.status)
			}
			if !tc.kept && left != 0 {
				t.Fatalf("подписка осталась после статуса %d", tc.status)
			}
		})
	}
}

// Инвариант: молчащий адрес не глушит остальных. Ошибка возвращается, но
// доставка продолжается по всем подпискам учётки.
func TestDeliveryContinuesAfterFailedEndpoint(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	goodHits := 0
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits++
		w.WriteHeader(http.StatusCreated)
	}))
	defer good.Close()

	svc := newPushService(t)
	svc.AllowLoopbackDeliveryForTest()
	if err := svc.EnsureKeys(t.Context()); err != nil {
		t.Fatal(err)
	}
	seedSubscriptionID(t, svc, "s-bad", bad.URL+"/ep")
	seedSubscriptionID(t, svc, "s-good", good.URL+"/ep")

	err := svc.SendTest(t.Context(), "acc")
	if err == nil {
		t.Fatal("сбой одной доставки должен вернуться ошибкой")
	}
	if goodHits != 1 {
		t.Fatalf("живой адрес получил %d запросов", goodHits)
	}
	var left int
	if err := svc.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 2 {
		t.Fatalf("подписок осталось %d, ожидалось 2", left)
	}
}

// Инвариант: без ключей VAPID доставка не начинается — подписки не трогаются.
func TestSendSignalWithoutKeys(t *testing.T) {
	svc := newPushService(t)
	seedSubscription(t, svc, "https://push.example/ep")
	err := svc.SendTest(t.Context(), "acc")
	if !errors.Is(err, push.ErrNotConfigured) {
		t.Fatalf("без ключей: %v", err)
	}
	var left int
	if err := svc.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("подписка снята без доставки: %d", left)
	}
}
