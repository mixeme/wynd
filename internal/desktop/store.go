package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrUnknownServer — сервера нет в списке.
var ErrUnknownServer = errors.New("unknown server")

// Server — добавленный сервер. Токенов и паролей здесь нет: вход живёт в
// сессии клиента, а список серверов — только адреса и имена.
type Server struct {
	Origin           string    `json:"origin"`
	Name             string    `json:"name"`
	Version          string    `json:"version,omitempty"`
	RegistrationMode string    `json:"registration_mode,omitempty"`
	AddedAt          time.Time `json:"added_at"`
}

type storeFile struct {
	Selected string   `json:"selected"`
	Servers  []Server `json:"servers"`
}

// Store — servers.json в папке настроек. Все изменения пишутся сразу, атомарно
// через временный файл: оборванная запись не оставляет половину списка.
type Store struct {
	mu   sync.Mutex
	path string
	data storeFile
}

// OpenStore читает список; отсутствие файла — пустой список, а не ошибка.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read servers: %w", err)
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("parse servers: %w", err)
	}
	// Выбранный сервер обязан быть в списке; иначе — первый.
	if s.data.Selected != "" && s.find(s.data.Selected) < 0 {
		s.data.Selected = ""
	}
	if s.data.Selected == "" && len(s.data.Servers) > 0 {
		s.data.Selected = s.data.Servers[0].Origin
	}
	return s, nil
}

func (s *Store) find(origin string) int {
	for i, server := range s.data.Servers {
		if server.Origin == origin {
			return i
		}
	}
	return -1
}

// Snapshot — выбранный адрес и копия списка.
func (s *Store) Snapshot() (string, []Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	servers := make([]Server, len(s.data.Servers))
	copy(servers, s.data.Servers)
	return s.data.Selected, servers
}

// Put добавляет сервер или обновляет имя; selectIt делает его выбранным.
func (s *Store) Put(server Server, selectIt bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if server.Origin == "" {
		return ErrInvalidAddress
	}
	if server.AddedAt.IsZero() {
		server.AddedAt = time.Now().UTC()
	}
	if i := s.find(server.Origin); i >= 0 {
		s.data.Servers[i] = server
	} else {
		s.data.Servers = append(s.data.Servers, server)
	}
	if selectIt {
		s.data.Selected = server.Origin
	}
	return s.writeLocked()
}

// Select делает сервер выбранным.
func (s *Store) Select(origin string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.find(origin) < 0 {
		return ErrUnknownServer
	}
	s.data.Selected = origin
	return s.writeLocked()
}

// ClearSelection снимает выбор, оставляя серверы в списке.
func (s *Store) ClearSelection() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Selected = ""
	return s.writeLocked()
}

// Remove убирает сервер; выбранным становится следующий.
func (s *Store) Remove(origin string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(origin)
	if i < 0 {
		return ErrUnknownServer
	}
	s.data.Servers = append(s.data.Servers[:i], s.data.Servers[i+1:]...)
	if s.data.Selected == origin {
		s.data.Selected = ""
		if len(s.data.Servers) > 0 {
			s.data.Selected = s.data.Servers[0].Origin
		}
	}
	return s.writeLocked()
}

func (s *Store) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	b, err := json.MarshalIndent(s.data, "", "\t")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write servers: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace servers: %w", err)
	}
	return nil
}
