package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/version"
)

// Полный архив круга (план 51, A1): владелец просит собрать файл со всем
// кругом (6.29), сервер собирает его в фоне (6.40) и держит до скачивания или
// до срока (6.33); места под готовый файл нет — отдаёт потоком (6.43).

// FullArchiveDir — где лежат готовые полные архивы. Не DataDir/tmp: её чистит
// запуск, а готовый архив ждёт скачивания и через перезапуск.
func FullArchiveDir(dataDir string) string {
	return filepath.Join(dataDir, "archives")
}

func (s *Server) fullArchivePath(id string) string {
	return filepath.Join(FullArchiveDir(s.DataDir), id+".wynd")
}

// fullLinkTTL — сколько живёт ссылка на готовый архив. Не одноразовая, как у
// архива участника: оборванное скачивание браузер докачивает по той же
// ссылке (план 46: «докачка работает»), а файл лежит, пока его не скачали.
const fullLinkTTL = 24 * time.Hour

// fullStreamLinkTTL — ссылка на сборку потоком (6.43): приложение открывает
// её сразу, работает она один раз.
const fullStreamLinkTTL = time.Minute

type fullLink struct {
	archiveID, circleID, accountID string
	key                            *archive.KeyParams
	stream                         bool
	expires                        time.Time
}

type fullArchiveKeyBody struct {
	PublicKey string `json:"public_key"`
	Salt      string `json:"salt"`
	LogN      int    `json:"log_n"`
}

type fullArchiveStartBody struct {
	DeleteCircle bool                `json:"delete_circle"`
	Key          *fullArchiveKeyBody `json:"key"`
}

func (b *fullArchiveKeyBody) params() (*archive.KeyParams, error) {
	if b == nil {
		return nil, nil
	}
	pub, err1 := base64.StdEncoding.DecodeString(b.PublicKey)
	salt, err2 := base64.StdEncoding.DecodeString(b.Salt)
	if err1 != nil || err2 != nil {
		return nil, chronicle.ErrInvalid
	}
	k := &archive.KeyParams{PublicKey: pub, Salt: salt, LogN: b.LogN}
	// Проверка ключа — та же, что при шифровании: плохой ключ — отказ сразу,
	// а не сборка, которая упадёт через полчаса.
	if _, err := archive.EncryptedSize(0, *k); err != nil {
		return nil, chronicle.ErrInvalid
	}
	return k, nil
}

func fullArchiveJSON(a *chronicle.FullArchive) map[string]any {
	if a == nil {
		return nil
	}
	out := map[string]any{
		"id": a.ID, "status": a.Status, "requested_at": a.RequestedAt,
		"delete_circle": a.DeleteCircle, "encrypted": a.Encrypted, "size_bytes": a.SizeBytes,
	}
	put := func(k string, t *time.Time) {
		if t != nil {
			out[k] = *t
		}
	}
	put("snapshot_at", a.SnapshotAt)
	put("ready_at", a.ReadyAt)
	put("expires_at", a.ExpiresAt)
	put("downloaded_at", a.DownloadedAt)
	put("removed_at", a.RemovedAt)
	if a.RemovedReason != "" {
		out["removed_reason"] = a.RemovedReason
	}
	return out
}

// handleFullArchive — что с полным архивом круга: последний запрос и оценка
// для экрана «Полный архив круга» (6.29: «340 записей… · 8,4 ГБ»; 6.43:
// помещается ли готовый файл на сервер).
func (s *Server) handleFullArchive(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	estimate, posts, members, err := s.Chronicle.CircleArchiveEstimate(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	room, err := s.Blobs.RoomBytes(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	latest, err := s.Chronicle.LatestFullArchive(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	retention, err := s.Chronicle.ArchiveRetention(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	frozen, err := s.Chronicle.CircleFrozenAt(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"frozen_at":       frozen,
		"estimate_bytes":  estimate,
		"posts":           posts,
		"members":         members,
		"fits":            estimate <= room,
		"retention_hours": int(retention / time.Hour),
		"archive":         fullArchiveJSON(latest),
	})
}

// handleStartFullArchive — «Собрать архив» (6.29): запрос записан, место под
// оценку занято, сборка пошла в фоне. Места нет — 507 no_room (6.43).
func (s *Server) handleStartFullArchive(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, ok := bindJSON[fullArchiveStartBody](w, r)
	if !ok {
		return
	}
	key, err := body.Key.params()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	estimate, _, _, err := s.Chronicle.CircleArchiveEstimate(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	room, err := s.Blobs.RoomBytes(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if estimate > room {
		writeJSON(w, http.StatusInsufficientStorage, map[string]string{"error": "no_room"})
		return
	}
	a, froze, err := s.Chronicle.StartFullArchive(r.Context(), circleID, sess.AccountID, body.DeleteCircle, key != nil, estimate, time.Now())
	if errors.Is(err, chronicle.ErrArchiveBusy) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "archive_busy"})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if froze {
		s.circleFrozen(circleID, sess.AccountID)
	}
	s.buildFullArchive(a, key)
	writeJSON(w, http.StatusCreated, fullArchiveJSON(a))
}

// circleFrozen — круг замер (6.32): участникам одно письмо, экранам —
// сигнал перечитать круг (баннер 6.36, строка 2.29).
func (s *Server) circleFrozen(circleID, ownerAccountID string) {
	s.sendCircleLeavingEmails(circleID, ownerAccountID)
	s.notifyCircle(circleID, ownerAccountID, "event")
}

// sendCircleLeavingEmails — единственное письмо участникам о замершем круге
// (решение владельца 2026-10-12: сроков и напоминаний нет).
func (s *Server) sendCircleLeavingEmails(circleID, ownerAccountID string) {
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
			log.Printf("circle leaving mail %s: %v", circleID, err)
			return
		}
		owner, _ := s.Chronicle.OwnerName(ctx, circleID)
		ownerEmail, _ := s.Chronicle.AccountEmail(ctx, ownerAccountID)
		emails, err := s.Chronicle.CircleMemberEmails(ctx, circleID)
		if err != nil {
			log.Printf("circle leaving mail %s: members: %v", circleID, err)
			return
		}
		for _, email := range emails {
			if email == ownerEmail {
				continue
			}
			if err := s.Mail.SendCircleLeaving(ctx, email, name, owner, link); err != nil {
				log.Printf("circle leaving mail %s to %s: %v", circleID, email, err)
			}
		}
	}()
}

// handleReviveFrozenCircle — «Отменить — оживить круг» (6.34, 6.30, 6.41): архив
// убирается, если он ещё на сервере, круг снова пишет.
func (s *Server) handleReviveFrozenCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := s.Chronicle.LatestFullArchive(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if a != nil && (a.Status == chronicle.FullArchiveBuilding || a.Status == chronicle.FullArchiveReady) {
		reason := chronicle.RemovedOwner
		if a.Status == chronicle.FullArchiveBuilding {
			reason = chronicle.RemovedCanceled
		}
		s.removeFullArchive(r.Context(), a.ID, reason)
	}
	if err := s.Chronicle.UnfreezeCircle(r.Context(), circleID, sess.AccountID, time.Now()); err != nil {
		writeDomainError(w, err)
		return
	}
	s.notifyCircle(circleID, sess.AccountID, "event")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleRemoveFrozenCircle — «Убрать круг с сервера» (6.30): только когда
// файл архива с удалением скачан до последнего байта — без него круг не
// вернуть.
func (s *Server) handleRemoveFrozenCircle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := s.Chronicle.LatestFullArchive(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if a == nil || !a.DeleteCircle || a.DownloadedAt == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "not_downloaded"})
		return
	}
	name, err := s.Chronicle.CircleName(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	archives, err := s.Chronicle.FullArchivesWithFiles(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
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
		log.Printf("remove frozen circle %s: release blobs: %v", circleID, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleRemoveFullArchive — «Отменить сборку» (6.40) и «Удалить архив, не
// скачивая» (6.33): файл стирается, место освобождается.
func (s *Server) handleRemoveFullArchive(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := s.Chronicle.LatestFullArchive(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if a == nil || (a.Status != chronicle.FullArchiveBuilding && a.Status != chronicle.FullArchiveReady) {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	reason := chronicle.RemovedOwner
	if a.Status == chronicle.FullArchiveBuilding {
		reason = chronicle.RemovedCanceled
	}
	s.removeFullArchive(r.Context(), a.ID, reason)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// removeFullArchive останавливает сборку, если она идёт, отмечает причину и
// стирает файл.
func (s *Server) removeFullArchive(ctx context.Context, id, reason string) {
	if cancel, ok := s.fullBuilds.Load(id); ok {
		cancel.(context.CancelFunc)()
	}
	removed, err := s.Chronicle.RemoveFullArchive(ctx, id, reason, time.Now())
	if err != nil {
		log.Printf("full archive %s: remove (%s): %v", id, reason, err)
		return
	}
	if removed {
		s.deleteFullArchiveFiles(id)
	}
}

func (s *Server) deleteFullArchiveFiles(id string) {
	for _, p := range []string{s.fullArchivePath(id), s.fullArchivePath(id) + ".part"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("full archive %s: remove file: %v", id, err)
		}
	}
}

// handleFullArchiveLink — ссылка на готовый файл: по ней качает сам браузер,
// со своим ходом и докачкой.
func (s *Server) handleFullArchiveLink(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	a, err := s.Chronicle.LatestFullArchive(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if a == nil || a.Status != chronicle.FullArchiveReady {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	token, err := s.issueFullLink(fullLink{archiveID: a.ID, circleID: circleID, accountID: sess.AccountID}, fullLinkTTL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": "/api/v1/full_archive/" + token})
}

// handleFullArchiveStreamLink — «Скачать сразу» (6.43): готовый файл положить
// некуда, сервер собирает архив прямо в скачивание.
func (s *Server) handleFullArchiveStreamLink(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, ok := bindJSON[fullArchiveStartBody](w, r)
	if !ok {
		return
	}
	key, err := body.Key.params()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	link := fullLink{circleID: circleID, accountID: sess.AccountID, key: key, stream: true}
	if body.DeleteCircle {
		// С удалением (6.43 при включённом переключателе): круг замирает до
		// снимка, а «скачано» записывается, когда поток дошёл до конца, — тогда
		// откроется 6.30. Места поток не занимает: размер записи — ноль.
		a, froze, err := s.Chronicle.StartFullArchive(r.Context(), circleID, sess.AccountID, true, key != nil, 0, time.Now())
		if errors.Is(err, chronicle.ErrArchiveBusy) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "archive_busy"})
			return
		}
		if err != nil {
			writeDomainError(w, err)
			return
		}
		if froze {
			s.circleFrozen(circleID, sess.AccountID)
		}
		link.archiveID = a.ID
	}
	token, err := s.issueFullLink(link, fullStreamLinkTTL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": "/api/v1/full_archive_stream/" + token})
}

func (s *Server) issueFullLink(l fullLink, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	now := time.Now()
	s.fullLinks.Range(func(k, v any) bool {
		if now.After(v.(fullLink).expires) {
			s.fullLinks.Delete(k)
		}
		return true
	})
	l.expires = now.Add(ttl)
	s.fullLinks.Store(token, l)
	return token, nil
}

// handleFullArchiveByLink отдаёт готовый файл — с докачкой (Range). Дошёл
// последний байт — файл стирается: «скачал» — когда до устройства дошёл
// конец файла (6.33), оборванное скачивание файл не трогает.
func (s *Server) handleFullArchiveByLink(w http.ResponseWriter, r *http.Request) {
	v, ok := s.fullLinks.Load(r.PathValue("token"))
	if !ok || time.Now().After(v.(fullLink).expires) || v.(fullLink).stream {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	link := v.(fullLink)
	a, err := s.Chronicle.GetFullArchive(r.Context(), link.archiveID)
	if err != nil || a.Status != chronicle.FullArchiveReady {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	f, err := os.Open(s.fullArchivePath(a.ID))
	if err != nil {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		writeError(w, err)
		return
	}
	name, err := s.Chronicle.CircleName(r.Context(), a.CircleID)
	if err != nil {
		name = "circle"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(name+".wynd"))
	w.Header().Set("Cache-Control", "no-store")
	rec := &rangeRecorder{ResponseWriter: w}
	http.ServeContent(rec, r, "", fi.ModTime(), f)
	// Закрыть до стирания: открытый файл Windows не стирает.
	_ = f.Close()
	if r.Method != http.MethodHead && rec.reachedEnd(fi.Size()) {
		s.fullLinks.Delete(r.PathValue("token"))
		s.removeFullArchive(context.WithoutCancel(r.Context()), a.ID, chronicle.RemovedDownloaded)
	}
}

// handleFullArchiveStream собирает полный архив прямо в ответ (6.43): докачки
// нет, на сервере ничего не остаётся.
func (s *Server) handleFullArchiveStream(w http.ResponseWriter, r *http.Request) {
	v, ok := s.fullLinks.LoadAndDelete(r.PathValue("token"))
	if !ok || time.Now().After(v.(fullLink).expires) || !v.(fullLink).stream {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	link := v.(fullLink)
	// Запись архива с удалением: поток не дошёл — сбой, и владелец видит
	// экран сборки снова; дошёл — «скачано» (6.30).
	finished := false
	if link.archiveID != "" {
		defer func() {
			if !finished {
				_ = s.Chronicle.FailFullArchive(context.WithoutCancel(r.Context()), link.archiveID)
			}
		}()
	}
	in, err := s.fullArchiveInput(r.Context(), link.circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if link.archiveID != "" {
		if err := s.Chronicle.SetFullArchiveSnapshot(r.Context(), link.archiveID, in.Data.SnapshotAt); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	size, err := archive.FullSize(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	if link.key != nil {
		if size, err = archive.EncryptedSize(size, *link.key); err != nil {
			writeDomainError(w, chronicle.ErrInvalid)
			return
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(in.Data.CircleName+".wynd"))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	rec := &deliveryRecorder{ResponseWriter: w}
	rec.WriteHeader(http.StatusOK)
	if err := writeFullArchive(r.Context(), rec, in, link.key); err != nil || rec.written != size {
		log.Printf("full archive %s: stream: %v (written %d of %d)", link.circleID, err, rec.written, size)
		panic(http.ErrAbortHandler)
	}
	if link.archiveID != "" && rec.deliveredToEnd(size) {
		if _, err := s.Chronicle.RemoveFullArchive(context.WithoutCancel(r.Context()), link.archiveID, chronicle.RemovedDownloaded, time.Now()); err != nil {
			log.Printf("full archive %s: stream downloaded: %v", link.archiveID, err)
		}
		finished = true
	}
}

// writeFullArchive пишет архив в w — зашифрованным, если есть ключ.
func writeFullArchive(ctx context.Context, w io.Writer, in archive.FullInput, key *archive.KeyParams) error {
	if key == nil {
		return archive.BuildFullArchive(ctx, w, in)
	}
	ew, err := archive.Encrypt(w, *key)
	if err != nil {
		return err
	}
	if err := archive.BuildFullArchive(ctx, ew, in); err != nil {
		return err
	}
	return ew.Close()
}

// fullArchiveInput снимает круг: строки базы одной транзакцией, лента и
// названия дней — на тот же момент.
func (s *Server) fullArchiveInput(ctx context.Context, circleID string) (archive.FullInput, error) {
	data, err := s.Chronicle.FullArchiveData(ctx, circleID)
	if err != nil {
		return archive.FullInput{}, err
	}
	posts, err := s.Chronicle.FullArchiveFeed(ctx, circleID, data.SnapshotAt)
	if err != nil {
		return archive.FullInput{}, err
	}
	dayTitles, err := s.Chronicle.FullArchiveDayTitles(ctx, circleID, data.SnapshotAt)
	if err != nil {
		return archive.FullInput{}, err
	}
	// Аватары всех лиц круга, не только тех, кто писал: ушедшие тоже в архиве.
	var identities []string
	for _, t := range data.Tables {
		if t.Name != "identities" {
			continue
		}
		col := slices.Index(t.Columns, "id")
		for _, row := range t.Rows {
			if id, ok := row[col].(string); ok {
				identities = append(identities, id)
			}
		}
	}
	avatars, err := s.Chronicle.IdentityAvatarBlobIDs(ctx, identities)
	if err != nil {
		return archive.FullInput{}, err
	}
	schema, err := s.schemaVersion(ctx)
	if err != nil {
		return archive.FullInput{}, err
	}
	return archive.FullInput{
		Manifest: archive.FullManifest{
			WyndVersion: version.Number, SchemaVersion: schema, Server: s.PublicURL(),
		},
		Data: data,
		Readable: archive.BuildInput{
			CircleName: data.CircleName,
			CutoffDate: data.SnapshotAt.Format("2006-01-02"),
			Posts:      posts,
			Blobs:      s.Blobs,
			Avatars:    avatars,
			DayTitles:  dayTitles,
			Color:      data.Color,
			Fonts:      s.ArchiveFonts,
		},
	}, nil
}

func (s *Server) schemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.Chronicle.DB().QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}

// buildFullArchive собирает файл в фоне. Ключ живёт только в памяти сборки:
// перезапуск сервера посреди неё — сбой, владелец собирает снова.
func (s *Server) buildFullArchive(a *chronicle.FullArchive, key *archive.KeyParams) {
	ctx, cancel := context.WithCancel(context.Background())
	s.fullBuilds.Store(a.ID, cancel)
	go func() {
		defer s.fullBuilds.Delete(a.ID)
		defer cancel()
		if err := s.runFullArchiveBuild(ctx, a, key); err != nil {
			if ctx.Err() == nil {
				log.Printf("full archive %s: build: %v", a.ID, err)
			}
			s.deleteFullArchiveFiles(a.ID)
			if err := s.Chronicle.FailFullArchive(context.Background(), a.ID); err != nil {
				log.Printf("full archive %s: fail: %v", a.ID, err)
			}
		}
	}()
}

func (s *Server) runFullArchiveBuild(ctx context.Context, a *chronicle.FullArchive, key *archive.KeyParams) error {
	in, err := s.fullArchiveInput(ctx, a.CircleID)
	if err != nil {
		return err
	}
	if err := s.Chronicle.SetFullArchiveSnapshot(ctx, a.ID, in.Data.SnapshotAt); err != nil {
		return err
	}
	if err := os.MkdirAll(FullArchiveDir(s.DataDir), 0o700); err != nil {
		return err
	}
	part := s.fullArchivePath(a.ID) + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	err = writeFullArchive(ctx, bw, in, key)
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
	if err := os.Rename(part, s.fullArchivePath(a.ID)); err != nil {
		return err
	}
	retention, err := s.Chronicle.ArchiveRetention(ctx)
	if err != nil {
		return err
	}
	ok, err := s.Chronicle.FinishFullArchive(ctx, a.ID, fi.Size(), time.Now(), retention)
	if err != nil {
		return err
	}
	if !ok {
		// Отменили, пока дописывался последний кусок.
		s.deleteFullArchiveFiles(a.ID)
		return nil
	}
	s.sendFullArchiveReady(a, in.Data.CircleName)
	return nil
}

// sendFullArchiveReady — письмо владельцу: файл готов (6.40: «Когда файл
// будет готов, придёт письмо»).
func (s *Server) sendFullArchiveReady(a *chronicle.FullArchive, circleName string) {
	if s.Mail == nil {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/circles/" + a.CircleID + "/quota/full"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		email, err := s.Chronicle.AccountEmail(ctx, a.RequestedBy)
		if err != nil {
			log.Printf("full archive %s: owner mail: %v", a.ID, err)
			return
		}
		ready, err := s.Chronicle.GetFullArchive(ctx, a.ID)
		if err != nil || ready.ExpiresAt == nil {
			log.Printf("full archive %s: ready mail: %v", a.ID, err)
			return
		}
		if err := s.Mail.SendFullArchiveReady(ctx, email, circleName, *ready.ExpiresAt, link); err != nil {
			log.Printf("full archive %s: ready mail to %s: %v", a.ID, email, err)
		}
	}()
}

// ExpireFullArchives стирает готовые архивы, срок которых вышел (зовётся
// ежечасно рядом с работами цикла архивации).
func (s *Server) ExpireFullArchives(ctx context.Context, now time.Time) int {
	ids, err := s.Chronicle.ExpiredFullArchives(ctx, now)
	if err != nil {
		log.Printf("full archive expire: %v", err)
		return 0
	}
	for _, id := range ids {
		s.removeFullArchive(ctx, id, chronicle.RemovedExpired)
	}
	return len(ids)
}

// RecoverFullArchives — при старте: прерванные сборки — сбой, а в каталоге
// архивов остаётся только то, что ждёт скачивания.
func (s *Server) RecoverFullArchives(ctx context.Context) {
	if ids, err := s.Chronicle.FailInterruptedFullArchives(ctx); err != nil {
		log.Printf("full archive recover: %v", err)
	} else if len(ids) > 0 {
		log.Printf("full archive: %d builds interrupted by restart", len(ids))
	}
	keep := map[string]bool{}
	ready, err := s.Chronicle.FullArchivesWithFiles(ctx, "")
	if err != nil {
		log.Printf("full archive recover: %v", err)
		return
	}
	for _, a := range ready {
		keep[a.ID+".wynd"] = true
	}
	entries, err := os.ReadDir(FullArchiveDir(s.DataDir))
	if err != nil {
		return
	}
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(FullArchiveDir(s.DataDir), e.Name()))
		}
	}
}

// rangeRecorder запоминает, что ушло клиенту: дошёл ли ответ (весь файл или
// его последний кусок по Range) до последнего байта.
type rangeRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (d *rangeRecorder) WriteHeader(status int) {
	d.status = status
	d.ResponseWriter.WriteHeader(status)
}

func (d *rangeRecorder) Write(p []byte) (int, error) {
	if d.status == 0 {
		d.status = http.StatusOK
	}
	n, err := d.ResponseWriter.Write(p)
	d.written += int64(n)
	return n, err
}

func (d *rangeRecorder) reachedEnd(size int64) bool {
	switch d.status {
	case http.StatusOK:
		return d.written == size
	case http.StatusPartialContent:
		// «bytes A-B/size»: кусок кончается последним байтом и ушёл весь.
		var start, end, total int64
		cr := d.Header().Get("Content-Range")
		if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &total); err != nil {
			return false
		}
		return total == size && end == size-1 && d.written == end-start+1
	}
	return false
}

// contentDisposition — имя файла для сохранения: кириллица по RFC 5987,
// латинская запасная копия для старых браузеров.
func contentDisposition(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '/' {
			return '_'
		}
		return r
	}, name)
	return `attachment; filename="` + ascii + `"; filename*=UTF-8''` + url.PathEscape(name)
}
