package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Копии архивов по расписанию (план 66, функция 7). Собирает сама оболочка,
// пока приложение запущено — окно может быть закрыто в значок: интерфейс для
// сборки не нужен. Архив — тот же, что отдаётся на «Скачать архив»
// (участнический, с докачкой); файл кладётся в выбранную папку, старые копии
// удаляются по настройке хранения — только свои (по имени вида «круг ГГГГ-ММ-ДД.zip»).

// BackupSchedule — как часто собирать копию.
type BackupSchedule string

const (
	ScheduleDaily   BackupSchedule = "daily"
	ScheduleWeekly  BackupSchedule = "weekly"
	ScheduleMonthly BackupSchedule = "monthly"
	ScheduleCustom  BackupSchedule = "custom"
)

// KeepMode — как хранить копии: число последних или срок в днях.
type KeepMode string

const (
	KeepCount KeepMode = "count"
	KeepDays  KeepMode = "days"
)

// BackupConfig — настройки копий (backups.json в папке окна).
type BackupConfig struct {
	Enabled     bool           `json:"enabled"`
	Folder      string         `json:"folder,omitempty"`
	Schedule    BackupSchedule `json:"schedule"`
	CustomDays  int            `json:"custom_days,omitempty"`
	KeepMode    KeepMode       `json:"keep_mode"`
	Keep        int            `json:"keep"`
	LastAt      string         `json:"last_at,omitempty"`      // RFC3339
	LastCircles int            `json:"last_circles,omitempty"` // сколько копий собралось в последний раз
}

func DefaultBackupConfig() BackupConfig {
	return BackupConfig{Schedule: ScheduleDaily, KeepMode: KeepCount, Keep: 10}
}

// NormalizeBackupConfig проверяет значения формы: неизвестное расписание или
// пустая папка при включённых копиях — отказ, а не молчаливая подмена.
func NormalizeBackupConfig(cfg BackupConfig) (BackupConfig, error) {
	switch cfg.Schedule {
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthly, ScheduleCustom:
	default:
		return BackupConfig{}, errors.New("backups: неизвестное расписание")
	}
	if cfg.Schedule == ScheduleCustom && (cfg.CustomDays < 1 || cfg.CustomDays > 365) {
		return BackupConfig{}, errors.New("backups: свой интервал — от 1 до 365 дней")
	}
	limit := 1000
	if cfg.KeepMode == KeepDays {
		limit = 3650
	} else if cfg.KeepMode != KeepCount {
		return BackupConfig{}, errors.New("backups: неизвестный способ хранения")
	}
	if cfg.Keep < 1 || cfg.Keep > limit {
		return BackupConfig{}, fmt.Errorf("backups: хранить — от 1 до %d", limit)
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Folder) == "" {
		return BackupConfig{}, errors.New("backups: выберите папку")
	}
	cfg.Folder = strings.TrimSpace(cfg.Folder)
	return cfg, nil
}

// backupInterval — через сколько после прошлой копии собирать следующую.
// Месяц — календарный, а не 30 дней: «каждый месяц» значит тот же день месяца.
func backupInterval(cfg BackupConfig) func(time.Time) time.Time {
	switch cfg.Schedule {
	case ScheduleWeekly:
		return func(last time.Time) time.Time { return last.AddDate(0, 0, 7) }
	case ScheduleMonthly:
		return func(last time.Time) time.Time { return last.AddDate(0, 1, 0) }
	case ScheduleCustom:
		return func(last time.Time) time.Time { return last.AddDate(0, 0, cfg.CustomDays) }
	default:
		return func(last time.Time) time.Time { return last.AddDate(0, 0, 1) }
	}
}

// BackupDue — пора ли собирать: включено и с прошлой копии прошёл срок.
// Первая копия после включения — сразу.
func BackupDue(lastAt time.Time, cfg BackupConfig, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	if lastAt.IsZero() {
		return true
	}
	return !now.Before(backupInterval(cfg)(lastAt))
}

// backupFilePattern — свои файлы копий: «<круг> ГГГГ-ММ-ДД.zip» (возможен
// суффикс « (2)» при повторной копии в тот же день).
var backupFilePattern = regexp.MustCompile(`^(.+) (\d{4}-\d{2}-\d{2})(?: \((\d+)\))?\.zip$`)

// backupFileName — имя файла копии; запрещённые в именах Windows символы
// заменяются, пустое имя круга — «круг». Существующее имя не перезаписывается:
// повторная копия в тот же день получает « (2)», « (3)».
func backupFileName(folder, circleName string, day time.Time) string {
	base := sanitizeBackupName(circleName)
	if base == "" {
		base = "круг"
	}
	name := fmt.Sprintf("%s %s.zip", base, day.Format("2006-01-02"))
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(folder, name)); errors.Is(err, os.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("%s %s (%d).zip", base, day.Format("2006-01-02"), i)
	}
}

// sanitizeBackupName убирает из имени круга то, что нельзя в имени файла.
func sanitizeBackupName(name string) string {
	replacer := strings.NewReplacer(
		`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_",
		`"`, "_", "<", "_", ">", "_", "|", "_",
	)
	clean := strings.TrimSpace(replacer.Replace(name))
	return strings.Trim(clean, " .")
}

// pruneBackups удаляет лишние копии в папке: count — оставить keep новейших
// на каждый круг, days — удалить старше keep дней. Трогает только свои файлы
// (совпали с шаблоном имени); возвращает удалённые имена.
func pruneBackups(folder string, cfg BackupConfig, now time.Time) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type backupFile struct {
		name string
		when time.Time
	}
	groups := make(map[string][]backupFile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := backupFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		when, err := time.Parse("2006-01-02", match[2])
		if err != nil {
			continue
		}
		groups[match[1]] = append(groups[match[1]], backupFile{name: entry.Name(), when: when})
	}
	var removed []string
	for _, files := range groups {
		sort.Slice(files, func(i, j int) bool {
			if files[i].when.Equal(files[j].when) {
				return files[i].name > files[j].name // « (2)» новее « (1)»
			}
			return files[i].when.After(files[j].when)
		})
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		for i, file := range files {
			old := cfg.KeepMode == KeepCount && i >= cfg.Keep
			if cfg.KeepMode == KeepDays {
				// «Не старше N дней»: копия за сегодня и за N дней назад остаётся.
				old = file.when.Before(today.AddDate(0, 0, -cfg.Keep))
			}
			if !old {
				continue
			}
			if err := os.Remove(filepath.Join(folder, file.name)); err == nil {
				removed = append(removed, file.name)
			}
		}
	}
	return removed, nil
}

// backupCircle — круг для копии: идентификатор и имя для файла.
type backupCircle struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// fetchBackupCircles — круги учётки на сервере (GET /circles).
func fetchBackupCircles(ctx context.Context, client *http.Client, origin, token string) ([]backupCircle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/v1/circles", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backups: circles: HTTP %d", res.StatusCode)
	}
	var body struct {
		Circles []backupCircle `json:"circles"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	return body.Circles, nil
}

// fetchBackupArchive — архив участника потоком (GET /archive/download).
func fetchBackupArchive(ctx context.Context, client *http.Client, origin, token, circleID string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/v1/circles/"+circleID+"/archive/download", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("backups: архив %s: HTTP %d", circleID, res.StatusCode)
	}
	return res, nil
}

// RunBackupOnce собирает свежие копии всех кругов учётки в папку и применяет
// хранение. Возвращает число собранных копий; сбой одного круга не отменяет
// остальные, но попадает в журнал ошибкой.
func RunBackupOnce(ctx context.Context, client *http.Client, origin, token, folder string, cfg BackupConfig, now time.Time) (int, error) {
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return 0, err
	}
	circles, err := fetchBackupCircles(ctx, client, origin, token)
	if err != nil {
		return 0, err
	}
	saved := 0
	var firstErr error
	for _, circle := range circles {
		res, err := fetchBackupArchive(ctx, client, origin, token, circle.ID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		name := backupFileName(folder, circle.Name, now)
		tmp := filepath.Join(folder, name+".part")
		file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = res.Body.Close()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		_, copyErr := io.Copy(file, res.Body)
		closeErr := file.Close()
		_ = res.Body.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(tmp)
			if firstErr == nil {
				firstErr = copyErr
			}
			continue
		}
		if err := os.Rename(tmp, filepath.Join(folder, name)); err != nil {
			_ = os.Remove(tmp)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		saved++
	}
	if _, err := pruneBackups(folder, cfg, now); err != nil && firstErr == nil {
		firstErr = err
	}
	return saved, firstErr
}
