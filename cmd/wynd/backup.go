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
	requireDatabase("backup", cfg.DataDir)
	if err := backup.Backup(cfg.DataDir, dest, *incremental); err != nil {
		log.Fatalf("backup: %v", err)
	}
	log.Printf("backup written to %s", dest)
}

// requireDatabase останавливает команду, если в каталоге данных нет базы.
// Пустая база — почти наверняка неверный WYND_DATA_DIR: раньше такой запуск
// создавал пустой wynd.db и «успешно» с ним работал (STB-4).
func requireDatabase(cmd, dataDir string) string {
	dbPath := filepath.Join(dataDir, "wynd.db")
	if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
		log.Fatalf("%s: в %s нет базы (%s): проверьте WYND_DATA_DIR", cmd, dataDir, dbPath)
	}
	return dbPath
}
