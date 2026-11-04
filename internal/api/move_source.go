package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// maxMoveCard — предел ответа прежнего сервера с карточкой.
const maxMoveCard = 64 << 10

// moveCall — запрос к прежнему серверу по ключу ссылки.
func (s *Server) moveCall(ctx context.Context, link, method, path string, body any, header map[string]string) (*http.Response, error) {
	base, token, ok := parseMoveLink(link)
	if !ok {
		return nil, chronicle.ErrInvalid
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Move "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return s.moveClient().Do(req)
}

// moveErrorCode — код ошибки из ответа прежнего сервера.
func moveErrorCode(resp *http.Response) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, maxMoveCard)).Decode(&e)
	return e.Error
}

// moveCard — карточка круга с прежнего сервера.
type moveCard struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"format_version"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expires_at"`
	ServerURL     string    `json:"server_url"`
	ServerName    string    `json:"server_name"`
	FrozenAt      string    `json:"frozen_at"`
	SchemaVersion int       `json:"schema_version"`
	Circle        *struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Color   string `json:"color"`
		Posts   int    `json:"posts"`
		Members int    `json:"members"`
		Bytes   int64  `json:"bytes"`
		// OwnerName и Emails — для «Аня — это вы?» (A12); у прежнего сервера
		// до 0.42 их нет.
		OwnerName string   `json:"owner_name"`
		Emails    []string `json:"emails"`
		// Names — «Кто уже здесь» (7.29в при переносе); с 0.42.1.
		Names []string `json:"names"`
	} `json:"circle"`
}

// fetchMoveCard спрашивает карточку. Ошибки — коды для 7.20: link_invalid,
// move_not_found, source_unreachable, source_too_old.
func (s *Server) fetchMoveCard(ctx context.Context, link string) (*moveCard, string) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := s.moveCall(ctx, link, http.MethodGet, "/api/v1/move", nil, nil)
	if errors.Is(err, chronicle.ErrInvalid) {
		return nil, "link_invalid"
	}
	if err != nil {
		log.Printf("move card: %v", err)
		return nil, "source_unreachable"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
		if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") && moveErrorCode(resp) == "move_not_found" {
			return nil, "move_not_found"
		}
		return nil, "source_too_old"
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "source_unreachable"
	}
	var card moveCard
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMoveCard)).Decode(&card); err != nil || card.Format != "wynd-move" {
		return nil, "source_too_old"
	}
	if card.FormatVersion != 1 {
		return nil, "source_too_old"
	}
	return &card, ""
}
