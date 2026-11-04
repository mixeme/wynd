package api

import (
	"context"
	"errors"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"log"
	"path/filepath"
	"syscall"
	"time"
)

func (s *Server) moveRetry() time.Duration {
	if s.MoveRetry > 0 {
		return s.MoveRetry
	}
	return time.Minute
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
	s.moveBudgetMu.Lock()
	if _, busy := s.incomingRuns.Load(id); busy {
		s.moveBudgetMu.Unlock()
		return
	}
	if s.moveActive >= maxIncomingMoves {
		s.moveBudgetMu.Unlock()
		_ = s.Chronicle.SetIncomingStatus(context.Background(), id, chronicle.IncomingFailed, "move_busy")
		return
	}
	if _, busy := s.incomingRuns.LoadOrStore(id, struct{}{}); busy {
		s.moveBudgetMu.Unlock()
		return
	}
	s.moveActive++
	s.moveBudgetMu.Unlock()
	go func() {
		defer func() {
			s.moveBudgetMu.Lock()
			delete(s.moveReservations, id)
			s.moveActive--
			s.incomingRuns.Delete(id)
			s.moveBudgetMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), s.moveTimeout())
		defer cancel()
		err := s.incomingMoveSafely(ctx, id)
		if err == nil {
			s.removeIncomingFile(id)
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
		case errors.Is(err, syscall.ENOSPC):
			code = "no_room"
		}
		log.Printf("incoming move %s: %v", id, err)
		if code != "move_expired" {
			s.removeIncomingFile(id)
			_ = s.Chronicle.SetIncomingArchiveETag(context.Background(), id, "")
			_ = s.Chronicle.SetIncomingPulled(context.Background(), id, 0)
		}
		if err := s.Chronicle.SetIncomingStatus(context.Background(), id, chronicle.IncomingFailed, code); err != nil {
			log.Printf("incoming move %s: fail: %v", id, err)
		}
	}()
}

func (s *Server) incomingMoveSafely(ctx context.Context, id string) (err error) {
	defer func() {
		if recover() != nil {
			// Do not log untrusted panic values or leave the process without a worker.
			err = &moveFailure{"archive_invalid"}
		}
	}()
	return s.incomingMove(ctx, id)
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
