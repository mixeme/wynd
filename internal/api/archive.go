package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	s.addFrozen(r, out, circleID, sess.AccountID)
	// «Отклики» (3.17): есть ли вкладка и сколько на ней нового.
	if others, err := s.Chronicle.CircleHasOthers(r.Context(), circleID, sess.AccountID); err == nil {
		out["has_others"] = others
	}
	if n, err := s.Chronicle.UnreadResponses(r.Context(), circleID, sess.AccountID); err == nil {
		out["responses_unread"] = n
	}
	// Приехал с кругом и ещё не входил — встреча 1.10 вместо ленты (план 53).
	if a, err := s.Chronicle.CircleArrival(r.Context(), circleID, sess.AccountID); err == nil && a != nil {
		returned := a.Returned
		if returned == nil {
			returned = []string{}
		}
		out["arrival"] = map[string]any{
			"kind": a.Kind, "returned": returned, "returned_count": a.ReturnedCount,
			"total": a.Total, "your_posts": a.YourPosts,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleSeeArrival — «Войти в круг» на 1.10: встреча больше не показывается.
func (s *Server) handleSeeArrival(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.SeeArrival(r.Context(), r.PathValue("circle_id"), sess.AccountID, time.Now()); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	// Пояс устройства (C59): столбики и отсечка — по дням, как их видит человек.
	tz := r.URL.Query().Get("tz")
	if !chronicle.ValidTimeZone(tz) {
		tz = ""
	}
	volume, step, err := s.Chronicle.MediaVolumeChartTZ(r.Context(), circleID, tz)
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
		"used_bytes":  used,
		"post_count":  postCount,
		"volume":      volume,
		"volume_step": step,
	}
	if quota.Valid {
		out["quota_bytes"] = quota.Int64
	}
	// Владелец видит круг не с начала — передали или оживил новым человеком
	// (A12, 6.17а): с какого дня; график и «Освободится» считают весь круг.
	if meta, err := s.Chronicle.FeedMetaForAccount(r.Context(), circleID, sess.AccountID); err == nil && meta.VisibleFrom != nil {
		out["visible_from"] = meta.VisibleFrom.UTC().Format(time.RFC3339)
	}
	if median > 0 {
		out["median_post_bytes"] = median
	}
	// Ждущая просьба (6.19) и отказ (6.20); одобренная не показывается — её
	// видно по квоте.
	if req, err := s.Blobs.LastQuotaRequest(r.Context(), circleID); err != nil {
		writeDomainError(w, err)
		return
	} else if req != nil && req.Status != "approved" {
		last := map[string]any{
			"requested_bytes": req.RequestedBytes,
			"status":          req.Status,
			"created_at":      req.CreatedAt,
		}
		if req.ResolvedAt != nil {
			last["resolved_at"] = *req.ResolvedAt
		}
		out["last_request"] = last
	}
	if cutoff := r.URL.Query().Get("cutoff_date"); cutoff != "" {
		// «Освободится» и «останется» — в том же поясе, как потом отсечёт архивация.
		freed, err := s.Chronicle.FreedBytesBeforeCutoffTZ(r.Context(), circleID, cutoff, tz)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out["freed_at_cutoff_bytes"] = freed
		kept, err := s.Chronicle.PostsKeptAtCutoffTZ(r.Context(), circleID, cutoff, tz)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out["posts_kept_at_cutoff"] = kept
	}
	writeJSON(w, http.StatusOK, out)
}

type startArchiveBody struct {
	CutoffDate        string `json:"cutoff_date"`
	Deadline          string `json:"deadline"`
	ReminderBeforeSec int64  `json:"reminder_before_sec"`
	// TimeZone — пояс устройства владельца (IANA): день отсечки — по нему (C59).
	TimeZone string `json:"time_zone"`
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
	if err := s.Chronicle.SetArchiveTimeZone(r.Context(), circleID, body.TimeZone); err != nil {
		log.Printf("archive %s: time zone: %v", circleID, err)
	}
	s.sendArchiveCycleStartEmails(circleID)
	s.notifyCircle(circleID, sess.AccountID, "event")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type moveCutoffBody struct {
	CutoffDate string `json:"cutoff_date"`
	// TimeZone — пояс устройства владельца (C59).
	TimeZone string `json:"time_zone"`
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
	if err := s.Chronicle.SetArchiveTimeZone(r.Context(), circleID, body.TimeZone); err != nil {
		log.Printf("archive %s: time zone: %v", circleID, err)
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

// archiveLinkTTL — сколько живёт одноразовая ссылка на архив: приложение
// открывает её сразу после выдачи.
const archiveLinkTTL = time.Minute

type archiveLink struct {
	accountID, circleID string
	expires             time.Time
}

// handleArchiveLink выдаёт одноразовую ссылку на архив участника (план 46,
// C38). По ней качает сам браузер — со своим ходом и без копии архива в
// памяти вкладки: fetch с заголовком входа собирал гигабайты в Blob, и
// запрос на сохранение появлялся только через минуты. Ссылка живёт минуту,
// работает один раз и ведёт только к этому архиву этого участника.
func (s *Server) handleArchiveLink(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RequireReader(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	if _, err := s.personalArchiveCutoff(r.Context(), circleID); err != nil {
		writeDomainError(w, err)
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, err)
		return
	}
	token := hex.EncodeToString(buf)
	now := time.Now()
	// Просроченные ссылки убираются при выдаче новых — их немного.
	s.archiveLinks.Range(func(k, v any) bool {
		if now.After(v.(archiveLink).expires) {
			s.archiveLinks.Delete(k)
		}
		return true
	})
	s.archiveLinks.Store(token, archiveLink{
		accountID: sess.AccountID, circleID: circleID, expires: now.Add(archiveLinkTTL),
	})
	writeJSON(w, http.StatusOK, map[string]string{"url": "/api/v1/archive/" + token})
}

// handleArchiveByLink отдаёт архив по одноразовой ссылке — без заголовка входа.
func (s *Server) handleArchiveByLink(w http.ResponseWriter, r *http.Request) {
	v, ok := s.archiveLinks.LoadAndDelete(r.PathValue("token"))
	if !ok || time.Now().After(v.(archiveLink).expires) {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	link := v.(archiveLink)
	s.streamArchive(w, r, link.circleID, link.accountID)
}

func (s *Server) handleArchiveDownload(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	// ?layout= прежних клиентов не мешает: в архиве обе раскладки (6.23).
	s.streamArchive(w, r, r.PathValue("circle_id"), sess.AccountID)
}

// personalArchiveCutoff — до какого дня собирать архив участника: отсечка
// идущего цикла архивации, иначе — всё, что было: «Свой архив можно скачать
// и вне цикла, в любой момент: тогда в нём всё, что человек видел в круге на
// момент скачивания» (wynd.html, «Квота и архив»; A7, решение владельца
// 2026-10-14). Так же — когда круг замер под полный архив с удалением (3.16).
// Вне цикла скачивание ничего в круге не фиксирует: отсечки нет.
func (s *Server) personalArchiveCutoff(ctx context.Context, circleID string) (chronicle.ArchiveCycle, error) {
	all := chronicle.ArchiveCycle{CutoffDate: time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")}
	frozen, err := s.Chronicle.CircleFrozenAt(ctx, circleID)
	if err != nil {
		return chronicle.ArchiveCycle{}, err
	}
	if frozen != nil {
		return all, nil
	}
	cycle, err := s.Chronicle.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return cycle, err
	}
	if !cycle.Active {
		return all, nil
	}
	return cycle, nil
}

func (s *Server) streamArchive(w http.ResponseWriter, r *http.Request, circleID, accountID string) {
	cycle, err := s.personalArchiveCutoff(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	now := time.Now().UTC()
	if !cycle.Deadline.IsZero() && now.After(cycle.Deadline) {
		writeDomainError(w, chronicle.ErrForbidden)
		return
	}
	if _, busy := s.archiveBuilds.LoadOrStore(accountID, struct{}{}); busy {
		writeError(w, &auth.RateLimitError{RetryAfterSec: archiveRetryAfterSec})
		return
	}
	defer s.archiveBuilds.Delete(accountID)
	posts, err := s.Chronicle.ArchiveSnapshot(r.Context(), circleID, accountID, cycle.CutoffDate)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), accountID)
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
	in := archive.BuildInput{
		CircleName: circleName,
		CutoffDate: cycle.CutoffDate,
		Posts:      posts,
		Blobs:      s.Blobs,
		Avatars:    avatars,
		DayTitles:  dayTitles,
		Color:      circleColor,
		Fonts:      s.ArchiveFonts,
	}
	// Архив участника — потоком (решение владельца 2026-10-14, план 46, C38):
	// собирается прямо в ответ и на сервере не лежит, иначе цикл архивации
	// круга без места заполнял бы диск копиями. Первый байт уходит сразу, а
	// длину заранее даёт холостая сборка — браузер показывает ход. Цена:
	// оборванное скачивание начинается заново, докачки нет.
	size, err := archive.Size(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"wynd-archive-"+circleID+".zip\"")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	rec := &deliveryRecorder{ResponseWriter: w}
	rec.WriteHeader(http.StatusOK)
	if err := archive.BuildPersonalArchive(r.Context(), rec, in); err != nil || rec.written != size {
		// Заголовки ушли — сказать об ошибке можно только оборвав ответ:
		// недописанный ZIP не должен выглядеть скачанным целиком.
		log.Printf("archive %s: stream: %v (written %d of %d)", circleID, err, rec.written, size)
		panic(http.ErrAbortHandler)
	}

	// Отсечка замирает, когда участник получил архив целиком — последний байт
	// ушёл, — а не когда архив собран: обрыв на отдаче замораживал её, хотя
	// никто ничего не получил (ARC-8).
	if cycle.Active && rec.deliveredToEnd(size) {
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

// deliveredToEnd: ответ ушёл целиком — 200 и все size байт тела.
func (d *deliveryRecorder) deliveredToEnd(size int64) bool {
	return d.status == http.StatusOK && d.written == size
}

// ArchiveTempDir — каталог, где до 0.30.1 собирались архивы участников. Теперь
// архив идёт потоком; сервер при старте чистит каталог от остатков прежних версий.
func ArchiveTempDir(dataDir string) string {
	if dataDir == "" {
		return filepath.Join(os.TempDir(), "wynd-archive")
	}
	return filepath.Join(dataDir, "tmp")
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
	// Экран «Архив круга», а не адрес API: из почты браузер идёт без ключа входа.
	link := strings.TrimRight(s.PublicURL(), "/") + "/circles/" + circleID + "/archive"
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
		name, err := s.Chronicle.CircleName(ctx, circleID)
		if err != nil {
			log.Printf("archive start mail %s: name: %v", circleID, err)
			return
		}
		emails, err := s.Chronicle.CircleMemberEmails(ctx, circleID)
		if err != nil {
			log.Printf("archive start mail %s: members: %v", circleID, err)
			return
		}
		for _, email := range emails {
			if err := s.Mail.SendArchiveCycleStart(ctx, email, name, cycle.CutoffDate, cycle.Deadline, link); err != nil {
				log.Printf("archive start mail %s to %s: %v", circleID, email, err)
			}
		}
	}()
}
