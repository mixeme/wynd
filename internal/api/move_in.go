package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Перенос круга по ссылке (план 53), новый сервер. Устройство отдаёт ссылку
// своему серверу; он спрашивает карточку у прежнего (из браузера туда нельзя —
// другой адрес) и после «Поднять круг здесь» забирает круг сам: телефон не
// нужен (7.20), обрыв не страшен — докачка с места обрыва, пока жива ссылка.

// moveChunk — сколько байт архива за один запрос: обрыв теряет не больше.
const moveChunk = 32 << 20

// maxMoveCard — предел ответа прежнего сервера с карточкой.
const maxMoveCard = 64 << 10

var moveTokenRe = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

// parseMoveLink — адрес прежнего сервера и ключ из ссылки «…/move/<ключ>».
// Сервер ходит только на этот адрес и только по своим путям /api/v1/move….
func parseMoveLink(link string) (base, token string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", "", false
	}
	prefix, token, found := strings.Cut(u.Path, "/move/")
	if !found || !moveTokenRe.MatchString(token) {
		return "", "", false
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(prefix, "/"), token, true
}

func (s *Server) moveClient() *http.Client {
	if s.MoveClient != nil {
		return s.MoveClient
	}
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (s *Server) moveRetry() time.Duration {
	if s.MoveRetry > 0 {
		return s.MoveRetry
	}
	return time.Minute
}

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
		Name    string `json:"name"`
		Color   string `json:"color"`
		Posts   int    `json:"posts"`
		Members int    `json:"members"`
		Bytes   int64  `json:"bytes"`
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

func incomingJSON(m *chronicle.IncomingMove, now time.Time) map[string]any {
	out := map[string]any{
		"id":           m.ID,
		"status":       m.Status,
		"source_url":   m.SourceURL,
		"source_name":  m.SourceName,
		"expires_at":   m.ExpiresAt,
		"link_alive":   now.Before(m.ExpiresAt),
		"send_invites": m.SendInvites,
		"pulled_bytes": m.PulledBytes,
		"circle": map[string]any{
			"name": m.CircleName, "color": m.CircleColor,
			"posts": m.Posts, "members": m.Members, "bytes": m.Bytes,
		},
	}
	if m.Link != "" {
		out["link"] = m.Link
	}
	if m.FrozenAt != nil {
		out["frozen_at"] = *m.FrozenAt
	}
	if m.Error != "" {
		out["error"] = m.Error
	}
	if m.Status == chronicle.IncomingDone {
		out["result"] = map[string]any{
			"circle_id": m.CircleID, "posts": m.ResultPosts, "names": m.ResultNames, "bytes": m.ResultBytes + m.ResultTextBytes,
			"verified": m.Verified, "invited": m.Invited, "not_joined": m.NotJoined,
		}
	}
	return out
}

// withRoom добавляет к ответу место для круга и просьбу о нём (7.21, 7.22,
// 7.23) — как у оживления из архива.
func (s *Server) withRoom(ctx context.Context, out map[string]any, accountID string, need int64) error {
	room, approved, last, err := s.reviveRoom(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.roomLimit(ctx, out, room, approved != nil); err != nil {
		return err
	}
	out["room_bytes"] = room
	out["fits"] = need <= room
	if last != nil {
		req := map[string]any{
			"status": last.Status, "requested_bytes": last.RequestedBytes,
			"circle_name": last.CircleName, "created_at": last.CreatedAt,
		}
		if last.ResolvedAt != nil {
			req["resolved_at"] = *last.ResolvedAt
		}
		out["request"] = req
	}
	return nil
}

// handlePutIncomingMove — вставили ссылку на 7.20: карточка круга с прежнего
// сервера, место здесь. Ссылка запоминается (7.22).
func (s *Server) handlePutIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		Link string `json:"link"`
	}](w, r)
	if !ok {
		return
	}
	link := strings.TrimSpace(body.Link)
	card, code := s.fetchMoveCard(r.Context(), link)
	if code != "" {
		status := http.StatusBadRequest
		if code == "source_unreachable" {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	if card.Circle == nil {
		// Перенос отменён или закончен: круга по этой ссылке больше нет.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "move_" + card.Status})
		return
	}
	// Адрес прежнего сервера — из самой ссылки: по нему сервер и ходит.
	base, _, _ := parseMoveLink(link)
	in := chronicle.IncomingCard{
		SourceURL: base, SourceName: card.ServerName,
		CircleName: card.Circle.Name, CircleColor: card.Circle.Color,
		Posts: card.Circle.Posts, Members: card.Circle.Members, Bytes: card.Circle.Bytes,
		ExpiresAt: card.ExpiresAt,
	}
	if t, err := time.Parse(time.RFC3339, card.FrozenAt); err == nil {
		in.FrozenAt = &t
	}
	m, err := s.Chronicle.PutIncomingDraft(r.Context(), sess.AccountID, link, in, time.Now())
	if errors.Is(err, chronicle.ErrMoveBusy) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "move_busy"})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := incomingJSON(m, time.Now())
	if card.Status == "expired" {
		out["link_alive"] = false
	}
	if err := s.withRoom(r.Context(), out, sess.AccountID, m.Bytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleIncomingMove — последний входящий перенос этой учётки (7.20 при
// открытии, ход, итог 7.25 / 7.26) и место для него.
func (s *Server) handleIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	m, err := s.Chronicle.LatestIncoming(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if m == nil {
		out := map[string]any{"move": nil}
		if err := s.withRoom(r.Context(), out, sess.AccountID, 0); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	out := incomingJSON(m, time.Now())
	if err := s.withRoom(r.Context(), out, sess.AccountID, m.Bytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"move": out})
}

// handleStartIncomingMove — «Поднять круг здесь» (7.20): дальше сервер
// работает сам.
func (s *Server) handleStartIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		SendInvites bool `json:"send_invites"`
	}](w, r)
	if !ok {
		return
	}
	m, err := s.Chronicle.GetIncoming(r.Context(), r.PathValue("id"))
	if err != nil || m.AccountID != sess.AccountID {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	if !time.Now().Before(m.ExpiresAt) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "move_expired"})
		return
	}
	room, _, _, err := s.reviveRoom(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	if m.Bytes > room {
		writeJSON(w, http.StatusInsufficientStorage, map[string]string{"error": "no_room"})
		return
	}
	started, err := s.Chronicle.StartIncoming(r.Context(), m.ID, body.SendInvites, time.Now())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if started {
		s.runIncomingMove(m.ID)
	}
	m, err = s.Chronicle.GetIncoming(r.Context(), m.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incomingJSON(m, time.Now()))
}

// ResumeIncomingMoves — при запуске: переносы, прерванные остановкой сервера,
// продолжаются с места обрыва.
func (s *Server) ResumeIncomingMoves(ctx context.Context) {
	ids, err := s.Chronicle.ActiveIncoming(ctx)
	if err != nil {
		log.Printf("incoming moves resume: %v", err)
		return
	}
	for _, id := range ids {
		s.runIncomingMove(id)
	}
}

func (s *Server) incomingPath(id string) string {
	return filepath.Join(MoveDir(s.DataDir), "in-"+id+".wynd")
}

// moveFailure — сбой, после которого повторять незачем (код — на экран).
type moveFailure struct{ code string }

func (e *moveFailure) Error() string { return "move: " + e.code }

// runIncomingMove — работа в фоне: забрать архив, поднять круг, сказать
// прежнему серверу «принято и сверено».
func (s *Server) runIncomingMove(id string) {
	if _, busy := s.incomingRuns.LoadOrStore(id, struct{}{}); busy {
		return
	}
	go func() {
		defer s.incomingRuns.Delete(id)
		ctx := context.Background()
		err := s.incomingMove(ctx, id)
		if err == nil {
			_ = os.Remove(s.incomingPath(id))
			return
		}
		code := "failed"
		var mf *moveFailure
		var re *reviveError
		switch {
		case errors.As(err, &mf):
			code = mf.code
		case errors.As(err, &re):
			code = re.code
		case errors.Is(err, blob.ErrInstanceQuotaExceeded):
			code = "no_room"
		}
		log.Printf("incoming move %s: %v", id, err)
		_ = os.Remove(s.incomingPath(id))
		if err := s.Chronicle.SetIncomingStatus(context.Background(), id, chronicle.IncomingFailed, code); err != nil {
			log.Printf("incoming move %s: fail: %v", id, err)
		}
	}()
}

func (s *Server) incomingMove(ctx context.Context, id string) error {
	m, err := s.Chronicle.GetIncoming(ctx, id)
	if err != nil {
		return err
	}
	if m.Status == chronicle.IncomingPulling {
		if err := s.pullMoveArchive(ctx, m); err != nil {
			return err
		}
		if err := s.Chronicle.SetIncomingStatus(ctx, id, chronicle.IncomingReviving, ""); err != nil {
			return err
		}
	}
	return s.reviveIncoming(ctx, m)
}

// pullMoveArchive — «беру» и архив кусками с докачкой. Обрыв сети — повтор
// через паузу, пока жива ссылка; отказ прежнего сервера — сбой с его кодом.
func (s *Server) pullMoveArchive(ctx context.Context, m *chronicle.IncomingMove) error {
	if err := os.MkdirAll(MoveDir(s.DataDir), 0o700); err != nil {
		return err
	}
	path := s.incomingPath(m.ID)
	name := ""
	if info, err := s.Auth.Instance(ctx); err == nil {
		name = info.Name
	}
	claim := map[string]string{"dest_url": s.PublicURL(), "dest_name": name}
	claimed := false
	for {
		if !time.Now().Before(m.ExpiresAt) {
			return &moveFailure{"move_expired"}
		}
		if !claimed {
			done, err := s.claimMove(ctx, m.Link, claim)
			if err != nil {
				return err
			}
			if !done {
				s.moveWait(ctx)
				continue
			}
			claimed = true
		}
		have := int64(0)
		if fi, err := os.Stat(path); err == nil {
			have = fi.Size()
		}
		state, total, err := s.pullMoveChunk(ctx, m.Link, path, have)
		switch {
		case err != nil:
			return err
		case state == "complete":
			_ = s.Chronicle.SetIncomingPulled(ctx, m.ID, total)
			return nil
		case state == "progress":
			if err := s.Chronicle.SetIncomingPulled(ctx, m.ID, have+min(moveChunk, max(total-have, 0))); err != nil {
				return err
			}
		case state == "archive_failed":
			// Прежний сервер не собрал архив — «беру» снова, сборка заново.
			claimed = false
			_ = os.Remove(path)
			s.moveWait(ctx)
		case state == "archive_building":
			// Прежний сервер собирает архив: спрашивать чаще, чем после обрыва.
			s.movePollWait(ctx)
		default: // обрыв
			s.moveWait(ctx)
		}
	}
}

// movePollWait — пауза, пока прежний сервер собирает архив: 5 секунд.
func (s *Server) movePollWait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(min(5*time.Second, s.moveRetry())):
	}
}

func (s *Server) moveWait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(s.moveRetry()):
	}
}

// claimMove — «беру». false без ошибки — обрыв, повторить.
func (s *Server) claimMove(ctx context.Context, link string, claim map[string]string) (bool, error) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := s.moveCall(cctx, link, http.MethodPost, "/api/v1/move/claim", claim, nil)
	if err != nil {
		log.Printf("move claim: %v", err)
		return false, nil
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusGone, http.StatusConflict, http.StatusNotFound, http.StatusUnauthorized:
		code := moveErrorCode(resp)
		if code == "" {
			code = "move_not_found"
		}
		return false, &moveFailure{code}
	}
	return false, nil
}

// pullMoveChunk докачивает один кусок: state — complete (файл весь),
// progress, archive_building, archive_failed или пусто (обрыв).
func (s *Server) pullMoveChunk(ctx context.Context, link, path string, have int64) (state string, total int64, err error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	resp, err := s.moveCall(cctx, link, http.MethodGet, "/api/v1/move/archive", nil,
		map[string]string{"Range": fmt.Sprintf("bytes=%d-%d", have, have+moveChunk-1)})
	if err != nil {
		log.Printf("move pull: %v", err)
		return "", 0, nil
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		// Файл уже весь здесь: прежний сервер говорит его длину.
		var size int64
		if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes */%d", &size); err == nil && size == have {
			return "complete", have, nil
		}
		return "", 0, &moveFailure{"archive_changed"}
	case http.StatusConflict:
		return moveErrorCode(resp), 0, nil
	case http.StatusGone, http.StatusNotFound, http.StatusUnauthorized:
		code := moveErrorCode(resp)
		if code == "" {
			code = "move_not_found"
		}
		return "", 0, &moveFailure{code}
	default:
		return "", 0, nil
	}
	var start, end int64
	if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err != nil || start != have {
		return "", 0, &moveFailure{"archive_changed"}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	if _, err := f.Seek(have, io.SeekStart); err != nil {
		return "", 0, err
	}
	want := end - start + 1
	n, err := io.Copy(f, io.LimitReader(resp.Body, want))
	if err != nil || n != want {
		// Обрыв посреди куска: файл — к прежней длине, кусок — заново.
		_ = f.Truncate(have)
		return "", 0, nil
	}
	if have+n >= total {
		return "complete", total, f.Sync()
	}
	return "progress", total, nil
}

// reviveIncoming поднимает круг из забранного файла: каждый файл — в
// хранилище от имени владельца со сверкой суммы, строки — в ReviveCircle;
// затем «принято и сверено» прежнему серверу.
func (s *Server) reviveIncoming(ctx context.Context, m *chronicle.IncomingMove) error {
	if m.CircleID == "" {
		out, err := s.reviveMoveFile(ctx, m)
		if err != nil {
			return err
		}
		if err := s.Chronicle.SetIncomingRevived(ctx, m.ID, out.CircleID, out.Posts, out.Names, out.MediaBytes, out.Verified, out.Invited, out.NotJoined); err != nil {
			return err
		}
		m.ResultPosts, m.ResultNames, m.ResultBytes = out.Posts, out.Names, out.MediaBytes
	}
	s.acceptMove(ctx, m.Link, m.ResultPosts, m.ResultNames, m.ResultBytes)
	return s.Chronicle.FinishIncoming(ctx, m.ID)
}

func (s *Server) reviveMoveFile(ctx context.Context, m *chronicle.IncomingMove) (reviveOutcome, error) {
	zr, err := archive.OpenFullArchive(s.incomingPath(m.ID))
	if err != nil {
		return reviveOutcome{}, &moveFailure{"archive_invalid"}
	}
	defer zr.Close()
	raw, err := zr.Tables()
	if err != nil {
		return reviveOutcome{}, &moveFailure{"archive_invalid"}
	}
	tables := map[string]chronicle.FullArchiveTable{}
	for name, t := range raw {
		tables[name] = chronicle.FullArchiveTable{Name: name, Columns: t.Columns, Rows: t.Rows}
	}
	blobMap, err := s.importMoveBlobs(ctx, m.AccountID, zr, tables["blobs"])
	if err != nil {
		return reviveOutcome{}, err
	}
	host := m.SourceURL
	if u, err := url.Parse(m.SourceURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return s.reviveFromTables(ctx, reviveRequest{
		AccountID:     m.AccountID,
		FormatVersion: zr.Manifest.FormatVersion,
		SchemaVersion: zr.Manifest.SchemaVersion,
		Posts:         zr.Manifest.Posts,
		Tables:        tables,
		BlobMap:       blobMap,
		SendInvites:   m.SendInvites,
		MovedFrom:     host,
	})
}

// importMoveBlobs кладёт файлы круга в хранилище. Файл, который у этого
// человека уже лежит с той же суммой, второй раз не кладётся.
func (s *Server) importMoveBlobs(ctx context.Context, accountID string, zr *archive.FullReader, t chronicle.FullArchiveTable) (map[string]string, error) {
	col := func(name string) int {
		for i, c := range t.Columns {
			if c == name {
				return i
			}
		}
		return -1
	}
	id, sum, size, mime, fname := col("id"), col("sha256"), col("size_bytes"), col("mime_type"), col("original_filename")
	if len(t.Rows) > 0 && (id < 0 || sum < 0 || size < 0) {
		return nil, &moveFailure{"archive_invalid"}
	}
	str := func(row []any, i int) string {
		if i < 0 {
			return ""
		}
		v, _ := row[i].(string)
		return v
	}
	var sums []string
	for _, row := range t.Rows {
		sums = append(sums, str(row, sum))
	}
	have, err := s.Blobs.OwnBlobsBySHA(ctx, accountID, sums)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, row := range t.Rows {
		old := str(row, id)
		if nw, ok := have[str(row, sum)]; ok {
			out[old] = nw
			continue
		}
		rc, err := zr.OpenMedia(old)
		if err != nil {
			return nil, &moveFailure{"archive_invalid"}
		}
		b, err := s.Blobs.ImportBlob(ctx, blob.ImportInput{
			AccountID: accountID, SHA256: str(row, sum), SizeBytes: jsonInt64(row[size]),
			MimeType: str(row, mime), OriginalFilename: str(row, fname), Now: time.Now(),
		}, rc)
		rc.Close()
		if errors.Is(err, blob.ErrInvalid) {
			return nil, &moveFailure{"archive_invalid"}
		}
		if err != nil {
			return nil, err
		}
		out[old] = b.ID
		have[b.SHA256] = b.ID
	}
	return out, nil
}

// acceptMove — «принято и сверено» прежнему серверу (6.35). Круг здесь уже
// стоит; обрыв — несколько повторов. Не дошло — на прежнем сервере перенос
// так и ждёт, пока не выйдет срок ссылки, и владелец там решает сам.
func (s *Server) acceptMove(ctx context.Context, link string, posts, names int, bytes int64) {
	body := map[string]any{"posts": posts, "names": names, "bytes": bytes}
	for attempt := range 5 {
		if attempt > 0 {
			s.moveWait(ctx)
		}
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, err := s.moveCall(cctx, link, http.MethodPost, "/api/v1/move/accepted", body, nil)
		if err != nil {
			cancel()
			log.Printf("move accepted: %v", err)
			continue
		}
		status, code := resp.StatusCode, ""
		if status != http.StatusOK {
			code = moveErrorCode(resp)
		}
		resp.Body.Close()
		cancel()
		if status == http.StatusOK {
			return
		}
		log.Printf("move accepted: %d %s", status, code)
		if status < 500 {
			return
		}
	}
}
