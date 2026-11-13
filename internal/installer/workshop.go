package installer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ServerProfile contains local connection preferences, never credentials or SSH output.
type ServerProfile struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Host        string       `json:"host"`
	User        string       `json:"user"`
	UseKeys     bool         `json:"use_keys"`
	Remember    bool         `json:"remember"`
	AutoConnect bool         `json:"auto_connect"`
	// Root — каталог установки на этом сервере (план «другой каталог»).
	// Пусто — /opt/wynd: профили прежних версий.
	Root      string       `json:"root,omitempty"`
	LastCheck *ServerCheck `json:"last_check,omitempty"`
}

// ServerCheck is explicitly historical; reconnect and inspect before any remote change.
type ServerCheck struct {
	At        time.Time `json:"at"`
	Domain    string    `json:"domain,omitempty"`
	Version   string    `json:"version,omitempty"`
	Installed bool      `json:"installed"`
	Running   bool      `json:"running"`
	FreeBytes uint64    `json:"free_bytes"`
	// Root — фактический каталог установки с сервера; пусто — неизвестен
	// (не осматривали или установки нет).
	Root string `json:"root,omitempty"`
}

type ProfileState struct {
	ServerProfile
	Busy       bool   `json:"busy"`
	ReservedBy string `json:"reserved_by,omitempty"`
}

var (
	ErrProfileMissing = errors.New("Сервер больше не сохранён в списке")
	ErrProfileBusy    = errors.New("Сначала дождитесь окончания операции на этом сервере")
	ErrWorkshopBusy   = errors.New("Сначала дождитесь окончания операций на всех серверах")
)

type profileSession struct {
	profile  ServerProfile
	wizard   *Wizard
	active   bool
	running  bool
	target   string
	owner    string
	endpoint string
	removing bool
}

// Workshop owns sessions by stable profile ID. Selecting a view never changes
// the destination of an in-flight call. mu protects metadata only: network and
// keyring calls must not hold it, so disabling auto-connect remains immediate.
type Workshop struct {
	mu       sync.Mutex
	vaultMu  sync.Mutex
	path     string
	create   func() *Wizard
	sessions []*profileSession
	updating bool
	closed   bool
}

func OpenWorkshop(path string, create func() *Wizard) (*Workshop, error) {
	w := &Workshop{path: path, create: create}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return w, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return nil, err
	}
	var disk struct {
		Version int             `json:"version"`
		Servers []ServerProfile `json:"servers"`
	}
	if len(data) > 4<<20 {
		return nil, errors.New("Список серверов слишком велик")
	}
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("Не удалось прочитать список серверов: %w", err)
	}
	if disk.Version != 1 {
		return nil, errors.New("Этот список серверов создан другой версией Мастерской")
	}
	seen := map[string]bool{}
	for _, p := range disk.Servers {
		if p.ID == "" || seen[p.ID] {
			return nil, errors.New("В списке серверов повторяется или отсутствует идентификатор")
		}
		if err := validateProfile(p); err != nil {
			return nil, err
		}
		seen[p.ID] = true
		wiz := w.makeWizard(p.ID)
		wiz.Root = p.Root
		w.sessions = append(w.sessions, &profileSession{profile: p, wizard: wiz})
	}
	return w, nil
}

// Profile registration and credential deletion share a lock; network calls and
// local auto-connect toggles do not. A login that opts out of remembering can
// remove its old password without deleting one used by another profile.
type profileVault struct {
	workshop *Workshop
	id       string
	base     Vault
}

func (v profileVault) Get(host, user string) (string, bool)  { return v.base.Get(host, user) }
func (v profileVault) Set(host, user, password string) error { return v.base.Set(host, user, password) }
func (v profileVault) Delete(host, user string) error {
	v.workshop.vaultMu.Lock()
	defer v.workshop.vaultMu.Unlock()
	v.workshop.mu.Lock()
	shared := false
	for _, s := range v.workshop.sessions {
		if s.profile.ID != v.id && s.profile.Remember && !s.profile.UseKeys && strings.EqualFold(host, s.profile.Host) && user == s.profile.User {
			shared = true
		}
	}
	v.workshop.mu.Unlock()
	if shared {
		return nil
	}
	return v.base.Delete(host, user)
}
func (w *Workshop) makeWizard(id string) *Wizard {
	wiz := w.create()
	if wiz.Vault != nil {
		wiz.Vault = profileVault{workshop: w, id: id, base: wiz.Vault}
	}
	// Каталог установки профиля, если он уже в списке (при загрузке список
	// наполняется после создания мастера — там каталог ставится отдельно).
	if s := w.find(id); s != nil {
		wiz.Root = s.profile.Root
	}
	return wiz
}

func validateProfile(p ServerProfile) error {
	if _, _, err := normalize(Access{Host: p.Host, User: p.User}); err != nil {
		return errors.New("Укажите адрес SSH и пользователя без пробелов и схемы https://")
	}
	if p.Name == "" || len(p.Name) > 200 || len(p.Host) > 253 || len(p.User) > 128 || strings.ContainsAny(p.Name, "\r\n\x00") {
		return errors.New("Укажите короткое имя сервера")
	}
	return nil
}

func (w *Workshop) find(id string) *profileSession {
	for _, s := range w.sessions {
		if s.profile.ID == id {
			return s
		}
	}
	return nil
}

func cloneProfile(p ServerProfile) ServerProfile {
	if p.LastCheck != nil {
		c := *p.LastCheck
		p.LastCheck = &c
	}
	return p
}

func (w *Workshop) Profiles() []ProfileState {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]ProfileState, 0, len(w.sessions))
	for _, s := range w.sessions {
		out = append(out, ProfileState{ServerProfile: cloneProfile(s.profile), Busy: s.active || s.running || s.owner != "", ReservedBy: s.owner})
	}
	return out
}

// saveLocked replaces the complete file atomically. A failed write never
// silently turns a preference into a memory-only change.
func (w *Workshop) saveLocked() error {
	profiles := make([]ServerProfile, 0, len(w.sessions))
	for _, s := range w.sessions {
		profiles = append(profiles, s.profile)
	}
	data, err := json.MarshalIndent(struct {
		Version int             `json:"version"`
		Servers []ServerProfile `json:"servers"`
	}{1, profiles}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(w.path), ".servers-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, w.path)
	}
	return err
}

func (w *Workshop) SaveProfile(p ServerProfile) (ServerProfile, error) {
	w.vaultMu.Lock()
	defer w.vaultMu.Unlock()
	p.Name, p.Host, p.User = strings.TrimSpace(p.Name), strings.TrimSpace(p.Host), strings.TrimSpace(p.User)
	if err := validateProfile(p); err != nil {
		return ServerProfile{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.updating {
		return ServerProfile{}, ErrWorkshopBusy
	}
	for _, other := range w.sessions {
		if other.removing && strings.EqualFold(p.Host, other.profile.Host) && p.User == other.profile.User {
			return ServerProfile{}, ErrProfileBusy
		}
	}
	s := w.find(p.ID)
	if p.ID != "" && s == nil {
		return ServerProfile{}, ErrProfileMissing
	}
	if s != nil && (s.active || s.running || s.owner != "" || s.target != "") {
		return ServerProfile{}, ErrProfileBusy
	}
	if s == nil {
		var id [16]byte
		if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
			return ServerProfile{}, err
		}
		p.ID, p.LastCheck = hex.EncodeToString(id[:]), nil
		s = &profileSession{wizard: w.makeWizard(p.ID)}
		w.sessions = append(w.sessions, s)
	} else {
		p.LastCheck = s.profile.LastCheck
		if !strings.EqualFold(p.Host, s.profile.Host) || p.User != s.profile.User {
			p.LastCheck = nil
		}
	}
	old := s.profile
	// Каталог установки (план «другой каталог»): у существующего профиля он
	// сохраняется, если не задан новый; менять его устоявшейся установки
	// простым редактированием нельзя — тогда обновление и удаление ушли бы в
	// другую папку. Проверяем до записи.
	if p.Root != "" {
		l, err := NewLayout(p.Root)
		if err != nil {
			return ServerProfile{}, err
		}
		p.Root = l.Root()
	}
	if old.ID != "" {
		if p.Root == "" {
			p.Root = old.Root
		} else if p.Root != old.Root && old.LastCheck != nil && old.LastCheck.Installed {
			return ServerProfile{}, errors.New("Каталог установки этого сервера уже известен — он меняется переездом, а не правкой профиля")
		}
	}
	// Password-based auto-connect is enabled only by SetAutoConnect after
	// successful authentication and storage. Editing access invalidates it.
	if !p.UseKeys {
		p.AutoConnect = old.AutoConnect && p.Remember && p.Host == old.Host && p.User == old.User && p.UseKeys == old.UseKeys && p.AutoConnect
	}
	s.profile = p
	if err := w.saveLocked(); err != nil {
		s.profile = old
		if old.ID == "" {
			w.sessions = w.sessions[:len(w.sessions)-1]
		}
		return ServerProfile{}, err
	}
	// Следующая операция профиля считает пути от этого каталога.
	s.wizard.Root = p.Root
	// No remote request is running; discard the old connection and any plans.
	s.wizard.Close()
	return cloneProfile(p), nil
}

// SetInstallRoot задаёт каталог установки до установки (план «другой
// каталог установки Docker»): проверяет путь, сохраняет в профиле и отдаёт
// мастеру, чтобы план и установка считали его. У устоявшейся установки
// каталог так не меняется — только переездом.
func (w *Workshop) SetInstallRoot(id, root string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.find(id)
	if s == nil {
		return ErrProfileMissing
	}
	if s.active || s.running || s.owner != "" || s.target != "" {
		return ErrProfileBusy
	}
	l, err := NewLayout(root)
	if err != nil {
		return err
	}
	if s.profile.Root != "" && s.profile.Root != l.Root() &&
		s.profile.LastCheck != nil && s.profile.LastCheck.Installed {
		return errors.New("Каталог установки этого сервера уже известен — он меняется переездом, а не правкой профиля")
	}
	old := s.profile.Root
	s.profile.Root = l.Root()
	if err := w.saveLocked(); err != nil {
		s.profile.Root = old
		return err
	}
	s.wizard.Root = l.Root()
	return nil
}

// SetAutoConnect is local even while SSH or another operation is blocked.
// Enabling needs a key or a password actually present in the OS vault.
func (w *Workshop) SetAutoConnect(id string, on bool) error {
	w.mu.Lock()
	s := w.find(id)
	if s == nil {
		w.mu.Unlock()
		return ErrProfileMissing
	}
	p, vault := s.profile, s.wizard.Vault
	w.mu.Unlock()
	if on && !p.UseKeys {
		if vault == nil {
			return errors.New("Для автоподключения сохраните пароль или выберите ключ")
		}
		if _, ok := vault.Get(p.Host, p.User); !ok || !p.Remember {
			return errors.New("Для автоподключения сохраните пароль или выберите ключ")
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.find(id) != s || s.profile.Host != p.Host || s.profile.User != p.User || s.profile.UseKeys != p.UseKeys || s.profile.Remember != p.Remember {
		return ErrProfileMissing
	}
	old := s.profile.AutoConnect
	s.profile.AutoConnect = on
	if err := w.saveLocked(); err != nil {
		s.profile.AutoConnect = old
		return err
	}
	return nil
}

// WithProfile serializes requests for one profile and refuses conflicts with
// asynchronous work or a move destination. Progress is read separately.
func (w *Workshop) WithProfile(id string, allowRunning bool, fn func(*Wizard, ServerProfile)) error {
	return w.withProfile(id, allowRunning, false, fn)
}

func (w *Workshop) WithMoveProfile(id string, allowRunning bool, fn func(*Wizard, ServerProfile)) error {
	return w.withProfile(id, allowRunning, true, fn)
}

func (w *Workshop) withProfile(id string, allowRunning, move bool, fn func(*Wizard, ServerProfile)) error {
	w.mu.Lock()
	s := w.find(id)
	if s == nil {
		w.mu.Unlock()
		return ErrProfileMissing
	}
	if w.closed || w.updating {
		w.mu.Unlock()
		return ErrWorkshopBusy
	}
	if s.active || s.owner != "" || (s.running && !allowRunning) || (s.target != "" && !move) {
		w.mu.Unlock()
		return ErrProfileBusy
	}
	// Two profiles of the same SSH host cannot mutate it concurrently.
	for _, other := range w.sessions {
		if other != s && (strings.EqualFold(other.profile.Host, s.profile.Host) || (s.endpoint != "" && s.endpoint == other.endpoint)) && (other.active || other.running || other.owner != "" || other.target != "") {
			w.mu.Unlock()
			return ErrProfileBusy
		}
	}
	s.active = true
	p := cloneProfile(s.profile)
	w.mu.Unlock()
	defer func() {
		s.wizard.mu.Lock()
		running := s.wizard.progress.Running
		endpoint := ""
		if s.wizard.session != nil {
			endpoint = s.wizard.session.client.RemoteAddr().String()
		}
		s.wizard.mu.Unlock()
		w.mu.Lock()
		s.active, s.running = false, running
		s.endpoint = endpoint
		w.mu.Unlock()
	}()
	fn(s.wizard, p)
	return nil
}

func (w *Workshop) Progress(id string) (Progress, error) {
	w.mu.Lock()
	s := w.find(id)
	w.mu.Unlock()
	if s == nil {
		return Progress{}, ErrProfileMissing
	}
	p := s.wizard.InstallProgress()
	w.mu.Lock()
	s.running = p.Running
	w.mu.Unlock()
	return p, nil
}

func (w *Workshop) RememberCheck(id string, check ServerCheck) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.find(id)
	if s == nil {
		return ErrProfileMissing
	}
	old := s.profile.LastCheck
	oldRoot := s.profile.Root
	s.profile.LastCheck = &check
	// Фактический каталог установки подтверждается осмотром: если в профиле
	// его ещё нет — запоминаем (план «другой каталог установки Docker»).
	if check.Installed && check.Root != "" && s.profile.Root == "" {
		s.profile.Root = check.Root
		s.wizard.Root = check.Root
	}
	if err := w.saveLocked(); err != nil {
		s.profile.LastCheck = old
		s.profile.Root = oldRoot
		return err
	}
	return nil
}

// ReserveMove occupies the destination throughout planning, fingerprint
// confirmation, execution and recovery. Auto-connect preferences stay local.
func (w *Workshop) ReserveMove(source, target string) (ServerProfile, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	s, t := w.find(source), w.find(target)
	if s == nil || t == nil {
		return ServerProfile{}, ErrProfileMissing
	}
	if s == t || strings.EqualFold(s.profile.Host, t.profile.Host) {
		return ServerProfile{}, errors.New("Для переезда нужен другой сервер")
	}
	if w.closed || w.updating || s.active || s.running || s.owner != "" || t.active || t.running || (t.owner != "" && t.owner != source) || t.target != "" {
		return ServerProfile{}, ErrProfileBusy
	}
	for _, other := range w.sessions {
		if other != t && (strings.EqualFold(other.profile.Host, t.profile.Host) || (t.endpoint != "" && other.endpoint == t.endpoint)) && (other.active || other.running || other.owner != "" || other.target != "") {
			return ServerProfile{}, ErrProfileBusy
		}
	}
	if old := w.find(s.target); old != nil {
		old.owner = ""
	}
	s.target, t.owner = target, source
	return cloneProfile(t.profile), nil
}

// BindMoveEndpoint runs after the destination's handshake, before planning.
// Hostnames may be aliases: reserve the actual SSH endpoint as well as its ID.
func (w *Workshop) BindMoveEndpoint(source string) error {
	w.mu.Lock()
	s := w.find(source)
	if s == nil {
		w.mu.Unlock()
		return ErrProfileMissing
	}
	w.mu.Unlock()
	s.wizard.mu.Lock()
	endpoint := ""
	if s.wizard.target != nil {
		endpoint = s.wizard.target.client.RemoteAddr().String()
	}
	s.wizard.mu.Unlock()
	if endpoint == "" {
		return ErrProfileMissing
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	t := w.find(s.target)
	if t == nil || t.owner != source {
		return ErrProfileMissing
	}
	if endpoint == s.endpoint {
		return errors.New("Для переезда нужен другой сервер")
	}
	for _, other := range w.sessions {
		if other != t && other != s && other.endpoint == endpoint && (other.active || other.running || other.owner != "" || other.target != "") {
			return ErrProfileBusy
		}
	}
	t.endpoint = endpoint
	return nil
}

func (w *Workshop) ReleaseMove(source string) (string, error) {
	w.mu.Lock()
	s := w.find(source)
	if s == nil {
		w.mu.Unlock()
		return "", ErrProfileMissing
	}
	if s.active || s.running {
		w.mu.Unlock()
		return "", ErrProfileBusy
	}
	s.active = true
	target := s.target
	t := w.find(target)
	w.mu.Unlock()
	endpoint := ""
	// Move the target connection into its own profile, preserving both SSH
	// identities and allowing password-in-memory access after the move.
	s.wizard.mu.Lock()
	if t != nil && s.wizard.target != nil {
		t.wizard.mu.Lock()
		if t.wizard.session != nil {
			_ = t.wizard.session.Close()
		}
		t.wizard.session, t.wizard.access, t.wizard.input = s.wizard.target, s.wizard.tAccess, s.wizard.tInput
		t.wizard.report, t.wizard.state, t.wizard.plan = nil, nil, nil
		t.wizard.pending, t.wizard.changed = nil, nil
		t.wizard.progress = Progress{}
		endpoint = t.wizard.session.client.RemoteAddr().String()
		t.wizard.mu.Unlock()
		s.wizard.target = nil
	}
	s.wizard.mu.Unlock()
	s.wizard.ReleaseMoveTarget()
	w.mu.Lock()
	if t := w.find(target); t != nil {
		t.owner = ""
		if endpoint != "" {
			t.endpoint = endpoint
		}
	}
	s.target, s.active = "", false
	w.mu.Unlock()
	return target, nil
}

// MoveInput can reuse a credential already held by the destination session;
// it never exports that credential through the frontend API or profile file.
func (w *Workshop) MoveInput(source, target, password string) (ConnectInput, error) {
	w.mu.Lock()
	t := w.find(target)
	if t == nil || t.owner != source {
		w.mu.Unlock()
		return ConnectInput{}, ErrProfileMissing
	}
	p := t.profile
	w.mu.Unlock()
	if password == "" {
		t.wizard.mu.Lock()
		if t.wizard.session != nil {
			password = t.wizard.access.Password
		}
		t.wizard.mu.Unlock()
	}
	return ConnectInput{Host: p.Host, User: p.User, UseKeys: p.UseKeys, Remember: p.Remember, Password: password}, nil
}

func (w *Workshop) RemoveProfile(id string) error {
	w.vaultMu.Lock()
	defer w.vaultMu.Unlock()
	w.mu.Lock()
	s := w.find(id)
	if s == nil {
		w.mu.Unlock()
		return ErrProfileMissing
	}
	if w.updating || s.active || s.running || s.owner != "" || s.target != "" {
		w.mu.Unlock()
		return ErrProfileBusy
	}
	s.active, s.removing = true, true
	p, vault := s.profile, s.wizard.Vault
	if scoped, ok := vault.(profileVault); ok {
		vault = scoped.base
	}
	shared := false
	for _, other := range w.sessions {
		if other != s && other.profile.Remember && !other.profile.UseKeys && strings.EqualFold(other.profile.Host, p.Host) && other.profile.User == p.User {
			shared = true
		}
	}
	w.mu.Unlock()
	defer func() { w.mu.Lock(); s.active, s.removing = false, false; w.mu.Unlock() }()
	var password string
	var saved bool
	if !shared && vault != nil {
		password, saved = vault.Get(p.Host, p.User)
		if err := vault.Delete(p.Host, p.User); err != nil {
			return err
		}
	}
	w.mu.Lock()
	idx := 0
	for w.sessions[idx] != s {
		idx++
	}
	old := append([]*profileSession(nil), w.sessions...)
	w.sessions = append(w.sessions[:idx], w.sessions[idx+1:]...)
	if err := w.saveLocked(); err != nil {
		w.sessions = old
		w.mu.Unlock()
		if !shared && saved {
			err = errors.Join(err, vault.Set(p.Host, p.User, password))
		}
		return err
	}
	w.mu.Unlock()
	s.wizard.Close()
	return nil
}

// BeginSelfUpdate prevents new server work until EndSelfUpdate, including the
// download interval; a check of the selected screen alone is insufficient.
func (w *Workshop) BeginSelfUpdate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.updating {
		return ErrWorkshopBusy
	}
	for _, s := range w.sessions {
		if s.active || s.running || s.owner != "" || s.target != "" {
			return ErrWorkshopBusy
		}
	}
	w.updating = true
	return nil
}

func (w *Workshop) EndSelfUpdate() {
	w.mu.Lock()
	w.updating = false
	w.mu.Unlock()
}

func (w *Workshop) Close() {
	w.mu.Lock()
	w.closed = true
	sessions := append([]*profileSession(nil), w.sessions...)
	w.mu.Unlock()
	for _, s := range sessions {
		s.wizard.Close()
	}
}
