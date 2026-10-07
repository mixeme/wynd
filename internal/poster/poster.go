// Package poster делает кадр ролика на сервере, когда его не смог сделать
// отправитель.
//
// Временная мера (план 49, решение 2026-10-07): Firefox на Android не отдаёт
// картинку с ролика из файла, и видео из галереи приходит без кадра. Снять её
// до появления нативной обёртки может только сервер. Это единственное место,
// где сервер разбирает медиа; правило «сервер не трогает медиа» остаётся
// для всего остального. Без ffmpeg на машине пакет ничего не делает.
package poster

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

const (
	// Один кадр 4K снимается за секунду-другую; дольше — файл битый или
	// машина занята, и запись ждать не должна.
	frameTimeout = 20 * time.Second
	// Больше JPEG со стороной 1024 не бывает; потолок — от мусора на выходе.
	maxFrameBytes = 4 << 20
	// Как у кадра, который шлёт клиент, и у обложки звука.
	frameEdge = "1024"
)

// frameFunc снимает JPEG кадра с файла ролика; подменяется в тестах.
type frameFunc func(ctx context.Context, path string) ([]byte, error)

// Maker снимает кадры по одному: два ffmpeg разом на дешёвом сервере ни к чему.
type Maker struct {
	blobs *blob.Store
	frame frameFunc

	// mu держится на один кадр, не на весь проход: новая запись не ждёт,
	// пока сервер пройдёт все старые ролики.
	mu sync.Mutex
	// Ролики, с которых кадр не вышел: до перезапуска их не трогаем, иначе
	// каждый проход по старым записям снова упирался бы в тот же файл.
	failed map[string]bool
}

// New ищет ffmpeg: путь в WYND_FFMPEG, иначе в PATH. WYND_FFMPEG=off
// выключает. Не найден — Maker есть, но Enabled() ложно.
func New(blobs *blob.Store) *Maker {
	m := &Maker{blobs: blobs, failed: map[string]bool{}}
	bin := strings.TrimSpace(os.Getenv("WYND_FFMPEG"))
	if strings.EqualFold(bin, "off") {
		return m
	}
	if bin == "" {
		bin = "ffmpeg"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return m
	}
	m.frame = func(ctx context.Context, file string) ([]byte, error) {
		return ffmpegFrame(ctx, path, file)
	}
	return m
}

// NewFunc — Maker, которому кадр снимает переданная функция (тесты).
func NewFunc(blobs *blob.Store, frame func(ctx context.Context, path string) ([]byte, error)) *Maker {
	return &Maker{blobs: blobs, frame: frame, failed: map[string]bool{}}
}

// Enabled — есть ли чем снимать кадр.
func (m *Maker) Enabled() bool {
	return m != nil && m.frame != nil
}

type pending struct {
	mediaID, blobID, accountID, relPath string
}

// FillPost снимает кадр роликам записи, у которых его нет. Ошибки не
// возвращает: запись уже создана, без кадра плитка рисует значок.
func (m *Maker) FillPost(ctx context.Context, postID string) {
	if !m.Enabled() || postID == "" {
		return
	}
	m.fill(ctx, `AND pm.post_id = ?`, postID)
}

// Backfill — то же для всех записей: ролики, лежавшие до этой версии.
func (m *Maker) Backfill(ctx context.Context) int {
	if !m.Enabled() {
		return 0
	}
	return m.fill(ctx, ``)
}

func (m *Maker) fill(ctx context.Context, where string, args ...any) int {
	rows, err := m.blobs.DB().QueryContext(ctx, `
		SELECT pm.id, pm.blob_id, b.account_id, b.storage_path
		FROM post_media pm
		JOIN posts p ON p.id = pm.post_id AND p.deleted = 0
		JOIN blobs b ON b.id = pm.blob_id AND b.status = 'complete'
		WHERE pm.kind = 'video' AND COALESCE(pm.video_poster_blob_id, '') = '' `+where+`
		ORDER BY p.created_at DESC
	`, args...)
	if err != nil {
		log.Printf("poster: list: %v", err)
		return 0
	}
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.mediaID, &p.blobID, &p.accountID, &p.relPath); err != nil {
			rows.Close()
			log.Printf("poster: scan: %v", err)
			return 0
		}
		todo = append(todo, p)
	}
	rows.Close()

	made := 0
	for _, p := range todo {
		if ctx.Err() != nil {
			break
		}
		done, err := m.make(ctx, p)
		if err != nil {
			log.Printf("poster: video %s: %v", p.blobID, err)
		}
		if done {
			made++
		}
	}
	return made
}

func (m *Maker) make(ctx context.Context, p pending) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failed[p.blobID] {
		return false, nil
	}
	frameCtx, cancel := context.WithTimeout(ctx, frameTimeout)
	defer cancel()
	path := filepath.Join(m.blobs.Dir(), filepath.FromSlash(p.relPath))
	jpeg, err := m.frame(frameCtx, path)
	if err != nil {
		m.failed[p.blobID] = true
		return false, err
	}
	// Кадр — блоб автора ролика: так же, как если бы его прислал он сам.
	b, err := m.blobs.PutGenerated(ctx, p.accountID, "image/jpeg", "poster.jpg", jpeg)
	if err != nil {
		return false, err
	}
	res, err := m.blobs.DB().ExecContext(ctx, `
		UPDATE post_media SET video_poster_blob_id = ?
		WHERE id = ? AND COALESCE(video_poster_blob_id, '') = ''
	`, b.ID, p.mediaID)
	if err == nil {
		if n, _ := res.RowsAffected(); n == 1 {
			return true, nil
		}
	}
	// Запись тем временем поправили или у ролика уже есть кадр: наш лишний.
	if relErr := m.blobs.ReleaseBlobs(ctx, []string{b.ID}); relErr != nil {
		log.Printf("poster: release %s: %v", b.ID, relErr)
	}
	return false, err
}

// ffmpegFrame — один кадр в JPEG на stdout. Сначала с первой секунды: начало
// ролика часто чёрное. Ролик короче — с самого начала.
func ffmpegFrame(ctx context.Context, bin, file string) ([]byte, error) {
	var lastErr error
	for _, at := range []string{"1", "0"} {
		out, err := runFFmpeg(ctx, bin, file, at)
		if err == nil && isJPEG(out) {
			return out, nil
		}
		if err == nil {
			err = errors.New("ffmpeg: no frame")
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func runFFmpeg(ctx context.Context, bin, file, at string) ([]byte, error) {
	// Файл чужой, поэтому разбор зажат: только контейнеры видео (плейлист
	// или «склейка» заставили бы ffmpeg читать другие файлы и ходить в
	// сеть), только протокол file на входе, один поток, без stdin.
	cmd := exec.CommandContext(ctx, bin,
		"-nostdin", "-v", "error", "-threads", "1",
		"-protocol_whitelist", "file",
		"-format_whitelist", "mov,mp4,m4a,3gp,3g2,mj2,matroska,webm",
		"-ss", at, "-i", file,
		"-map", "0:v:0", "-frames:v", "1",
		"-vf", "scale=w='min("+frameEdge+",iw)':h='min("+frameEdge+",ih)':force_original_aspect_ratio=decrease",
		"-q:v", "5", "-f", "mjpeg", "pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, left: 2048}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, maxFrameBytes+1))
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if len(out) > maxFrameBytes {
		return nil, errors.New("ffmpeg: frame too large")
	}
	if waitErr != nil {
		return nil, errors.New("ffmpeg: " + strings.TrimSpace(stderr.String()) + ": " + waitErr.Error())
	}
	return out, nil
}

func isJPEG(b []byte) bool {
	return len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
}

type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.left > 0 {
		n := len(p)
		if n > l.left {
			n = l.left
		}
		_, _ = l.w.Write(p[:n])
		l.left -= n
	}
	return len(p), nil
}
