package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) archiveCycleJSON(ctx context.Context, circleID, accountID string) (map[string]any, error) {
	cycle, err := s.Chronicle.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return nil, err
	}
	if !cycle.Active {
		return nil, nil
	}
	stats, err := s.Chronicle.EstimateArchivePersonal(ctx, circleID, accountID, cycle.CutoffDate)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"active":                       true,
		"cutoff_date":                  cycle.CutoffDate,
		"deadline":                     cycle.Deadline.UTC().Format(time.RFC3339),
		"reminder_before_sec":          cycle.ReminderBeforeSec,
		"cutoff_locked":                cycle.CutoffLockedAt != nil,
		"personal_archive_bytes":       stats.MediaBytes,
		"personal_archive_media_count": stats.MediaFiles,
		"personal_archive_post_count":  stats.PostCount,
		"download_url":                 "/api/v1/circles/" + circleID + "/archive/download",
	}
	return out, nil
}

func (s *Server) handleCircleDetail(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var found *chronicle.CircleSummary
	for i := range circles {
		if circles[i].ID == circleID {
			found = &circles[i]
			break
		}
	}
	if found == nil {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	// Карточка — тем, кто круг читает: исключённому список кругов и так
	// сообщает «gone», а настройки приглашений и лицо ему больше не отдаются
	// (аудит 2026-09-27).
	if err := s.Chronicle.RequireReader(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	banner, err := s.archiveCycleJSON(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	mem, err := s.Chronicle.MembershipForAccount(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	owner, err := s.Chronicle.CircleOwnerAccount(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	editWindow, err := s.Chronicle.CircleEditWindow(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	color, err := s.Chronicle.CircleColor(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	identityName, err := s.Chronicle.ResolveIdentityName(r.Context(), mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	avatarBlobID, err := s.Chronicle.ResolveIdentityAvatar(r.Context(), mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	inviteSettings, err := s.Chronicle.GetInviteSettings(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"id":                  found.ID,
		"name":                found.Name,
		"color":               color,
		"status":              string(found.Status),
		"edit_window_sec":     editWindow.Seconds,
		"is_owner":            owner == sess.AccountID,
		"can_settings":        mem.CanSettings,
		"share_place":         mem.SharePlace,
		"identity_id":         mem.IdentityID,
		"identity_name":       identityName,
		"invite_who":          inviteSettings.InviteWho,
		"invite_kind_default": inviteSettings.InviteKindDefault,
	}
	if avatarBlobID != "" {
		out["avatar_blob_id"] = avatarBlobID
	}
	if banner != nil {
		out["archive_cycle"] = banner
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCircleQuota(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	used, err := s.Blobs.CircleUsedBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	quota, err := s.Blobs.CircleQuotaBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	volume, err := s.Chronicle.MediaVolumeChart(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	median, err := s.Chronicle.MedianPostBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	postCount, err := s.Chronicle.CountCirclePosts(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"used_bytes": used,
		"post_count": postCount,
		"volume":     volume,
	}
	if quota.Valid {
		out["quota_bytes"] = quota.Int64
	}
	if median > 0 {
		out["median_post_bytes"] = median
	}
	if cutoff := r.URL.Query().Get("cutoff_date"); cutoff != "" {
		freed, err := s.Chronicle.FreedBytesBeforeCutoff(r.Context(), circleID, cutoff)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out["freed_at_cutoff_bytes"] = freed
	}
	writeJSON(w, http.StatusOK, out)
}

type startArchiveBody struct {
	CutoffDate        string `json:"cutoff_date"`
	Deadline          string `json:"deadline"`
	ReminderBeforeSec int64  `json:"reminder_before_sec"`
}

func (s *Server) handleStartArchiveCycle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[startArchiveBody](w, r)
	if !ok {
		return
	}
	deadline, err := time.Parse(time.RFC3339, body.Deadline)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	now := time.Now().UTC()
	if err := s.Chronicle.StartArchiveCycle(r.Context(), circleID, sess.AccountID, body.CutoffDate, deadline, body.ReminderBeforeSec, now); err != nil {
		writeDomainError(w, err)
		return
	}
	s.sendArchiveCycleStartEmails(circleID)
	s.notifyCircle(circleID, sess.AccountID, "event")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type moveCutoffBody struct {
	CutoffDate string `json:"cutoff_date"`
}

func (s *Server) handleMoveCutoff(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[moveCutoffBody](w, r)
	if !ok {
		return
	}
	err := s.Chronicle.MoveCutoff(r.Context(), circleID, sess.AccountID, body.CutoffDate, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type moveDeadlineBody struct {
	Deadline string `json:"deadline"`
}

func (s *Server) handleMoveDeadline(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[moveDeadlineBody](w, r)
	if !ok {
		return
	}
	deadline, err := time.Parse(time.RFC3339, body.Deadline)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	err = s.Chronicle.MoveDeadline(r.Context(), circleID, sess.AccountID, deadline, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// archiveRetryAfterSec — подсказка клиенту, когда повторить скачивание архива,
// если предыдущая сборка той же учётки ещё идёт.
const archiveRetryAfterSec = 30

func (s *Server) handleArchiveDownload(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	cycle, err := s.Chronicle.GetArchiveCycle(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !cycle.Active {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	now := time.Now().UTC()
	if !cycle.Deadline.IsZero() && now.After(cycle.Deadline) {
		writeDomainError(w, chronicle.ErrForbidden)
		return
	}
	layout := archive.LayoutFeed
	if v := strings.TrimSpace(r.URL.Query().Get("layout")); v == "posts" {
		layout = archive.LayoutPosts
	}
	if _, busy := s.archiveBuilds.LoadOrStore(sess.AccountID, struct{}{}); busy {
		writeError(w, &auth.RateLimitError{RetryAfterSec: archiveRetryAfterSec})
		return
	}
	defer s.archiveBuilds.Delete(sess.AccountID)
	posts, err := s.Chronicle.ArchiveSnapshot(r.Context(), circleID, sess.AccountID, cycle.CutoffDate)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	circleName, circleColor := circleID, ""
	for _, c := range circles {
		if c.ID == circleID {
			circleName, circleColor = c.Name, c.Color
			break
		}
	}
	avatars, err := s.Chronicle.IdentityAvatarBlobIDs(r.Context(), chronicle.IdentityIDsFromFeed(posts))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	dayTitles, err := s.Chronicle.ArchiveDayTitles(r.Context(), circleID, cycle.CutoffDate, posts)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Сборка — во временный файл каталога данных, отдача — ServeContent:
	// память не растёт с размером круга, а обрыв докачивается Range-запросом
	// (ARC-1). Сборка детерминирована, поэтому повторная даёт те же байты.
	f, err := s.createArchiveTemp()
	if err != nil {
		writeError(w, err)
		return
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if err := archive.BuildPersonalArchive(r.Context(), f, archive.BuildInput{
		CircleName: circleName,
		CutoffDate: cycle.CutoffDate,
		Layout:     layout,
		Posts:      posts,
		Blobs:      s.Blobs,
		Avatars:    avatars,
		DayTitles:  dayTitles,
		Color:      circleColor,
		Fonts:      s.ArchiveFonts,
	}); err != nil {
		writeError(w, err)
		return
	}
	info, err := f.Stat()
	if err != nil {
		writeError(w, err)
		return
	}
	var modtime time.Time
	if cycle.CycleStartedAt != nil {
		modtime = *cycle.CycleStartedAt
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"wynd-archive-"+circleID+".zip\"")
	rec := &deliveryRecorder{ResponseWriter: w}
	http.ServeContent(rec, r, "", modtime, f)

	// Отсечка замирает, когда участник получил архив целиком — последний байт
	// ушёл, — а не когда архив собран: обрыв на отдаче замораживал её, хотя
	// никто ничего не получил (ARC-8). Докачка по Range засчитывается запросом,
	// который дошёл до конца файла.
	if r.Method == http.MethodGet && rec.deliveredToEnd(info.Size()) {
		if _, err := s.Chronicle.LockCutoff(context.WithoutCancel(r.Context()), circleID, cycle.CutoffDate, now); err != nil {
			log.Printf("archive %s: lock cutoff: %v", circleID, err)
		}
	}
}

// deliveryRecorder считает, сколько байт тела ушло клиенту, и запоминает
// статус: по ним видно, дошёл ли ответ до последнего байта файла.
type deliveryRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (d *deliveryRecorder) WriteHeader(status int) {
	d.status = status
	d.ResponseWriter.WriteHeader(status)
}

func (d *deliveryRecorder) Write(p []byte) (int, error) {
	if d.status == 0 {
		d.status = http.StatusOK
	}
	n, err := d.ResponseWriter.Write(p)
	d.written += int64(n)
	return n, err
}

// deliveredToEnd: 200 — ушёл весь файл; 206 — ушёл диапазон, кончающийся
// последним байтом файла.
func (d *deliveryRecorder) deliveredToEnd(size int64) bool {
	switch d.status {
	case http.StatusOK:
		return d.written == size
	case http.StatusPartialContent:
		var first, last, total int64
		if _, err := fmt.Sscanf(d.Header().Get("Content-Range"), "bytes %d-%d/%d", &first, &last, &total); err != nil {
			return false
		}
		return total == size && last == size-1 && d.written == last-first+1
	}
	return false
}

// ArchiveTempDir — каталог недособранных архивов внутри каталога данных. Его
// содержимое живёт только на время запроса; сервер чистит его при старте.
func ArchiveTempDir(dataDir string) string {
	if dataDir == "" {
		return filepath.Join(os.TempDir(), "wynd-archive")
	}
	return filepath.Join(dataDir, "tmp")
}

func (s *Server) createArchiveTemp() (*os.File, error) {
	dir := ArchiveTempDir(s.DataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, "archive-*.zip")
}

// sendArchiveCycleStartEmails рассылает письма о старте цикла в фоне.
//
// Раньше рассылка шла в запросе владельца (до 30 с на медленном релее), а
// ошибки отбрасывались молча — участник мог не узнать о сроке, после которого
// его записи сотрутся, и этого не видел никто. Теперь владелец получает ответ
// сразу, а каждый сбой — строка в журнале (план 42, ARC-9); остановка сервера
// дожидается рассылки через notifyWG.
func (s *Server) sendArchiveCycleStartEmails(circleID string) {
	if s.Mail == nil {
		return
	}
	download := strings.TrimRight(s.PublicURL(), "/") + "/api/v1/circles/" + circleID + "/archive/download"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cycle, err := s.Chronicle.GetArchiveCycle(ctx, circleID)
		if err != nil || !cycle.Active {
			log.Printf("archive start mail %s: cycle: %v", circleID, err)
			return
		}
		emails, err := s.Chronicle.CircleMemberEmails(ctx, circleID)
		if err != nil {
			log.Printf("archive start mail %s: members: %v", circleID, err)
			return
		}
		for _, email := range emails {
			if err := s.Mail.SendArchiveCycleStart(ctx, email, cycle.CutoffDate, cycle.Deadline, download); err != nil {
				log.Printf("archive start mail %s to %s: %v", circleID, email, err)
			}
		}
	}()
}
