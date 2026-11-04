package archive

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ReadLimits bounds compressed-directory metadata and decompressed input.
// Zero fields select defaults. Callers may lower limits for small tests.
type ReadLimits struct {
	ManifestBytes, TableBytes, JSONBytes int64
	MediaBytes                           uint64
	Entries                              int
	Rows                                 int
	DirectoryBytes                       uint64
}

func (l ReadLimits) defaults() ReadLimits {
	if l.ManifestBytes <= 0 {
		l.ManifestBytes = 1 << 20
	}
	if l.TableBytes <= 0 {
		l.TableBytes = 16 << 20
	}
	if l.JSONBytes <= 0 {
		l.JSONBytes = 32 << 20
	}
	if l.MediaBytes == 0 {
		l.MediaBytes = 32 << 30
	}
	if l.Entries <= 0 {
		l.Entries = 100000
	}
	if l.Rows <= 0 {
		l.Rows = 250000
	}
	if l.DirectoryBytes == 0 {
		l.DirectoryBytes = 16 << 20
	}
	return l
}

// Check before archive/zip allocates a File for every directory entry. Wynd ZIPs
// are single-volume, without an executable prefix; normal and ZIP64 are accepted.
func checkFullDirectory(path string, limits ReadLimits) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	if size < 22 {
		return fmt.Errorf("archive: missing ZIP directory")
	}
	tail := make([]byte, min(size, 65535+22))
	if _, err := f.ReadAt(tail, size-int64(len(tail))); err != nil {
		return err
	}
	end := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:i+4]) == 0x06054b50 {
			if i+22+int(binary.LittleEndian.Uint16(tail[i+20:i+22])) > len(tail) {
				break
			}
			end = i
			break
		}
	}
	if end < 0 {
		return fmt.Errorf("archive: invalid ZIP end")
	}
	e := tail[end:]
	if binary.LittleEndian.Uint16(e[4:6]) != 0 || binary.LittleEndian.Uint16(e[6:8]) != 0 {
		return fmt.Errorf("archive: multi-volume ZIP")
	}
	count := uint64(binary.LittleEndian.Uint16(e[10:12]))
	dirSize, offset := uint64(binary.LittleEndian.Uint32(e[12:16])), uint64(binary.LittleEndian.Uint32(e[16:20]))
	boundary := size - int64(len(tail)) + int64(end)
	if count == 0xffff || dirSize == 0xffffffff || offset == 0xffffffff {
		var locator [20]byte
		if boundary < 20 {
			return fmt.Errorf("archive: missing ZIP64 locator")
		}
		if _, err := f.ReadAt(locator[:], boundary-20); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(locator[:4]) != 0x07064b50 || binary.LittleEndian.Uint32(locator[4:8]) != 0 || binary.LittleEndian.Uint32(locator[16:20]) != 1 {
			return fmt.Errorf("archive: invalid ZIP64 locator")
		}
		pos := binary.LittleEndian.Uint64(locator[8:16])
		if pos > uint64(boundary-20) || uint64(boundary-20)-pos < 56 {
			return fmt.Errorf("archive: invalid ZIP64 offset")
		}
		var record [56]byte
		if _, err := f.ReadAt(record[:], int64(pos)); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(record[:4]) != 0x06064b50 || binary.LittleEndian.Uint32(record[16:20]) != 0 || binary.LittleEndian.Uint32(record[20:24]) != 0 {
			return fmt.Errorf("archive: invalid ZIP64 record")
		}
		count, dirSize, offset = binary.LittleEndian.Uint64(record[32:40]), binary.LittleEndian.Uint64(record[40:48]), binary.LittleEndian.Uint64(record[48:56])
		boundary = int64(pos)
	}
	if count > uint64(limits.Entries) || dirSize > limits.DirectoryBytes || offset > uint64(boundary) || dirSize != uint64(boundary)-offset {
		return fmt.Errorf("archive: ZIP directory exceeds limits or has an invalid range")
	}
	// Do not trust the advertised count: walk the bounded actual directory too.
	actual := uint64(0)
	for remaining := dirSize; remaining > 0; {
		var header [46]byte
		if remaining < uint64(len(header)) {
			return fmt.Errorf("archive: truncated ZIP directory")
		}
		if _, err := f.ReadAt(header[:], int64(offset)); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(header[:4]) != 0x02014b50 {
			return fmt.Errorf("archive: invalid ZIP directory entry")
		}
		length := uint64(46) + uint64(binary.LittleEndian.Uint16(header[28:30])) + uint64(binary.LittleEndian.Uint16(header[30:32])) + uint64(binary.LittleEndian.Uint16(header[32:34]))
		if length > remaining {
			return io.ErrUnexpectedEOF
		}
		actual++
		if actual > uint64(limits.Entries) {
			return fmt.Errorf("archive: too many ZIP entries")
		}
		offset += length
		remaining -= length
	}
	if actual != count {
		return fmt.Errorf("archive: ZIP directory count mismatch")
	}
	return nil
}
