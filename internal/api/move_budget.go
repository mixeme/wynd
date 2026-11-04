package api

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
)

const defaultMoveMaxBytes int64 = 32 << 30
const maxIncomingMoves = 2
const moveDiskHeadroom int64 = 64 << 20

type moveContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r moveContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (s *Server) moveMaxBytes() int64 {
	if s.MoveMaxArchiveBytes > 0 {
		return s.MoveMaxArchiveBytes
	}
	return defaultMoveMaxBytes
}

// Reserve remaining disk work, not bytes already reflected in filesystem usage.
func (s *Server) reserveMoveDisk(ctx context.Context, id string, need int64) error {
	if need < 0 {
		return &moveFailure{"archive_invalid"}
	}
	s.moveBudgetMu.Lock()
	defer s.moveBudgetMu.Unlock()
	free, _, err := check.DiskUsage(s.DataDir)
	if err != nil {
		return err
	}
	available := int64(min(free, uint64(math.MaxInt64))) - moveDiskHeadroom
	if s.MoveDiskFree != nil {
		available, err = s.MoveDiskFree()
		if err != nil {
			return err
		}
	}
	if s.Blobs != nil {
		room, err := s.Blobs.RoomBytes(ctx)
		if err != nil {
			return err
		}
		available = min(available, room)
	}
	for other, reserved := range s.moveReservations {
		if other != id {
			available -= reserved
		}
	}
	if need > available {
		return &moveFailure{"no_room"}
	}
	if s.moveReservations == nil {
		s.moveReservations = map[string]int64{}
	}
	s.moveReservations[id] = need
	return nil
}

func (s *Server) pinMoveTotal(id string, total int64) error {
	if total <= 0 || total > s.moveMaxBytes() {
		return &moveFailure{"no_room"}
	}
	path := s.incomingPath(id) + ".range"
	if f, err := os.Open(path); err == nil {
		var buf [32]byte
		n, readErr := f.Read(buf[:])
		f.Close()
		previous, parseErr := strconv.ParseInt(string(buf[:n]), 10, 64)
		if readErr != nil || parseErr != nil || previous != total {
			return &moveFailure{"archive_changed"}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// The worker is unique per ID; a small durable record pins length across restarts.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, total)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

func (s *Server) removeIncomingFile(id string) {
	_ = os.Remove(s.incomingPath(id))
	_ = os.Remove(s.incomingPath(id) + ".range")
}

func (s *Server) moveTimeout() time.Duration {
	if s.MoveTimeout > 0 {
		return s.MoveTimeout
	}
	return 24 * time.Hour
}
