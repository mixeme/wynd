package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var sshFilesMu sync.Mutex

// ReplaceHost requires the exact old and new keys observed before the user's
// confirmation. Shared/wildcard/CA entries require manual maintenance, since
// replacing their line could affect unrelated hosts.
func (d Dialer) ReplaceHost(changed *ChangedHostError) error {
	if changed == nil || changed.key == nil || len(changed.previous) == 0 {
		return ErrInvalid
	}
	sshFilesMu.Lock()
	defer sshFilesMu.Unlock()
	path, err := d.knownHostsPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	for i, old := range changed.previous {
		if old.Filename != path || old.Line < 1 || old.Line > len(lines) {
			return errors.New("Сохранённый отпечаток изменился. Подключитесь ещё раз")
		}
		marker, hosts, key, _, _, err := ssh.ParseKnownHosts([]byte(lines[old.Line-1]))
		if err != nil || !bytes.Equal(key.Marshal(), old.Key.Marshal()) {
			return errors.New("Сохранённый отпечаток изменился. Подключитесь ещё раз")
		}
		if marker != "" || len(hosts) != 1 || (hosts[0] != knownhosts.Normalize(changed.address) && !strings.HasPrefix(hosts[0], "|1|")) {
			return errors.New("Эта запись known_hosts используется несколькими серверами. Обновите её вручную и подключитесь ещё раз")
		}
		lines[old.Line-1] = ""
		if i == 0 {
			lines[old.Line-1] = knownhosts.Line([]string{knownhosts.Normalize(changed.address)}, changed.key)
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".known-hosts-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.WriteString(strings.Join(lines, "\n")); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, current) {
		return errors.New("Файл known_hosts изменился. Подключитесь ещё раз")
	}
	return os.Rename(name, path)
}

func previousFingerprints(changed *ChangedHostError) string {
	var values []string
	for _, known := range changed.previous {
		values = append(values, ssh.FingerprintSHA256(known.Key))
	}
	return strings.Join(values, "\n")
}
