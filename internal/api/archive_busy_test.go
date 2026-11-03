package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Repeated downloads fail while the caller is still inside Wynd, before a
// browser token is issued. Access checks precede the account's busy state.
func TestArchiveLinkRejectsBusyBeforeOpeningBrowser(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "busy-link@example.com")
	stranger, _ := registerSession(t, srv, caps, "busy-stranger@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{"name": "Семья", "owner_name": "Аня"})
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Code, rec.Body.String())
	}
	cid := jsonStr(t, rec, "id")
	release := srv.HoldArchiveBuildForTest(accountID)
	defer release()
	linkPath := "/api/v1/circles/" + cid + "/archive/link"
	busy := doJSON(t, srv, http.MethodPost, linkPath, token, nil)
	var body struct {
		Error string `json:"error"`
		URL   string `json:"url"`
		Retry int    `json:"retry_after_sec"`
	}
	if err := json.Unmarshal(busy.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if busy.Code != http.StatusTooManyRequests || body.Error != "rate_limited" || body.URL != "" || body.Retry != 30 || busy.Header().Get("Retry-After") != "30" {
		t.Fatalf("busy link: %d %s", busy.Code, busy.Body.String())
	}
	denied := doJSON(t, srv, http.MethodPost, linkPath, stranger, nil)
	if denied.Code != http.StatusForbidden && denied.Code != http.StatusNotFound {
		t.Fatalf("stranger learned busy state: %d", denied.Code)
	}
	if noSession := doJSON(t, srv, http.MethodPost, linkPath, "", nil); noSession.Code != http.StatusForbidden || noSession.Header().Get("Retry-After") != "" {
		t.Fatalf("link without session: %d %s", noSession.Code, noSession.Body.String())
	}
	release()
	if ready := doJSON(t, srv, http.MethodPost, linkPath, token, nil); ready.Code != http.StatusOK {
		t.Fatalf("link after completion: %d %s", ready.Code, ready.Body.String())
	}
}

// Two windows can obtain tokens before either starts downloading. The second
// valid token explains the busy response without requiring a browser session;
// it neither cancels the first download nor makes that token reusable.
func TestArchiveBusyTokenNavigationKeepsOneTimeAuthorization(t *testing.T) {
	srv, caps, _, _ := setupAPI(t)
	token, accountID := registerSession(t, srv, caps, "busy-nav@example.com")
	rec := doJSON(t, srv, http.MethodPost, "/api/v1/circles", token, map[string]any{"name": "Семья", "owner_name": "Аня"})
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Code, rec.Body.String())
	}
	cid := jsonStr(t, rec, "id")
	linkPath := "/api/v1/circles/" + cid + "/archive/link"
	getLink := func() string {
		t.Helper()
		r := doJSON(t, srv, http.MethodPost, linkPath, token, nil)
		if r.Code != http.StatusOK {
			t.Fatal(r.Code, r.Body.String())
		}
		return jsonStr(t, r, "url")
	}
	first, second, third, safari := getLink(), getLink(), getLink(), getLink()
	release := srv.HoldArchiveBuildForTest(accountID)
	defer release()
	req := httptest.NewRequest(http.MethodGet, first, nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	browser := httptest.NewRecorder()
	srv.ServeHTTP(browser, req)
	if browser.Code != http.StatusSeeOther || browser.Header().Get("Location") != "/archive/busy?retry_after=30" || browser.Header().Get("Retry-After") != "30" {
		t.Fatalf("browser busy: %d %v", browser.Code, browser.Header())
	}
	if browser.Header().Get("Cache-Control") != "no-store" || browser.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("busy response may retain a token", browser.Header())
	}
	// Older browsers may supply Accept without Sec-Fetch-Mode.
	safariRequest := httptest.NewRequest(http.MethodGet, safari, nil)
	safariRequest.Header.Set("Accept", "text/html,application/xhtml+xml")
	safariResponse := httptest.NewRecorder()
	srv.ServeHTTP(safariResponse, safariRequest)
	if safariResponse.Code != http.StatusSeeOther || safariResponse.Header().Get("Location") != "/archive/busy?retry_after=30" {
		t.Fatalf("browser without Sec-Fetch-Mode: %d %v", safariResponse.Code, safariResponse.Header())
	}
	// An API consumer keeps the structured 429 contract.
	apiReply := doGET(t, srv, second, "")
	if apiReply.Code != http.StatusTooManyRequests || !strings.Contains(apiReply.Body.String(), `"error":"rate_limited"`) {
		t.Fatalf("API busy: %d %s", apiReply.Code, apiReply.Body.String())
	}
	// Busy navigation must not release the other request's lock.
	if r := doJSON(t, srv, http.MethodPost, linkPath, token, nil); r.Code != http.StatusTooManyRequests {
		t.Fatalf("active download was released: %d", r.Code)
	}
	// An invalid token cannot discover whether anybody is downloading.
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/archive/not-a-valid-token", nil)
	bad.Header.Set("Accept", "text/html")
	rejected := httptest.NewRecorder()
	srv.ServeHTTP(rejected, bad)
	if rejected.Code != http.StatusSeeOther || rejected.Header().Get("Location") != "/circles" {
		t.Fatalf("invalid token: %d %v", rejected.Code, rejected.Header())
	}
	release()
	if retry := doGET(t, srv, first, ""); retry.Code != http.StatusNotFound {
		t.Fatalf("consumed token can be reused: %d", retry.Code)
	}
	if download := doGET(t, srv, third, ""); download.Code != http.StatusOK || download.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("valid token now requires session: %d %s", download.Code, download.Body.String())
	}
}
