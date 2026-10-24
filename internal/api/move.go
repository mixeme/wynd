package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Перенос круга по ссылке (план 53, A1 этап 3), прежний сервер. Владелец
// выдаёт ссылку (6.33, 6.34), круг замирает. Новый сервер приходит по ней сам:
// карточка круга, «беру», полный архив без шифрования с докачкой, «принято и
// сверено». Через телефон владельца не идёт ни байта журнала.

// MoveDir — архивы под перенос (исходящие) и забираемые круги (входящие).
// Отдельно от архивов: их каталог при запуске чистится по своей таблице.
func MoveDir(dataDir string) string {
	return filepath.Join(dataDir, "moves")
}

func (s *Server) moveArchivePath(id string) string {
	return filepath.Join(MoveDir(s.DataDir), "out-"+id+".wynd")
}

// moveTokenAlphabet — буквы и цифры без похожих (0/O, 1/l/I): ссылку могут и
// переписать руками.
const moveTokenAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// newMoveToken — ключ ссылки: 16 знаков по четыре через дефис, как на 6.34
// («7Kq2-Zt9w-Xb41-Lm8c»), около 93 бит.
func newMoveToken() (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(moveTokenAlphabet)))
	for i := range 16 {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(moveTokenAlphabet[n.Int64()])
	}
	return b.String(), nil
}

func moveTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// moveLink — ссылка переноса: адрес этого сервера и ключ.
func (s *Server) moveLink(token string) string {
	return strings.TrimRight(s.PublicURL(), "/") + "/move/" + token
}

func moveJSON(m *chronicle.CircleMove, now time.Time) map[string]any {
	out := map[string]any{
		"id":             m.ID,
		"status":         m.Status,
		"started_at":     m.StartedAt,
		"expires_at":     m.ExpiresAt,
		"link_alive":     m.LinkAlive(now),
		"archive_status": m.ArchiveStatus,
		"archive_bytes":  m.ArchiveBytes,
		"pulled_bytes":   m.PulledBytes,
	}
	if m.DestURL != "" {
		out["dest_url"] = m.DestURL
		out["dest_name"] = m.DestName
	}
	if m.AcceptedAt != nil {
		out["accepted_at"] = *m.AcceptedAt
		out["accepted_posts"] = m.AcceptedPosts
		out["accepted_names"] = m.AcceptedNames
		out["accepted_bytes"] = m.AcceptedBytes
	}
	return out
}

// handleCircleMove — экран переноса у владельца: что уедет (6.33) и что с
// переносом сейчас (6.34, 6.35).
func (s *Server) handleCircleMove(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	bytes, posts, members, err := s.Chronicle.CircleArchiveEstimate(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"estimate": map[string]any{"bytes": bytes, "posts": posts, "members": members},
		"move":     nil,
	}
	m, err := s.Chronicle.LiveMove(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if m != nil {
		move := moveJSON(m, time.Now())
		if m.LinkAlive(time.Now()) {
			move["link"] = s.moveLink(m.Token)
		}
		out["move"] = move
	}
	if frozen, err := s.Chronicle.CircleFrozenAt(r.Context(), circleID); err == nil && frozen != nil {
		out["frozen_at"] = frozen.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleStartMove — «Остановить круг и получить ссылку» (6.33): круг замер,
// ссылка — в ответе и на 6.34, пока жива.
func (s *Server) handleStartMove(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	token, err := newMoveToken()
	if err != nil {
		writeError(w, err)
		return
	}
	m, err := s.Chronicle.StartMove(r.Context(), circleID, sess.AccountID, token, moveTokenHash(token), time.Now())
	if errors.Is(err, chronicle.ErrMoveBusy) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "circle_frozen"})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	s.sendCircleMovingEmails(circleID, sess.AccountID)
	s.notifyCircle(circleID, sess.AccountID, "event")
	out := moveJSON(m, time.Now())
	out["link"] = s.moveLink(token)
	writeJSON(w, http.StatusCreated, out)
}

// sendCircleMovingEmails — письмо участникам: круг переезжает (3.14).
func (s *Server) sendCircleMovingEmails(circleID, ownerAccountID string) {
	if s.Mail == nil {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/circles/" + circleID
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		name, err := s.Chronicle.CircleName(ctx, circleID)
		if err != nil {
			log.Printf("circle moving mail %s: %v", circleID, err)
			return
		}
		owner, _ := s.Chronicle.OwnerName(ctx, circleID)
		ownerEmail, _ := s.Chronicle.AccountEmail(ctx, ownerAccountID)
		emails, err := s.Chronicle.CircleMemberEmails(ctx, circleID)
		if err != nil {
			log.Printf("circle moving mail %s: members: %v", circleID, err)
			return
		}
		for _, email := range emails {
			if email == ownerEmail {
				continue
			}
			if err := s.Mail.SendCircleMoving(ctx, email, name, owner, link); err != nil {
				log.Printf("circle moving mail %s to %s: %v", circleID, email, err)
			}
		}
	}()
}

// handleCancelMove — «Отменить перенос» (6.34, 6.35): ключ гаснет, архив
// стирается, круг оживает.
func (s *Server) handleCancelMove(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	m, err := s.Chronicle.CancelMove(r.Context(), circleID, sess.AccountID, time.Now())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	s.dropMoveArchive(r.Context(), m.ID)
	s.notifyCircle(circleID, sess.AccountID, "event")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleCompleteMove — «Завершить перенос» (6.35): только когда новый сервер
// принял круг и сверил. Круг удаляется, участникам — письмо с новым адресом.
func (s *Server) handleCompleteMove(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	m, err := s.Chronicle.LiveMove(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if m == nil || m.Status != chronicle.MoveAccepted {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "not_accepted"})
		return
	}
	name, err := s.Chronicle.CircleName(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	emails, err := s.Chronicle.CircleMemberEmails(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	ownerEmail, _ := s.Chronicle.AccountEmail(r.Context(), sess.AccountID)
	archives, err := s.Chronicle.FullArchivesWithFiles(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	s.dropMoveArchive(r.Context(), m.ID)
	blobIDs, err := s.Chronicle.DeleteCircle(r.Context(), circleID, sess.AccountID, name, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	for _, x := range archives {
		if cancel, ok := s.fullBuilds.Load(x.ID); ok {
			cancel.(context.CancelFunc)()
		}
		s.deleteFullArchiveFiles(x.ID)
	}
	if err := s.Blobs.ReleaseBlobs(r.Context(), blobIDs); err != nil {
		log.Printf("complete move %s: release blobs: %v", circleID, err)
	}
	s.sendCircleMovedEmails(name, m.DestURL, ownerEmail, emails)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// sendCircleMovedEmails — уведомление «круг переехал» с адресом нового
// сервера (6.35): его шлёт прежний сервер всегда, приглашение — новый.
func (s *Server) sendCircleMovedEmails(circleName, destURL, ownerEmail string, emails []string) {
	if s.Mail == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for _, email := range emails {
			if email == ownerEmail {
				continue
			}
			if err := s.Mail.SendCircleMoved(ctx, email, circleName, destURL); err != nil {
				log.Printf("circle moved mail %s to %s: %v", circleName, email, err)
			}
		}
	}()
}

// dropMoveArchive останавливает сборку архива под перенос и стирает файл.
func (s *Server) dropMoveArchive(ctx context.Context, id string) {
	if cancel, ok := s.fullBuilds.Load("move:" + id); ok {
		cancel.(context.CancelFunc)()
	}
	for _, p := range []string{s.moveArchivePath(id), s.moveArchivePath(id) + ".part"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("move %s: remove archive: %v", id, err)
		}
	}
	if err := s.Chronicle.SetMoveArchive(ctx, id, "none"); err != nil {
		log.Printf("move %s: archive state: %v", id, err)
	}
}

// refuseWhileMoving — пока круг переезжает, его нельзя ни передать, ни
// удалить иначе, чем «Завершить перенос», ни оживить мимо «Отменить перенос»:
// 409 circle_moving.
func (s *Server) refuseWhileMoving(w http.ResponseWriter, r *http.Request, circleID string) bool {
	m, err := s.Chronicle.LiveMove(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return true
	}
	if m != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "circle_moving"})
		return true
	}
	return false
}

// --- Обмен серверов: без входа, по ключу ссылки ---

// moveByRequest — перенос по заголовку «Authorization: Move <ключ>». Нет
// такого — 404 move_not_found: новый сервер говорит «ссылка не подходит».
func (s *Server) moveByRequest(w http.ResponseWriter, r *http.Request) (*chronicle.CircleMove, bool) {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Move ")
	if !ok || strings.TrimSpace(token) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "move_not_found"})
		return nil, false
	}
	m, err := s.Chronicle.MoveByTokenHash(r.Context(), moveTokenHash(token))
	if errors.Is(err, chronicle.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "move_not_found"})
		return nil, false
	}
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	return m, true
}

// moveState — состояние для нового сервера: истёкшая ссылка — «expired»
// (7.24), отменённый или законченный перенос — как есть.
func moveState(m *chronicle.CircleMove, now time.Time) string {
	if (m.Status == chronicle.MoveWaiting || m.Status == chronicle.MovePulling) && !now.Before(m.ExpiresAt) {
		return "expired"
	}
	return m.Status
}

// handleMoveCard — карточка круга для 7.20: что забирает новый сервер.
func (s *Server) handleMoveCard(w http.ResponseWriter, r *http.Request) {
	m, ok := s.moveByRequest(w, r)
	if !ok {
		return
	}
	now := time.Now()
	out := map[string]any{
		"format":         "wynd-move",
		"format_version": 1,
		"status":         moveState(m, now),
		"expires_at":     m.ExpiresAt,
		"server_url":     s.PublicURL(),
	}
	if info, err := s.Auth.Instance(r.Context()); err == nil {
		out["server_name"] = info.Name
	}
	if m.Live() {
		name, err := s.Chronicle.CircleName(r.Context(), m.CircleID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		color, _ := s.Chronicle.CircleColor(r.Context(), m.CircleID)
		bytes, posts, members, err := s.Chronicle.CircleArchiveEstimate(r.Context(), m.CircleID)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		circle := map[string]any{"name": name, "color": color, "posts": posts, "members": members, "bytes": bytes}
		// «Аня — это вы?» на новом сервере (A12): кто владелец и чьи почты в
		// круге. Ссылку даёт сам владелец, и в архиве по ней те же почты.
		if owner, emails, names, err := s.Chronicle.CircleMoveFolk(r.Context(), m.CircleID); err == nil {
			circle["owner_name"] = owner
			circle["emails"] = emails
			circle["names"] = names
		}
		out["circle"] = circle
		if frozen, err := s.Chronicle.CircleFrozenAt(r.Context(), m.CircleID); err == nil && frozen != nil {
			out["frozen_at"] = frozen.UTC().Format(time.RFC3339)
		}
		if schema, err := s.schemaVersion(r.Context()); err == nil {
			out["schema_version"] = schema
		}
	}
	if m.Status == chronicle.MovePulling || m.Status == chronicle.MoveAccepted {
		out["dest_url"] = m.DestURL
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMoveClaim — новый сервер берёт перенос: первый — навсегда, тот же —
// продолжает после обрыва. Архив под перенос начинает собираться.
func (s *Server) handleMoveClaim(w http.ResponseWriter, r *http.Request) {
	m, ok := s.moveByRequest(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		DestURL  string `json:"dest_url"`
		DestName string `json:"dest_name"`
	}](w, r)
	if !ok {
		return
	}
	dest, err := url.Parse(strings.TrimSpace(body.DestURL))
	if err != nil || (dest.Scheme != "http" && dest.Scheme != "https") || dest.Host == "" {
		writeDomainError(w, chronicle.ErrInvalid)
		return
	}
	destURL := dest.Scheme + "://" + dest.Host
	started, err := s.Chronicle.ClaimMove(r.Context(), m.ID, destURL, strings.TrimSpace(body.DestName), time.Now())
	switch {
	case errors.Is(err, chronicle.ErrMoveExpired):
		writeJSON(w, http.StatusGone, map[string]string{"error": "move_expired"})
		return
	case errors.Is(err, chronicle.ErrMoveTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "move_taken"})
		return
	case err != nil:
		writeDomainError(w, err)
		return
	}
	if started {
		s.buildMoveArchive(m.ID, m.CircleID)
		s.notifyCircle(m.CircleID, "", "event")
	}
	m, err = s.Chronicle.GetMove(r.Context(), m.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"archive_status": m.ArchiveStatus, "archive_bytes": m.ArchiveBytes})
}

// buildMoveArchive собирает полный архив без шифрования под перенос, в фоне.
func (s *Server) buildMoveArchive(moveID, circleID string) {
	ctx, cancel := context.WithCancel(context.Background())
	key := "move:" + moveID
	s.fullBuilds.Store(key, cancel)
	go func() {
		defer s.fullBuilds.Delete(key)
		defer cancel()
		if err := s.runMoveArchiveBuild(ctx, moveID, circleID); err != nil {
			if ctx.Err() == nil {
				log.Printf("move %s: build: %v", moveID, err)
			}
			_ = os.Remove(s.moveArchivePath(moveID) + ".part")
			if err := s.Chronicle.SetMoveArchive(context.Background(), moveID, "failed"); err != nil {
				log.Printf("move %s: fail: %v", moveID, err)
			}
		}
	}()
}

func (s *Server) runMoveArchiveBuild(ctx context.Context, moveID, circleID string) error {
	in, err := s.fullArchiveInput(ctx, circleID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(MoveDir(s.DataDir), 0o700); err != nil {
		return err
	}
	part := s.moveArchivePath(moveID) + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	err = writeFullArchive(ctx, bw, in, nil)
	if err == nil {
		err = bw.Flush()
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	fi, err := os.Stat(part)
	if err != nil {
		return err
	}
	if err := os.Rename(part, s.moveArchivePath(moveID)); err != nil {
		return err
	}
	posts, names, media := fullArchiveCounts(in.Data)
	return s.Chronicle.MoveArchiveReady(ctx, moveID, fi.Size(), posts, names, media)
}

// fullArchiveCounts — записей, имён и байт файлов в снимке: то же, что
// считает оживление (ReviveResult), — для сверки «принято».
func fullArchiveCounts(d *chronicle.FullArchiveData) (posts, names int, media int64) {
	for _, t := range d.Tables {
		switch t.Name {
		case "posts":
			posts = len(t.Rows)
		case "identities":
			names = len(t.Rows)
		case "blobs":
			col := -1
			for i, c := range t.Columns {
				if c == "size_bytes" {
					col = i
				}
			}
			for _, row := range t.Rows {
				if col >= 0 {
					media += jsonInt64(row[col])
				}
			}
		}
	}
	return posts, names, media
}

// handleMoveArchive отдаёт архив под перенос с докачкой (Range). Пока он
// собирается — 409 building; сбой — 409 failed (новый сервер берёт перенос
// снова, сборка начинается заново).
func (s *Server) handleMoveArchive(w http.ResponseWriter, r *http.Request) {
	m, ok := s.moveByRequest(w, r)
	if !ok {
		return
	}
	if !m.LinkAlive(time.Now()) || m.Status != chronicle.MovePulling {
		writeJSON(w, http.StatusGone, map[string]string{"error": "move_expired"})
		return
	}
	if m.ArchiveStatus != "ready" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "archive_" + m.ArchiveStatus})
		return
	}
	f, err := os.Open(s.moveArchivePath(m.ID))
	if err != nil {
		log.Printf("move %s: open archive: %v", m.ID, err)
		_ = s.Chronicle.SetMoveArchive(r.Context(), m.ID, "failed")
		writeJSON(w, http.StatusConflict, map[string]string{"error": "archive_failed"})
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	rec := &rangeRecorder{ResponseWriter: w}
	http.ServeContent(rec, r, "circle.wynd", fi.ModTime(), f)
	// Ход для 6.34: докуда дошёл этот кусок.
	var start, end, total int64
	switch rec.status {
	case http.StatusOK:
		end = rec.written
	case http.StatusPartialContent:
		if _, err := fmt.Sscanf(rec.Header().Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err == nil {
			end = start + rec.written
		}
	}
	if end > 0 {
		if err := s.Chronicle.SetMovePulled(context.Background(), m.ID, end); err != nil {
			log.Printf("move %s: pulled: %v", m.ID, err)
		}
	}
}

// handleMoveAccepted — новый сервер поднял круг и сверил (6.35). Числа
// сверяются с архивом под перенос: не сошлись — 409, перенос не принят.
func (s *Server) handleMoveAccepted(w http.ResponseWriter, r *http.Request) {
	m, ok := s.moveByRequest(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		Posts int   `json:"posts"`
		Names int   `json:"names"`
		Bytes int64 `json:"bytes"`
	}](w, r)
	if !ok {
		return
	}
	if m.Status == chronicle.MoveAccepted {
		writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
		return
	}
	if m.Status != chronicle.MovePulling || m.ArchiveStatus != "ready" {
		writeJSON(w, http.StatusGone, map[string]string{"error": "move_expired"})
		return
	}
	if body.Posts != m.ArchivePosts || body.Names != m.ArchiveNames || body.Bytes != m.ArchiveMediaBytes {
		log.Printf("move %s: accepted with mismatch: %d/%d posts, %d/%d names, %d/%d bytes", m.ID,
			body.Posts, m.ArchivePosts, body.Names, m.ArchiveNames, body.Bytes, m.ArchiveMediaBytes)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "mismatch"})
		return
	}
	host := m.DestURL
	if u, err := url.Parse(m.DestURL); err == nil && u.Host != "" {
		host = u.Host
	}
	if err := s.Chronicle.AcceptMove(r.Context(), m.ID, body.Posts, body.Names, body.Bytes, host, time.Now()); err != nil {
		writeDomainError(w, err)
		return
	}
	s.dropMoveArchive(r.Context(), m.ID)
	s.notifyCircle(m.CircleID, "", "event")
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// ExpireMoveArchives стирает архивы переносов, которые больше не нужны:
// ссылка истекла или перенос закончился (зовётся ежечасно).
func (s *Server) ExpireMoveArchives(ctx context.Context, now time.Time) int {
	ids, err := s.Chronicle.EndedMoveArchives(ctx, now)
	if err != nil {
		log.Printf("move expire: %v", err)
		return 0
	}
	for _, id := range ids {
		s.dropMoveArchive(ctx, id)
	}
	return len(ids)
}

// RecoverMoveArchives — при старте: недособранные архивы переносов — сбой
// (новый сервер возьмёт перенос снова), лишние файлы стираются.
func (s *Server) RecoverMoveArchives(ctx context.Context) {
	keep, err := s.Chronicle.RecoverMoveArchives(ctx, time.Now())
	if err != nil {
		log.Printf("move recover: %v", err)
		return
	}
	names := map[string]bool{}
	for _, id := range keep {
		names["out-"+id+".wynd"] = true
	}
	entries, err := os.ReadDir(MoveDir(s.DataDir))
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "out-") && !names[e.Name()] {
			_ = os.Remove(filepath.Join(MoveDir(s.DataDir), e.Name()))
		}
	}
}
