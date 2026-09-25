package check

import (
	"context"
	"encoding/binary"
	"net"
	"time"
)

const ntpEpochOffset = 2208988800

func ntpDrift(ctx context.Context, local time.Time) (time.Duration, error) {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "udp", "pool.ntp.org:123")
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	req := make([]byte, 48)
	req[0] = 0x1b
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Second)
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write(req); err != nil {
		return 0, err
	}
	resp := make([]byte, 48)
	if _, err := conn.Read(resp); err != nil {
		return 0, err
	}
	sec := binary.BigEndian.Uint32(resp[40:44])
	frac := binary.BigEndian.Uint32(resp[44:48])
	ntpSec := float64(sec-ntpEpochOffset) + float64(frac)/float64(1<<32)
	remote := time.Unix(int64(ntpSec), int64((ntpSec-float64(int64(ntpSec)))*1e9)).UTC()
	return local.Sub(remote), nil
}
