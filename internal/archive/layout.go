package archive

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
)

// LayoutPart is either fixed ZIP metadata or an interval of an immutable blob.
// Paths are resolved through the store so backups and storage migrations keep working.
type LayoutPart struct {
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	Data   []byte `json:"data,omitempty"`
	BlobID string `json:"blob_id,omitempty"`
}

// FullLayout persists the exact response across restarts without copying media.
type FullLayout struct {
	Version int          `json:"version"`
	Size    int64        `json:"size"`
	Parts   []LayoutPart `json:"parts"`
}

type layoutBlob struct {
	size int64
	sha  string
}
type layoutWriter struct {
	plan     FullLayout
	blobs    map[string]layoutBlob
	external bool
}

func (w *layoutWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if int64(len(p)) > math.MaxInt64-w.plan.Size {
		return 0, errors.New("archive: ZIP too large")
	}
	if w.external {
		w.plan.Parts[len(w.plan.Parts)-1].Size += int64(len(p))
	} else {
		if len(w.plan.Parts) == 0 || w.plan.Parts[len(w.plan.Parts)-1].BlobID != "" {
			w.plan.Parts = append(w.plan.Parts, LayoutPart{Offset: w.plan.Size})
		}
		part := &w.plan.Parts[len(w.plan.Parts)-1]
		part.Data = append(part.Data, p...)
		part.Size += int64(len(p))
	}
	w.plan.Size += int64(len(p))
	return len(p), nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (w *layoutWriter) addBlob(ctx context.Context, zw *zip.Writer, store *blob.Store, id, name string) error {
	expected, ok := w.blobs[id]
	if !ok {
		return fmt.Errorf("archive: blob %s is not in snapshot", id)
	}
	info, err := store.OpenBlob(ctx, id)
	if err != nil {
		return err
	}
	f, err := os.Open(info.Path)
	if err != nil {
		return err
	}
	h := sha256.New()
	crc := crc32.NewIEEE()
	n, err := io.Copy(io.MultiWriter(h, crc), contextReader{ctx, f})
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n != expected.size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), expected.sha) {
		return fmt.Errorf("archive: blob %s changed since snapshot", id)
	}
	entry, err := zw.CreateRaw(&zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc.Sum32(), CompressedSize64: uint64(n), UncompressedSize64: uint64(n)})
	if err != nil {
		return err
	}
	// Flush headers before recording the external interval, and its buffered
	// tail before returning to metadata. CreateRaw already has CRC and sizes.
	if err := zw.Flush(); err != nil {
		return err
	}
	w.plan.Parts = append(w.plan.Parts, LayoutPart{Offset: w.plan.Size, BlobID: id})
	w.external = true
	_, err = io.CopyN(entry, zeros{}, n)
	if err == nil {
		err = zw.Flush()
	}
	w.external = false
	if err != nil {
		return err
	}
	if w.plan.Parts[len(w.plan.Parts)-1].Size != n {
		return errors.New("archive: inconsistent ZIP interval")
	}
	return nil
}

// BuildFullLayout freezes compressed text, HTML, fonts and ZIP headers. Each
// media file is read once for checksum verification, never copied to the index.
func BuildFullLayout(ctx context.Context, in FullInput) (*FullLayout, error) {
	w := &layoutWriter{plan: FullLayout{Version: 1}, blobs: map[string]layoutBlob{}}
	if in.Data == nil {
		return nil, errors.New("archive: missing snapshot")
	}
	for _, table := range in.Data.Tables {
		if table.Name != "blobs" {
			continue
		}
		idCol, sizeCol, shaCol := -1, -1, -1
		for i, col := range table.Columns {
			switch col {
			case "id":
				idCol = i
			case "size_bytes":
				sizeCol = i
			case "sha256":
				shaCol = i
			}
		}
		if idCol < 0 || sizeCol < 0 || shaCol < 0 {
			return nil, errors.New("archive: invalid blob metadata")
		}
		for _, row := range table.Rows {
			id, _ := row[idCol].(string)
			sha, _ := row[shaCol].(string)
			var size int64
			switch value := row[sizeCol].(type) {
			case int64:
				size = value
			case float64:
				size = int64(value)
			default:
				return nil, errors.New("archive: invalid blob size")
			}
			w.blobs[id] = layoutBlob{size, sha}
		}
	}
	in.Readable.layout = w
	if err := BuildFullArchive(ctx, w, in); err != nil {
		return nil, err
	}
	return &w.plan, nil
}

// LayoutReader implements random access over frozen metadata and pinned blobs.
// One reader belongs to one response; it opens at most one blob file at a time.
type LayoutReader struct {
	ctx      context.Context
	store    *blob.Store
	plan     *FullLayout
	pos      int64
	file     *os.File
	fileID   string
	closed   bool
	metadata func(part int, offset int64, p []byte) (int, error)
}

// OpenFullLayout validates persisted boundaries before serving any bytes.
func OpenFullLayout(ctx context.Context, store *blob.Store, plan *FullLayout) (*LayoutReader, error) {
	return OpenIndexedFullLayout(ctx, store, plan, nil)
}

// OpenIndexedFullLayout reads persisted literals only for requested intervals.
// This keeps transfer memory bounded even when the text snapshot is large.
func OpenIndexedFullLayout(ctx context.Context, store *blob.Store, plan *FullLayout, metadata func(int, int64, []byte) (int, error)) (*LayoutReader, error) {
	if plan == nil || plan.Version != 1 || plan.Size < 0 {
		return nil, errors.New("archive: unknown ZIP index")
	}
	var end int64
	for _, part := range plan.Parts {
		if part.Offset != end || part.Size < 0 || part.Size > math.MaxInt64-end ||
			(part.BlobID == "" && int64(len(part.Data)) != part.Size && !(metadata != nil && len(part.Data) == 0)) || (part.BlobID != "" && len(part.Data) != 0) {
			return nil, errors.New("archive: corrupt ZIP index")
		}
		end += part.Size
	}
	if end != plan.Size {
		return nil, errors.New("archive: inconsistent ZIP length")
	}
	return &LayoutReader{ctx: ctx, store: store, plan: plan, metadata: metadata}, nil
}

func (r *LayoutReader) Read(p []byte) (int, error) {
	n, err := r.ReadAt(p, r.pos)
	r.pos += int64(n)
	return n, err
}
func (r *LayoutReader) ReadAt(p []byte, offset int64) (int, error) {
	if r.closed {
		return 0, os.ErrClosed
	}
	if offset < 0 {
		return 0, errors.New("archive: negative offset")
	}
	total := 0
	for len(p) > 0 && offset < r.plan.Size {
		if err := r.ctx.Err(); err != nil {
			return total, err
		}
		i := sort.Search(len(r.plan.Parts), func(i int) bool { return r.plan.Parts[i].Offset+r.plan.Parts[i].Size > offset })
		part := r.plan.Parts[i]
		inside := offset - part.Offset
		n := int(min(int64(len(p)), part.Size-inside))
		if part.BlobID == "" {
			if r.metadata != nil && len(part.Data) == 0 {
				read, err := r.metadata(i, inside, p[:n])
				if err != nil {
					return total + read, err
				}
				if read != n {
					return total + read, io.ErrUnexpectedEOF
				}
			} else {
				copy(p[:n], part.Data[inside:inside+int64(n)])
			}
		} else {
			if r.fileID != part.BlobID {
				if r.file != nil {
					_ = r.file.Close()
					r.file = nil
					r.fileID = ""
				}
				info, err := r.store.OpenBlob(r.ctx, part.BlobID)
				if err != nil {
					return total, err
				}
				f, err := os.Open(info.Path)
				if err != nil {
					return total, err
				}
				fi, err := f.Stat()
				if err != nil || fi.Size() != part.Size {
					f.Close()
					return total, errors.New("archive: pinned blob unavailable")
				}
				r.file, r.fileID = f, part.BlobID
			}
			read, err := r.file.ReadAt(p[:n], inside)
			if err != nil {
				return total + read, err
			}
		}
		total += n
		offset += int64(n)
		p = p[n:]
	}
	if len(p) > 0 {
		return total, io.EOF
	}
	return total, nil
}
func (r *LayoutReader) Seek(offset int64, whence int) (int64, error) {
	if r.closed {
		return r.pos, os.ErrClosed
	}
	base := int64(0)
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = r.pos
	case io.SeekEnd:
		base = r.plan.Size
	default:
		return r.pos, errors.New("archive: invalid seek")
	}
	if offset > 0 && base > math.MaxInt64-offset || offset < 0 && offset < -base {
		return r.pos, errors.New("archive: invalid offset")
	}
	r.pos = base + offset
	return r.pos, nil
}
func (r *LayoutReader) Close() error {
	r.closed = true
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
