// Package installer — ядро настольного установщика (план 45): подключение к
// чужому серверу по SSH и осмотр без изменений. Окно (cmd/wynd-installer)
// только показывает то, что решено здесь.
package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	dialTimeout    = 15 * time.Second
	commandTimeout = 30 * time.Second
	// Потолок вывода команды: осмотр печатает килобайты, а чужой сервер не
	// должен уметь залить память приложения.
	maxCommandOutput = 1 << 20
)

// Access — как войти на сервер. Пароль живёт только в памяти процесса.
type Access struct {
	Host     string
	Port     int
	User     string
	Password string
	// UseKeys — войти ключом с этого компьютера (~/.ssh) вместо пароля.
	UseKeys bool
}

// Отказы подключения, которые окно объясняет словами.
var (
	// ErrAuth — сервер не принял пароль или ключ.
	ErrAuth = errors.New("installer: access denied")
	// ErrUnreachable — сервер не ответил по этому адресу.
	ErrUnreachable = errors.New("installer: server unreachable")
	// ErrNoKeys — в ~/.ssh нет ключа, которым можно войти.
	ErrNoKeys = errors.New("installer: no usable keys")
	// ErrInvalid — адрес или пользователь не заполнены.
	ErrInvalid = errors.New("installer: invalid access")
)

// UnknownHostError — сервер встречен впервые. Подключение не состоялось:
// человек сверяет отпечаток и подтверждает, тогда его запоминает Trust.
type UnknownHostError struct {
	Host        string
	Fingerprint string
	key         ssh.PublicKey
	address     string
}

func (e *UnknownHostError) Error() string {
	return "installer: unknown host " + e.Host + " " + e.Fingerprint
}

// ChangedHostError — сервер назвал не тот отпечаток, что запомнен. Обойти из
// окна нельзя: либо сервер переустановили, либо между ним и нами кто-то есть.
type ChangedHostError struct {
	Host        string
	Fingerprint string
}

func (e *ChangedHostError) Error() string {
	return "installer: host key changed for " + e.Host
}

// Dialer подключается к серверам и помнит их отпечатки в known_hosts — том
// же файле, что у ssh: терминал потом тоже узнает сервер.
type Dialer struct {
	// SSHDir — каталог с known_hosts и ключами; пусто — ~/.ssh.
	SSHDir string
}

func (d Dialer) dir() (string, error) {
	if d.SSHDir != "" {
		return d.SSHDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".ssh"), nil
}

func (d Dialer) knownHostsPath() (string, error) {
	dir, err := d.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

func normalize(a Access) (Access, string, error) {
	a.Host = strings.TrimSpace(a.Host)
	a.User = strings.TrimSpace(a.User)
	if a.Port == 0 {
		a.Port = 22
	}
	// Адрес — только имя или IP: ни пробелов, ни «user@», ни схемы. Он уходит
	// в known_hosts и в сообщения окна.
	if a.Host == "" || a.User == "" || a.Port < 1 || a.Port > 65535 ||
		strings.ContainsAny(a.Host, " \t\r\n@/\\") || strings.ContainsAny(a.User, " \t\r\n") {
		return a, "", ErrInvalid
	}
	return a, net.JoinHostPort(a.Host, strconv.Itoa(a.Port)), nil
}

// Dial подключается и проверяет отпечаток. Сервер, которого нет в
// known_hosts, даёт *UnknownHostError — без входа и без записи в файл.
func (d Dialer) Dial(ctx context.Context, access Access) (*Session, error) {
	a, address, err := normalize(access)
	if err != nil {
		return nil, err
	}
	auth, err := d.authMethods(a)
	if err != nil {
		return nil, err
	}
	hostKey, err := d.hostKeyCallback(a.Host)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            a.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         dialTimeout,
	}
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	// Рукопожатие тоже под сроком: сервер, принявший TCP и замолчавший,
	// иначе держал бы окно на «Подключаемся…» без конца.
	_ = conn.SetDeadline(time.Now().Add(dialTimeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, cfg)
	if err != nil {
		_ = conn.Close()
		var unknown *UnknownHostError
		var changed *ChangedHostError
		switch {
		case errors.As(err, &unknown):
			return nil, unknown
		case errors.As(err, &changed):
			return nil, changed
		case strings.Contains(err.Error(), "unable to authenticate"):
			return nil, ErrAuth
		}
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	_ = conn.SetDeadline(time.Time{})
	return &Session{client: ssh.NewClient(sshConn, chans, reqs), Host: a.Host, User: a.User}, nil
}

// Trust запоминает отпечаток сервера, который человек подтвердил.
func (d Dialer) Trust(unknown *UnknownHostError) error {
	if unknown == nil || unknown.key == nil {
		return ErrInvalid
	}
	path, err := d.knownHostsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ssh dir: %w", err)
	}
	// Файл может не кончаться переводом строки — тогда запись слиплась бы с
	// чужой последней строкой и испортила обе.
	prefix := ""
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 && raw[len(raw)-1] != '\n' {
		prefix = "\n"
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("known_hosts: %w", err)
	}
	defer func() { _ = f.Close() }()
	line := knownhosts.Line([]string{knownhosts.Normalize(unknown.address)}, unknown.key)
	if _, err := f.WriteString(prefix + line + "\n"); err != nil {
		return fmt.Errorf("known_hosts: %w", err)
	}
	return nil
}

func (d Dialer) hostKeyCallback(host string) (ssh.HostKeyCallback, error) {
	path, err := d.knownHostsPath()
	if err != nil {
		return nil, err
	}
	var check ssh.HostKeyCallback
	if _, statErr := os.Stat(path); statErr == nil {
		check, err = knownhosts.New(path)
		if err != nil {
			return nil, fmt.Errorf("known_hosts: %w", err)
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		if check == nil {
			return &UnknownHostError{Host: host, Fingerprint: fingerprint, key: key, address: hostname}
		}
		err := check(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				return &UnknownHostError{Host: host, Fingerprint: fingerprint, key: key, address: hostname}
			}
			return &ChangedHostError{Host: host, Fingerprint: fingerprint}
		}
		return err
	}, nil
}

// keyFiles — имена ключей в порядке, в каком их пробует ssh.
var keyFiles = []string{"id_ed25519", "id_ecdsa", "id_rsa"}

func (d Dialer) authMethods(a Access) ([]ssh.AuthMethod, error) {
	if !a.UseKeys {
		if a.Password == "" {
			return nil, ErrInvalid
		}
		return []ssh.AuthMethod{ssh.Password(a.Password)}, nil
	}
	dir, err := d.dir()
	if err != nil {
		return nil, err
	}
	var signers []ssh.Signer
	for _, name := range keyFiles {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		// Ключ под паролем пропускаем: спрашивать его пароль окно пока не умеет.
		signer, err := ssh.ParsePrivateKey(raw)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	if len(signers) == 0 {
		return nil, ErrNoKeys
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}, nil
}

// Session — открытое подключение к серверу.
type Session struct {
	client *ssh.Client
	Host   string
	User   string
}

// Close закрывает подключение.
func (s *Session) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// RemoteIP — адрес, по которому подключение состоялось: с ним сверяют домен.
func (s *Session) RemoteIP() string {
	if tcp, ok := s.client.RemoteAddr().(*net.TCPAddr); ok {
		return tcp.IP.String()
	}
	return ""
}

// limitedBuffer копит вывод до потолка, остальное молча отбрасывает.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

// Run выполняет команду на сервере и возвращает её вывод. Ненулевой код
// выхода — не ошибка подключения: осмотр читает вывод и в этом случае.
func (s *Session) Run(ctx context.Context, command string) (string, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer func() { _ = sess.Close() }()
	out := &limitedBuffer{max: maxCommandOutput}
	sess.Stdout = out
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()
	select {
	case err := <-done:
		var exit *ssh.ExitError
		if err != nil && !errors.As(err, &exit) {
			return out.buf.String(), fmt.Errorf("%w: %v", ErrUnreachable, err)
		}
		return out.buf.String(), nil
	case <-runCtx.Done():
		_ = sess.Close()
		return "", fmt.Errorf("%w: %v", ErrUnreachable, runCtx.Err())
	}
}
