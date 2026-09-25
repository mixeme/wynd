package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gitea.mixdep.ru/mix/wynd/internal/backup"
	"gitea.mixdep.ru/mix/wynd/internal/config"
)

func runBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	incremental := fs.Bool("incremental", false, "copy only new or changed blobs")
	_ = fs.Parse(args)

	dest := fs.Arg(0)
	if dest == "" {
		fmt.Fprintln(os.Stderr, "usage: wynd backup [-incremental] <destination-dir>")
		os.Exit(2)
	}
	dest, err := filepath.Abs(dest)
	if err != nil {
		log.Fatalf("dest: %v", err)
	}
	cfg := loadExistingConfig("backup")
	if err := backup.Backup(cfg.DataDir, dest, *incremental); err != nil {
		log.Fatalf("backup: %v", err)
	}
	log.Printf("backup written to %s", dest)
}

// loadExistingConfig читает настройки служебной команды, ничего не создавая
// на диске, и останавливает её, если в каталоге данных нет базы. Пустой или
// отсутствующий wynd.db — почти наверняка опечатка в WYND_DATA_DIR: раньше
// команда оставляла после себя каталог с blobs/, keys/ и пустым wynd.db
// (STB-4, найдено при проверке `wynd admin-password`).
func loadExistingConfig(cmd string) *config.Config {
	cfg, err := config.LoadExisting()
	if errors.Is(err, config.ErrNoDatabase) {
		log.Fatalf("%s: %v — проверьте WYND_DATA_DIR", cmd, err)
	}
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	return cfg
}
