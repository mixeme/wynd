package mail

import "testing"

// Размеры в письме — как на экране: клиентский formatBytes.
func TestFormatBytesMatchesClient(t *testing.T) {
	const gb = int64(1) << 30
	for _, c := range []struct {
		in   int64
		want string
	}{
		{20 * gb, "20 ГБ"},
		{10 * gb, "10 ГБ"},
		{gb + gb/2, "1,5 ГБ"},
		{512 << 20, "512 МБ"},
		{0, "0 Б"},
	} {
		if got := formatBytes(c.in); got != c.want {
			t.Errorf("formatBytes(%d) = %q, ждали %q", c.in, got, c.want)
		}
	}
}
