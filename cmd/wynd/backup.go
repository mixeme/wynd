package main

import (
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

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	dest := fs.Arg(0)
	if dest == "" {
		fmt.Fprintln(os.Stderr, "usage: wynd backup [-incremental] <destination-dir>")
		os.Exit(2)
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		log.Fatalf("dest: %v", err)
	}
	// Пустая база — почти наверняка неверный WYND_DATA_DIR. Раньше такой
	// запуск создавал пустой wynd.db и «успешно» его бэкапил (STB-4).
	dbPath := filepath.Join(cfg.DataDir, "wynd.db")
	if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
		log.Fatalf("backup: в %s нет базы (%s): проверьте WYND_DATA_DIR", cfg.DataDir, dbPath)
	}
	if err := backup.Backup(cfg.DataDir, dest, *incremental); err != nil {
		log.Fatalf("backup: %v", err)
	}
	log.Printf("backup written to %s", dest)
}
