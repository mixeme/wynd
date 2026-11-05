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
	LastCheck   *ServerCheck `json:"last_check,omitempty"`
}

// ServerCheck is explicitly historical; reconnect and inspect before any remote change.
type ServerCheck struct {
	At        time.Time `json:"at"`
	Domain    string    `json:"domain,omitempty"`
	Version   string    `json:"version,omitempty"`
	Installed bool      `json:"installed"`
	Running   bool      `json:"running"`
	FreeBytes uint64    `json:"free_bytes"`
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
	profile ServerProfile
	wizard  *Wizard
	active  bool
	running bool
	target  string
	owner   string
}

// Workshop owns sessions by stable profile ID. Selecting a view never changes
// the destination of an in-flight call. mu protects metadata only: network and
// keyring calls must not hold it, so disabling auto-connect remains immediate.
type Workshop struct {
	mu       sync.Mutex
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
		w.sessions = append(w.sessions, &profileSession{profile: p, wizard: create()})
	}
	return w, nil
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
	p.Name, p.Host, p.User = strings.TrimSpace(p.Name), strings.TrimSpace(p.Host), strings.TrimSpace(p.User)
	if err := validateProfile(p); err != nil {
		return ServerProfile{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.updating {
		return ServerProfile{}, ErrWorkshopBusy
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
		s = &profileSession{wizard: w.create()}
		w.sessions = append(w.sessions, s)
	} else {
		p.LastCheck = s.profile.LastCheck
		if !strings.EqualFold(p.Host, s.profile.Host) || p.User != s.profile.User {
			p.LastCheck = nil
		}
	}
	old := s.profile
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
	// No remote request is running; discard the old connection and any plans.
	s.wizard.Close()
	return cloneProfile(p), nil
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
	if s.active || s.owner != "" || (s.running && !allowRunning) {
		w.mu.Unlock()
		return ErrProfileBusy
	}
	// Two profiles of the same SSH host cannot mutate it concurrently.
	for _, other := range w.sessions {
		if other != s && strings.EqualFold(other.profile.Host, s.profile.Host) && (other.active || other.running || other.owner != "") {
			w.mu.Unlock()
			return ErrProfileBusy
		}
	}
	s.active = true
	p := cloneProfile(s.profile)
	w.mu.Unlock()
	defer func() {
		running := s.wizard.InstallProgress().Running
		w.mu.Lock()
		s.active, s.running = false, running
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
	s.profile.LastCheck = &check
	if err := w.saveLocked(); err != nil {
		s.profile.LastCheck = old
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
		if other != t && strings.EqualFold(other.profile.Host, t.profile.Host) && (other.active || other.running || other.owner != "" || other.target != "") {
			return ServerProfile{}, ErrProfileBusy
		}
	}
	if old := w.find(s.target); old != nil {
		old.owner = ""
	}
	s.target, t.owner = target, source
	return cloneProfile(t.profile), nil
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
	w.mu.Unlock()
	s.wizard.ReleaseMoveTarget()
	w.mu.Lock()
	if t := w.find(target); t != nil {
		t.owner = ""
	}
	s.target, s.active = "", false
	w.mu.Unlock()
	return target, nil
}

func (w *Workshop) RemoveProfile(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.find(id)
	if s == nil {
		return ErrProfileMissing
	}
	if w.updating || s.active || s.running || s.owner != "" || s.target != "" {
		return ErrProfileBusy
	}
	idx := 0
	for w.sessions[idx] != s {
		idx++
	}
	old := append([]*profileSession(nil), w.sessions...)
	w.sessions = append(w.sessions[:idx], w.sessions[idx+1:]...)
	if err := w.saveLocked(); err != nil {
		w.sessions = old
		return err
	}
	s.wizard.Close()
	for _, other := range w.sessions {
		if strings.EqualFold(other.profile.Host, s.profile.Host) && other.profile.User == s.profile.User {
			return nil
		}
	}
	if s.wizard.Vault != nil {
		return s.wizard.Vault.Delete(s.profile.Host, s.profile.User)
	}
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
