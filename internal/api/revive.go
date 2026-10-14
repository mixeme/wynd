package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Оживление круга из полного архива (план 52). Устройство расшифровало и
// прочитало файл, загрузило файлы круга обычными загрузками частями и
// присылает строки таблиц и соответствие «блоб архива → блоб сервера».

// maxReviveBody — строки таблиц круга одним JSON: на большом круге это
// мегабайты, обычные 1 МБ запросов сюда не годятся. Файлы идут загрузками.
const maxReviveBody = 256 << 20

type reviveTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type reviveBody struct {
	Manifest struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
		SchemaVersion int    `json:"schema_version"`
		Posts         int    `json:"posts"`
	} `json:"manifest"`
	Tables      map[string]reviveTable `json:"tables"`
	BlobMap     map[string]string      `json:"blob_map"`
	SendInvites bool                   `json:"send_invites"`
}

// handleReviveBlobs — какие файлы архива у этого человека уже есть на
// сервере (та же сумма и длина): их второй раз не грузят. Только свои —
// поиск по чужим выдавал бы, лежит ли на сервере такая фотография.
func (s *Server) handleReviveBlobs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		SHA256 []string `json:"sha256"`
	}](w, r)
	if !ok {
		return
	}
	found, err := s.Blobs.OwnBlobsBySHA(r.Context(), sess.AccountID, body.SHA256)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blobs": found})
}

// handleReviveCircle — «Оживить круг здесь» (2.22): круг вставляется заново,
// тот, кто оживил, — владелец; итог — для 2.28 / 2.34.
func (s *Server) handleReviveCircle(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	var body reviveBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Manifest.Format != "wynd-circle" || body.Manifest.FormatVersion != 1 {
		writeDomainError(w, chronicle.ErrInvalid)
		return
	}
	tables := map[string]chronicle.FullArchiveTable{}
	for name, t := range body.Tables {
		tables[name] = chronicle.FullArchiveTable{Name: name, Columns: t.Columns, Rows: t.Rows}
	}
	accounts := map[string]string{}
	if t, ok := tables["accounts"]; ok {
		id, email := slices.Index(t.Columns, "id"), slices.Index(t.Columns, "email")
		for _, row := range t.Rows {
			if id < 0 || email < 0 {
				break
			}
			a, _ := row[id].(string)
			e, _ := row[email].(string)
			if a != "" && strings.Contains(e, "@") {
				accounts[a] = e
			}
		}
	}
	// Каждый файл архива — на сервере, своей суммой и длиной (2.28: «сверено
	// с файлом архива, ничего не потерялось»).
	var archiveBytes int64
	if t, ok := tables["blobs"]; ok {
		id, sum, size := slices.Index(t.Columns, "id"), slices.Index(t.Columns, "sha256"), slices.Index(t.Columns, "size_bytes")
		for _, row := range t.Rows {
			old, _ := row[id].(string)
			nw, ok := body.BlobMap[old]
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "blob_missing", "detail": old})
				return
			}
			gotSum, gotSize, owner, err := s.Blobs.BlobDigest(r.Context(), nw)
			if err != nil || owner != sess.AccountID {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "blob_missing", "detail": old})
				return
			}
			wantSum, _ := row[sum].(string)
			wantSize, _ := row[size].(float64)
			if gotSum != wantSum || gotSize != int64(wantSize) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "blob_mismatch", "detail": old})
				return
			}
			archiveBytes += gotSize
		}
	}
	res, err := s.Chronicle.ReviveCircle(r.Context(), chronicle.ReviveInput{
		SchemaVersion: body.Manifest.SchemaVersion,
		Tables:        tables,
		Accounts:      accounts,
		BlobMap:       body.BlobMap,
		ReviverID:     sess.AccountID,
		Now:           time.Now(),
	})
	if err != nil {
		if errors.Is(err, chronicle.ErrInvalid) {
			log.Printf("revive: %v", err)
		}
		writeDomainError(w, err)
		return
	}
	invited := 0
	if body.SendInvites {
		invited = s.inviteNotJoined(res.CircleID, sess.AccountID, res.NotJoined)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"circle_id":  res.CircleID,
		"posts":      res.Posts,
		"names":      res.Names,
		"bytes":      res.Bytes,
		"verified":   res.Posts == body.Manifest.Posts && res.Bytes == archiveBytes,
		"invited":    invited,
		"not_joined": len(res.NotJoined),
	})
}

// inviteNotJoined — письмо каждому, кто приехал с кругом, но на этом сервере
// ещё не был (2.22, 6.35): войти той же почтой — и он в своём имени.
func (s *Server) inviteNotJoined(circleID, ownerAccountID string, accountIDs []string) int {
	if s.Mail == nil || len(accountIDs) == 0 {
		return 0
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		name, err := s.Chronicle.CircleName(ctx, circleID)
		if err != nil {
			log.Printf("revive invites %s: %v", circleID, err)
			return
		}
		owner, _ := s.Chronicle.OwnerName(ctx, circleID)
		for _, id := range accountIDs {
			email, err := s.Chronicle.AccountEmail(ctx, id)
			if err != nil {
				continue
			}
			if err := s.Mail.SendCircleWaiting(ctx, email, name, owner, link); err != nil {
				log.Printf("revive invite %s to %s: %v", circleID, email, err)
			}
		}
	}()
	return len(accountIDs)
}

// withReviveBody — участник с оплатой и телом до maxReviveBody.
func (s *Server) withReviveBody(next http.HandlerFunc) http.HandlerFunc {
	return s.RequireParticipantStream(func(w http.ResponseWriter, r *http.Request) {
		if err := s.requirePaidSession(r); err != nil {
			writeError(w, err)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxReviveBody)
		}
		next(w, r)
	})
}
