package installer

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
)

// Бэкап на этот компьютер (план 45, кадры 8а–8г): вся папка установки
// /opt/wynd — данные, compose.yaml, Caddyfile, сертификат, всё, что человек
// положил туда сам, — кроме копий (backups/). База — снимком: на работающем
// сайте её снимает сам Wynd (`wynd backup -no-blobs`), на остановленном
// копируется как есть. Фото и видео Wynd не меняет: в следующий раз
// докачивается только новое. Новый бэкап собирается рядом (папка.new) и
// заменяет прошлый только целиком; оборванный продолжается с того же места.

// Что делать с сайтом на время бэкапа (8а, три варианта).
const (
	SiteKeep        = "keep"  // работает, пока идёт бэкап
	SitePauseResume = "pause" // остановить и запустить снова
	SiteStopLeave   = "stop"  // остановить и не запускать
)

// BackupManifest — wynd-backup.json в папке бэкапа: что в нём и с каким
// размером (до шифрования), чтобы докачивать только новое.
type BackupManifest struct {
	Format  string           `json:"format"`
	Server  string           `json:"server"`
	Version string           `json:"version"`
	Made    string           `json:"made"`
	Files   map[string]int64 `json:"files"`
	// Encryption — бэкап зашифрован паролем (8а2): соль и стоимость ключа.
	Encryption *BackupKey `json:"encryption,omitempty"`
}

// BackupKey — открытые параметры ключа шифрования бэкапа.
type BackupKey struct {
	Salt string `json:"salt"`
	LogN int    `json:"log_n"`
	// Check — сумма открытого ключа: тот ли пароль, без расшифровки файлов.
	Public string `json:"public"`
}

const (
	manifestFile = "wynd-backup.json"
	backupFormat = "wynd-local-backup"
	snapshotDir  = dataDir + "/.workshop-backup"
	backupLogN   = 15
	encSuffix    = ".age"
)

// RemoteFile — файл папки установки на сервере.
type RemoteFile struct {
	Path string // относительно /opt/wynd
	Size int64
}

// listScript — файлы папки установки, кроме копий и живой базы; база — из снимка.
const listScript = `cd ` + installDir + ` && find . \( -path ./backups -o -path ./data/backups -o -path ./data/.workshop-copy -o -path ./data/.workshop-backup -o -path ./src \) -prune -o -type f ! -path './data/wynd.db*' -printf '%s\t%P\n'
cd ` + snapshotDir + ` && find . -type f -printf '%s\tdata/%P\n'`

func parseListing(out string) []RemoteFile {
	var files []RemoteFile
	for _, line := range strings.Split(out, "\n") {
		size, path, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		if !ok || path == "" {
			continue
		}
		n, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			continue
		}
		files = append(files, RemoteFile{Path: path, Size: n})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// immutable — файлы, которые Wynd пишет один раз: их не перекачиваем.
func immutable(path string) bool { return strings.HasPrefix(path, "data/blobs/") }

// fetchAction — что сделать с файлом.
type fetchAction int

const (
	fetchSkip   fetchAction = iota // уже скачан в этот прогон или прошлый, оборванный
	fetchLink                      // есть в прошлом бэкапе — ссылкой или копией
	fetchResume                    // скачан наполовину — докачать
	fetchAll                       // скачать
)

// planFetch решает по файлу: done — файл скачан целиком (журнал .fetched в
// папке.new), partial — сколько его уже лежит в папке.new, prev — размеры
// прошлого бэкапа той же шифровки. Пропуск и докачка — только у фото и видео:
// база и настройки между попытками могут поменяться (снимок базы повтор
// снимает заново), их кусок от прошлой попытки с ними не сойдётся.
func planFetch(f RemoteFile, encrypted, done bool, partial int64, prev map[string]int64) fetchAction {
	switch {
	case !immutable(f.Path):
		return fetchAll
	case done:
		return fetchSkip
	case partial > 0 && !encrypted && partial < f.Size:
		// Зашифрованный кусок не докачать: шифр пишется заново целиком.
		return fetchResume
	case immutable(f.Path) && prev != nil && prev[f.Path] == f.Size:
		return fetchLink
	}
	return fetchAll
}

// encKey — ключ шифрования бэкапа (8а2).
type encKey struct {
	pub  []byte
	salt []byte
	logN int
}

// LocalBackupSpec — бэкап на компьютер.
type LocalBackupSpec struct {
	Folder   string
	Site     string
	Password string
	// Running — сайт работал; LiveCopy — Wynd умеет снимок без остановки.
	Running  bool
	LiveCopy bool
	Server   string
	Version  string
}

// LocalBackupPlan — строка «Скачаем …» на 8а.
type LocalBackupPlan struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Total   int64  `json:"total"`
	Fetch   int64  `json:"fetch"`
	// Prev — когда сделан прошлый бэкап в этой папке; Encrypted — он с паролем.
	Prev      string `json:"prev,omitempty"`
	Encrypted bool   `json:"encrypted,omitempty"`
	FreeLocal uint64 `json:"free_local"`
}

func readManifest(folder string) (BackupManifest, bool) {
	raw, err := os.ReadFile(filepath.Join(folder, manifestFile))
	if err != nil {
		return BackupManifest{}, false
	}
	var m BackupManifest
	if json.Unmarshal(raw, &m) != nil || m.Format != backupFormat {
		return BackupManifest{}, false
	}
	return m, true
}

// PlanLocalBackup считает, сколько скачать (8а). Список файлов — без снимка
// базы: её размер — у живой базы.
func PlanLocalBackup(ctx context.Context, sess *Session, folder string) LocalBackupPlan {
	res, err := sess.Exec(ctx, listScript+"; du -sk "+dataDir+"/wynd.db* 2>/dev/null | awk '{s+=$1} END {print \"DB\\t\" s*1024}'", nil, shortTimeout)
	if err != nil {
		return LocalBackupPlan{Message: "Связь с сервером оборвалась"}
	}
	plan := LocalBackupPlan{OK: true}
	prev, has := readManifest(folder)
	if has {
		plan.Prev, plan.Encrypted = prev.Made, prev.Encryption != nil
	}
	for _, line := range strings.Split(res.Output, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "DB\t"); ok {
			n, _ := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
			plan.Total += n
			plan.Fetch += n
		}
	}
	for _, f := range parseListing(res.Output) {
		plan.Total += f.Size
		if !(immutable(f.Path) && has && prev.Files[f.Path] == f.Size) {
			plan.Fetch += f.Size
		}
	}
	return plan
}

// LocalBackup делает бэкап. report — ход для окна (шаги 8б).
func LocalBackup(ctx context.Context, sess *Session, spec LocalBackupSpec, report func(Progress)) Progress {
	var key *encKey
	var bkey *BackupKey
	prev, prevHas := readManifest(spec.Folder)
	if spec.Password != "" {
		var salt []byte
		logN := backupLogN
		if prevHas && prev.Encryption != nil {
			// Тот же пароль, что у прошлого бэкапа, — иначе докачка не сойдётся.
			s, err := decodeB64(prev.Encryption.Salt)
			if err != nil {
				return failed("Прошлый бэкап в этой папке повреждён", "Выберите другую папку.")
			}
			salt, logN = s, prev.Encryption.LogN
		} else {
			salt = make([]byte, 16)
			if _, err := rand.Read(salt); err != nil {
				return failed("Не получилось начать шифрование", err.Error())
			}
		}
		priv, err := archive.DeriveKey(spec.Password, salt, logN)
		if err != nil {
			return failed("Не получилось начать шифрование", err.Error())
		}
		key = &encKey{pub: priv.PublicKey().Bytes(), salt: salt, logN: logN}
		bkey = &BackupKey{Salt: encodeB64(salt), LogN: logN, Public: encodeB64(key.pub)}
		if prevHas && prev.Encryption != nil && prev.Encryption.Public != bkey.Public {
			return failed("Пароль не тот, что у прошлого бэкапа в этой папке",
				"Введите прежний пароль — или выберите другую папку: тогда бэкап скачается целиком заново.")
		}
	}
	work := spec.Folder + ".new"
	var fetched, total atomic.Int64
	list := []step{
		{
			id: "snapshot", title: "Копия базы на сервере",
			do: func(ctx context.Context, r *Run) error {
				stop := spec.Running && spec.Site != SiteKeep
				if stop {
					cmd := composeCmd + "stop wynd 2>&1"
					if spec.Site == SiteStopLeave {
						cmd += " && date -u +%Y-%m-%dT%H:%M:%SZ > " + stoppedMark
					}
					if _, err := r.must(ctx, cmd, shortTimeout); err != nil {
						return err
					}
				}
				var snap string
				if spec.Running && !stop {
					if !spec.LiveCopy {
						return &StepError{
							Message: "Wynd " + spec.Version + " не умеет бэкап без остановки",
							Advice:  "Выберите «Остановить на время бэкапа и запустить снова» — или обновите Wynd.",
						}
					}
					snap = "rm -rf " + snapshotDir + " && " + composeCmd + "exec -T wynd wynd backup -no-blobs /data/.workshop-backup </dev/null"
				} else {
					snap = "rm -rf " + snapshotDir + " && mkdir -p " + snapshotDir + " && cp -a " + dataDir + "/wynd.db " + snapshotDir + "/ && for f in wynd.db-wal wynd.db-shm; do if [ -e " + dataDir + "/$f ]; then cp -a " + dataDir + "/$f " + snapshotDir + "/; fi; done"
				}
				_, err := r.must(ctx, "set -e; "+snap, imageTimeout)
				return err
			},
		},
		{
			id: "fetch", title: "Скачиваем",
			do: func(ctx context.Context, r *Run) error {
				out, err := r.must(ctx, listScript, shortTimeout)
				if err != nil {
					return err
				}
				files := parseListing(out)
				for _, f := range files {
					total.Add(f.Size)
				}
				if err := os.MkdirAll(work, 0o700); err != nil {
					return localErr(err)
				}
				done := readFetched(work)
				var prevFiles map[string]int64
				if prevHas && (prev.Encryption != nil) == (key != nil) {
					prevFiles = prev.Files
				}
				for _, f := range files {
					if err := ctx.Err(); err != nil {
						return err
					}
					if err := fetchOne(ctx, r, spec, f, work, key, done[f.Path], prevFiles, &fetched); err != nil {
						return err
					}
					if !done[f.Path] {
						markFetched(work, f.Path)
					}
					r.note(fmt.Sprintf("%s из %s", formatGB(uint64(fetched.Load())), formatGB(uint64(total.Load()))))
				}
				m := BackupManifest{Format: backupFormat, Server: spec.Server, Version: spec.Version,
					Made: time.Now().UTC().Format(time.RFC3339), Files: map[string]int64{}, Encryption: bkey}
				for _, f := range files {
					m.Files[f.Path] = f.Size
				}
				raw, _ := json.MarshalIndent(m, "", " ")
				if err := os.WriteFile(filepath.Join(work, manifestFile), raw, 0o600); err != nil {
					return localErr(err)
				}
				return localErr(os.Remove(filepath.Join(work, fetchedFile)))
			},
			// Кадр 8г; «Прошлый бэкап в папке не тронут…» окно пишет под рамкой.
			explain: func(_ error, output string) *StepError {
				return &StepError{
					Message: "Связь с сервером прервалась",
					Advice:  "Скачано " + fetchedOf(fetched.Load(), total.Load()) + ". Скачанное осталось в папке — повтор продолжит с того же места.",
				}
			},
		},
		{
			id: "check", title: "Проверка скачанного",
			do: func(ctx context.Context, r *Run) error {
				m, ok := readManifest(work)
				if !ok {
					return localErr(errors.New("нет описи бэкапа"))
				}
				for path, size := range m.Files {
					name := filepath.Join(work, filepath.FromSlash(path))
					if key != nil {
						name += encSuffix
					}
					st, err := os.Stat(name)
					if err != nil || key == nil && st.Size() != size {
						return &StepError{Message: "Скачанный файл не сошёлся с сервером", Advice: path + ". Нажмите «Повторить»."}
					}
				}
				// Снимок базы на сервере больше не нужен; сайт — как выбрали.
				cleanup := "rm -rf " + snapshotDir
				if spec.Running && spec.Site == SitePauseResume {
					cleanup += " && " + composeCmd + "start wynd 2>&1"
				}
				if _, err := r.must(ctx, cleanup, shortTimeout); err != nil {
					return err
				}
				return localErr(swapFolders(spec.Folder, work))
			},
		},
	}
	p := runSteps(ctx, sess, Spec{}, list, report, nil)
	if p.Failure != nil && spec.Running && spec.Site == SitePauseResume {
		// «Остановить на время бэкапа и запустить снова»: бэкап не вышел — сайт
		// всё равно запускаем, а не оставляем стоять до повтора. Связи нет —
		// страница сервера скажет «не работает», там есть «Запустить».
		_, _ = sess.Exec(ctx, composeCmd+"start wynd 2>&1", nil, shortTimeout)
	}
	return p
}

// fetchedOf — «56 из 140 МБ» (кадр 8г).
func fetchedOf(done, total int64) string {
	if total < 1<<30 {
		return fmt.Sprintf("%d из %d МБ", done>>20, total>>20)
	}
	return formatGB(uint64(done)) + " из " + formatGB(uint64(total))
}

func failed(msg, advice string) Progress {
	return Progress{Failure: &Failure{Message: msg, Advice: advice}}
}

// localErr — отказ на этом компьютере (диск, права): словами человека.
func localErr(err error) error {
	if err == nil {
		return nil
	}
	return &StepError{Message: "Не получилось записать бэкап на этот компьютер", Advice: err.Error()}
}

func fetchOne(ctx context.Context, r *Run, spec LocalBackupSpec, f RemoteFile, work string, key *encKey,
	done bool, prev map[string]int64, fetched *atomic.Int64) error {

	local := filepath.Join(work, filepath.FromSlash(f.Path))
	if key != nil {
		local += encSuffix
	}
	var partial int64
	if st, err := os.Stat(local); err == nil {
		partial = st.Size()
	}
	action := planFetch(f, key != nil, done, partial, prev)
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		return localErr(err)
	}
	switch action {
	case fetchSkip:
		fetched.Add(f.Size)
		return nil
	case fetchLink:
		old := filepath.Join(spec.Folder, filepath.FromSlash(f.Path))
		if key != nil {
			old += encSuffix
		}
		_ = os.Remove(local)
		if err := os.Link(old, local); err == nil || copyLocal(old, local) == nil {
			fetched.Add(f.Size)
			return nil
		}
	}
	remote := shQuote(installDir + "/" + f.Path)
	if f.Path == "data/wynd.db" || strings.HasPrefix(f.Path, "data/wynd.db-") {
		remote = shQuote(snapshotDir + "/" + strings.TrimPrefix(f.Path, "data/"))
	}
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	cmd := "cat " + remote
	if action == fetchResume {
		flag = os.O_WRONLY | os.O_APPEND
		cmd = "tail -c +" + strconv.FormatInt(partial+1, 10) + " " + remote
		fetched.Add(partial)
	}
	out, err := os.OpenFile(local, flag, 0o600)
	if err != nil {
		return localErr(err)
	}
	var w io.Writer = out
	var enc io.WriteCloser
	if key != nil {
		enc, err = archive.Encrypt(out, archive.KeyParams{PublicKey: key.pub, Salt: key.salt, LogN: key.logN})
		if err != nil {
			out.Close()
			return localErr(err)
		}
		w = enc
	}
	r.record(cmd, "")
	err = r.sess.Download(ctx, cmd, &countWriter{w: w, n: fetched})
	if enc != nil && err == nil {
		err = enc.Close()
	}
	if cerr := out.Close(); err == nil && cerr != nil {
		err = localErr(cerr)
	}
	return err
}

// Журнал скачанного целиком в папке.new: оборванный прогон их не качает.
const fetchedFile = ".fetched"

func readFetched(work string) map[string]bool {
	raw, _ := os.ReadFile(filepath.Join(work, fetchedFile))
	done := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			done[line] = true
		}
	}
	return done
}

func markFetched(work, path string) {
	f, err := os.OpenFile(filepath.Join(work, fetchedFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(path + "\n")
	_ = f.Close()
}

func encodeB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func decodeB64(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

type countWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))
	return n, err
}

func copyLocal(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// swapFolders ставит новый бэкап на место прошлого: прошлый — в .old,
// новый — на его имя, .old удаляется.
func swapFolders(folder, work string) error {
	old := folder + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(folder); err == nil {
		if err := os.Rename(folder, old); err != nil {
			return err
		}
	}
	if err := os.Rename(work, folder); err != nil {
		_ = os.Rename(old, folder)
		return err
	}
	return os.RemoveAll(old)
}
