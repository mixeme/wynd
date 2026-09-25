package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/version"
)

func TestBinaryEmptyDataDir(t *testing.T) {
	dataDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	bin := filepath.Join(t.TempDir(), "wynd-test.exe")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = packageDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(),
		"WYND_DATA_DIR="+dataDir,
		"WYND_LISTEN="+addr,
		"WYND_PUBLIC_URL=http://"+addr,
	)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	logCh := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(stderr)
		logCh <- string(b)
	}()

	if err := waitHealth(ctx, addr); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("health: %v\nlogs:\n%s", err, <-logCh)
	}

	instReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/v1/instance", nil)
	if err != nil {
		t.Fatal(err)
	}
	instResp, err := http.DefaultClient.Do(instReq)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("instance: %v\nlogs:\n%s", err, <-logCh)
	}
	if instResp.StatusCode != http.StatusOK {
		_ = instResp.Body.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("instance status: %d\nlogs:\n%s", instResp.StatusCode, <-logCh)
	}
	var inst map[string]any
	if err := json.NewDecoder(instResp.Body).Decode(&inst); err != nil {
		_ = instResp.Body.Close()
		t.Fatal(err)
	}
	_ = instResp.Body.Close()
	if inst["version"] != version.String() {
		t.Fatalf("instance version: %v want %s", inst["version"], version.String())
	}
	if inst["version"] == "0.0.0-dev" {
		t.Fatal("empty-machine binary reported 0.0.0-dev")
	}

	_ = cmd.Process.Kill()
	logs := <-logCh

	if !strings.Contains(logs, "bootstrap URL:") {
		t.Fatalf("log missing bootstrap URL:\n%s", logs)
	}
	if !strings.Contains(logs, "data dir:") {
		t.Fatalf("log missing data dir:\n%s", logs)
	}

	// config.json не создаётся чтением конфигурации (API-5).
	for _, name := range []string{"wynd.db", "blobs", "keys"} {
		p := filepath.Join(dataDir, name)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("data layout %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "keys", "bootstrap")); err != nil {
		t.Fatalf("bootstrap token file: %v", err)
	}
}

func waitHealth(ctx context.Context, addr string) error {
	deadline := time.Now().Add(10 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			last = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		last = fmt.Errorf("status %d", resp.StatusCode)
		time.Sleep(50 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("timeout waiting for health")
	}
	return last
}

func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Dir(file)
}
