package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

const moveChunk = 32 << 20

// pullMoveArchive — «беру» и архив кусками с докачкой. Обрыв сети — повтор
// через паузу, пока жива ссылка; отказ прежнего сервера — сбой с его кодом.
func (s *Server) pullMoveArchive(ctx context.Context, m *chronicle.IncomingMove) error {
	if err := os.MkdirAll(MoveDir(s.DataDir), 0o700); err != nil {
		return err
	}
	path := s.incomingPath(m.ID)
	// Only the synced, committed prefix is accepted. A crash may leave an
	// uncommitted tail in the file; never join that tail to the next response.
	if fi, err := os.Stat(path); err == nil && fi.Size() > m.PulledBytes {
		if err := os.Truncate(path, m.PulledBytes); err != nil {
			return err
		}
	}
	name := ""
	if info, err := s.Auth.Instance(ctx); err == nil {
		name = info.Name
	}
	claim := map[string]string{"dest_url": s.PublicURL(), "dest_name": name}
	claimed := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		state, total, err := s.pullMoveChunk(ctx, m, path, have)
		switch {
		case err != nil:
			return err
		case state == "complete":
			_ = s.Chronicle.SetIncomingPulled(ctx, m.ID, total)
			return nil
		case state == "progress":
			fi, err := os.Stat(path)
			if err != nil {
				return err
			}
			if err := s.Chronicle.SetIncomingPulled(ctx, m.ID, fi.Size()); err != nil {
				return err
			}
		case state == "archive_failed":
			// Прежний сервер не собрал архив — «беру» снова, сборка заново.
			claimed = false
			s.removeIncomingFile(m.ID)
			m.ArchiveETag = ""
			_ = s.Chronicle.SetIncomingArchiveETag(ctx, m.ID, "")
			s.moveWait(ctx)
		case state == "archive_changed":
			return &moveFailure{"archive_changed"}
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
func (s *Server) pullMoveChunk(ctx context.Context, m *chronicle.IncomingMove, path string, have int64) (state string, total int64, err error) {
	if have < 0 || have > s.moveMaxBytes() || have > int64(^uint64(0)>>1)-moveChunk {
		return "", 0, &moveFailure{"no_room"}
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	headers := map[string]string{"Range": fmt.Sprintf("bytes=%d-%d", have, have+moveChunk-1)}
	if have > 0 && m.ArchiveETag != "" {
		headers["If-Range"] = m.ArchiveETag
	}
	resp, err := s.moveCall(cctx, m.Link, http.MethodGet, "/api/v1/move/archive", nil, headers)
	if err != nil {
		log.Printf("move pull: %v", err)
		return "", 0, nil
	}
	defer resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		if m.ArchiveETag != "" && etag != m.ArchiveETag {
			return "", 0, &moveFailure{"archive_changed"}
		}
	}
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		return "", 0, &moveFailure{"archive_changed"}
	case http.StatusRequestedRangeNotSatisfiable:
		// Файл уже весь здесь: прежний сервер говорит его длину.
		var size int64
		if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes */%d", &size); err == nil && size == have {
			if err := s.pinMoveTotal(m.ID, size); err != nil {
				return "", 0, err
			}
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
	if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err != nil || start < 0 || start != have || end < start || end >= total || end-start+1 > moveChunk {
		return "", 0, &moveFailure{"archive_changed"}
	}
	if err := s.pinMoveTotal(m.ID, total); err != nil {
		return "", 0, err
	}
	if err := s.reserveMoveDisk(ctx, m.ID, total-have); err != nil {
		return "", 0, err
	}
	if m.ArchiveETag == "" && etag != "" && !strings.HasPrefix(etag, "W/") {
		if err := s.Chronicle.SetIncomingArchiveETag(ctx, m.ID, etag); err != nil {
			return "", 0, err
		}
		m.ArchiveETag = etag
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
		var fileError *os.PathError
		if errors.As(err, &fileError) {
			_ = f.Truncate(have)
			return "", 0, err
		}
		// The response belongs to a frozen representation, so its received prefix
		// remains usable even when the connection ends halfway through a chunk.
		if n > 0 {
			return "progress", total, f.Sync()
		}
		return "", 0, nil
	}
	if have+n >= total {
		return "complete", total, f.Sync()
	}
	return "progress", total, f.Sync()
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
