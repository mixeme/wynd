package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/store"
)

// runAdminPassword — `wynd admin-password`: новый пароль панели со стандартного
// ввода, без старого. Забывший пароль панели держит сервер — ему и менять;
// открытые сессии панели закрываются (план 42, BKP-4). Сервер можно не
// останавливать.
func runAdminPassword(args []string) {
	fs := flag.NewFlagSet("admin-password", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: wynd admin-password < file-with-password")
		fmt.Fprintln(os.Stderr, "Читает новый пароль панели первой строкой стандартного ввода.")
	}
	_ = fs.Parse(args)

	cfg := loadExistingConfig("admin-password")
	dbPath := filepath.Join(cfg.DataDir, "wynd.db")

	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Новый пароль панели (ввод виден на экране): ")
	}
	password, err := readPasswordLine(os.Stdin)
	if err != nil {
		log.Fatalf("admin-password: %v", err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()
	ch, err := chronicle.New(st)
	if err != nil {
		log.Fatalf("chronicle: %v", err)
	}
	authSvc, err := auth.New(st, ch, nil, false)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	err = authSvc.SetAdminPassword(context.Background(), password, time.Now().UTC())
	switch {
	case errors.Is(err, auth.ErrWeakPassword):
		log.Fatalf("admin-password: пароль короче %d знаков", auth.MinPasswordLen)
	case errors.Is(err, auth.ErrNotFound):
		log.Fatalf("admin-password: инстанс ещё не установлен — задайте пароль на странице первого запуска")
	case err != nil:
		log.Fatalf("admin-password: %v", err)
	}
	log.Print("пароль панели изменён, открытые сессии панели закрыты")
}

// readPasswordLine берёт первую строку ввода без перевода строки и без
// пробелов по краям — так же, как её обрезало бы поле формы.
func readPasswordLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("пустой пароль")
	}
	return line, nil
}
