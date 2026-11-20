package push_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/push"
)

// The opt-in Android probe supplies keys from two on-device registrations.
// SendSignal uses Wynd's real RFC 8291 encoder and different VAPID keys per server.
func TestAndroidConnectorWebPushFixture(t *testing.T) {
	input := os.Getenv("WYND_ANDROID_PUSH_PROBE_KEYS")
	if input == "" {
		t.Skip("run with scripts/test-android-push.mjs")
	}
	var subscriptions map[string]struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	}
	if err := json.Unmarshal([]byte(input), &subscriptions); err != nil {
		t.Fatal("invalid fixture subscriptions")
	}
	first, second := subscriptions["first"], subscriptions["second"]
	if first.P256dh == second.P256dh || first.Auth == second.Auth {
		t.Fatal("the registrations must have independent keys")
	}

	sealed := make(map[string]string)
	vapidKeys := make(map[string]bool)
	signals := []push.Signal{
		{CircleID: "first", PostID: "fixture-post", Type: "comment", Count: 1},
		{CircleID: "second", PostID: "fixture-other-post", Type: "mention", Count: 1},
	}
	for _, signal := range signals {
		svc := newPushService(t)
		svc.AllowLoopbackDeliveryForTest()
		if err := svc.EnsureKeys(t.Context()); err != nil {
			t.Fatal(err)
		}
		vapid, err := svc.PublicKey(t.Context())
		if err != nil || vapidKeys[vapid] {
			t.Fatal("could not get a separate server VAPID key")
		}
		vapidKeys[vapid] = true
		capture := make(chan []byte, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil || r.Method != http.MethodPost || r.Header.Get("Content-Encoding") != "aes128gcm" ||
				!strings.HasSuffix(r.Header.Get("Authorization"), "k="+vapid) || len(body) != 2048 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			capture <- body
			w.WriteHeader(http.StatusCreated)
		}))
		t.Cleanup(server.Close)
		keys := subscriptions[signal.CircleID]
		// The loopback-only fixture never changes production endpoint validation.
		if _, err := svc.DB().ExecContext(t.Context(), `
			INSERT INTO push_subscriptions (id, account_id, endpoint, p256dh, auth, user_agent, created_at)
			VALUES ('android-probe', 'acc', ?, ?, ?, '', ?)
		`, server.URL+"/fixture", keys.P256dh, keys.Auth, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		if err := svc.SendSignal(t.Context(), "acc", signal); err != nil {
			t.Fatal(err)
		}
		select {
		case body := <-capture:
			sealed[signal.CircleID] = base64.RawURLEncoding.EncodeToString(body)
		default:
			t.Fatal("no encrypted signal")
		}
	}
	encoded, err := json.Marshal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("WYND_ANDROID_PUSH_PROBE_SEALED=%s", encoded)
}
