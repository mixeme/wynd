package archive_test

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func testKey(t *testing.T, password string) archive.KeyParams {
	t.Helper()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatal(err)
	}
	// Меньше повторов, чем у устройства: проверке хватает нижней границы.
	const iterations = 100_000
	priv, err := archive.DeriveKey(password, salt, iterations)
	if err != nil {
		t.Fatal(err)
	}
	return archive.KeyParams{PublicKey: priv.PublicKey().Bytes(), Salt: salt, Iterations: iterations}
}

// Инвариант (план 51): файл, зашифрованный на открытый ключ, открывается
// паролем, из которого ключ выведен, и не открывается другим; длина
// зашифрованного файла известна заранее — на границах кусков age тоже.
func TestEncryptRoundTripAndSize(t *testing.T) {
	k := testKey(t, "дача-самовар-облако-17")
	for _, n := range []int{0, 1, 65535, 65536, 65537, 131072, 200000} {
		plain := make([]byte, n)
		_, _ = rand.Read(plain)
		var buf bytes.Buffer
		w, err := archive.Encrypt(&buf, k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(plain); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		want, err := archive.EncryptedSize(int64(n), k)
		if err != nil {
			t.Fatal(err)
		}
		if int64(buf.Len()) != want {
			t.Fatalf("%d байт: EncryptedSize = %d, а файл — %d", n, want, buf.Len())
		}
		if !strings.HasPrefix(buf.String(), archive.EncryptedMagic+"\n{") {
			t.Fatalf("%d байт: нет заголовка wynd: %q", n, buf.String()[:40])
		}
		r, err := archive.Decrypt(bytes.NewReader(buf.Bytes()), "дача-самовар-облако-17")
		if err != nil {
			t.Fatalf("%d байт: расшифровка: %v", n, err)
		}
		got, err := io.ReadAll(r)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("%d байт: расшифровалось не то (%v)", n, err)
		}
		if n == 1 {
			if r, err := archive.Decrypt(bytes.NewReader(buf.Bytes()), "чужой пароль"); err == nil {
				if _, err := io.ReadAll(r); err == nil {
					t.Fatal("файл открылся чужим паролем")
				}
			}
		}
	}
}

// Ключ, который не точка кривой, или короткая соль — отказ, а не файл, который
// никто не откроет.
func TestEncryptRejectsBadKey(t *testing.T) {
	k := testKey(t, "x")
	bad := k
	bad.Salt = []byte{1, 2, 3}
	if _, err := archive.Encrypt(io.Discard, bad); err != archive.ErrBadKey {
		t.Fatalf("короткая соль: %v", err)
	}
	bad = k
	bad.Iterations = 1000
	if _, err := archive.Encrypt(io.Discard, bad); err != archive.ErrBadKey {
		t.Fatalf("мало повторов: %v", err)
	}
	bad = k
	bad.PublicKey = make([]byte, 31)
	if _, err := archive.Encrypt(io.Discard, bad); err != archive.ErrBadKey {
		t.Fatalf("короткий ключ: %v", err)
	}
}

func fullInput(t *testing.T) archive.FullInput {
	t.Helper()
	blobs, cleanup := openBlobStore(t)
	t.Cleanup(cleanup)
	snap := time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC)
	data := &chronicle.FullArchiveData{
		CircleID: "c1", CircleName: "Семья", Color: "ochre", SnapshotAt: snap, Posts: 1,
		Tables: []chronicle.FullArchiveTable{
			{Name: "circles", Columns: []string{"id", "name"}, Rows: [][]any{{"c1", "Семья"}}},
			{Name: "posts", Columns: []string{"id", "body"}, Rows: [][]any{{"p1", strings.Repeat("на даче ", 200)}}},
			{Name: "reactions", Columns: []string{"id"}},
			{Name: "accounts", Columns: []string{"id", "email"}, Rows: [][]any{{"acc1", "anya@example.com"}}},
		},
	}
	return archive.FullInput{
		Manifest: archive.FullManifest{WyndVersion: "9.9.9", SchemaVersion: 34, Server: "https://home.example.org"},
		Data:     data,
		Readable: archive.BuildInput{
			CircleName: "Семья", CutoffDate: "2026-10-15", Blobs: blobs,
			Posts: []chronicle.FeedPost{{Post: chronicle.Post{
				ID: "p1", AuthorName: "Аня", Body: strings.Repeat("на даче ", 200),
				EntryDate: "2026-10-14", CreatedAt: snap.Add(-time.Hour),
			}}},
		},
	}
}

// Инвариант (план 51): в полном архиве — машинная часть (manifest и строки
// таблиц, пустая таблица — пустым списком) и читаемая лента «на дату»;
// FullSize заранее даёт точную длину.
func TestBuildFullArchive(t *testing.T) {
	in := fullInput(t)
	size, err := archive.FullSize(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := archive.BuildFullArchive(t.Context(), &buf, in); err != nil {
		t.Fatal(err)
	}
	if size != int64(buf.Len()) {
		t.Fatalf("FullSize = %d, а архив — %d байт", size, buf.Len())
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) []byte {
		t.Helper()
		f, err := zr.Open(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		return b
	}
	var m archive.FullManifest
	if err := json.Unmarshal(read("manifest.json"), &m); err != nil {
		t.Fatal(err)
	}
	if m.Format != archive.FullFormat || m.FormatVersion != 1 || m.SchemaVersion != 34 || m.CircleID != "c1" ||
		m.Posts != 1 || len(m.Tables) != 4 || !m.SnapshotAt.Equal(in.Data.SnapshotAt) {
		t.Fatalf("manifest: %+v", m)
	}
	var reactions struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	}
	if err := json.Unmarshal(read("data/reactions.json"), &reactions); err != nil || reactions.Rows == nil {
		t.Fatalf("пустая таблица: %s (%v)", read("data/reactions.json"), err)
	}
	if !strings.Contains(string(read("data/accounts.json")), "anya@example.com") {
		t.Fatal("в архиве нет почты участника")
	}
	index := string(read("index.html"))
	if !strings.Contains(index, "Полный архив круга на 15 октября 2026") || !strings.Contains(index, "полный архив круга") {
		t.Fatalf("шапка ленты: %.300s", index)
	}
}

// Зашифрованный полный архив: внутри тот же ZIP.
func TestFullArchiveEncrypted(t *testing.T) {
	in := fullInput(t)
	k := testKey(t, "пароль")
	plain, err := archive.FullSize(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	want, err := archive.EncryptedSize(plain, k)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w, err := archive.Encrypt(&buf, k)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.BuildFullArchive(t.Context(), w, in); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if int64(buf.Len()) != want {
		t.Fatalf("ждали %d байт, вышло %d", want, buf.Len())
	}
	r, err := archive.Decrypt(&buf, "пароль")
	if err != nil {
		t.Fatal(err)
	}
	z, _ := io.ReadAll(r)
	if _, err := zip.NewReader(bytes.NewReader(z), int64(len(z))); err != nil {
		t.Fatalf("внутри не ZIP: %v", err)
	}
}
