package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPICoversMuxRoutes(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	dir := filepath.Dir(thisFile)

	mux := muxAPIOps(t, filepath.Join(dir, "server.go"))
	spec := map[string]struct{}{}
	mergeOps(spec, openAPIOps(t, filepath.Join(dir, "openapi-participant.yaml")))
	mergeOps(spec, openAPIOps(t, filepath.Join(dir, "openapi-admin.yaml")))

	var missing []string
	for op := range mux {
		if _, ok := spec[op]; !ok {
			missing = append(missing, op)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("OpenAPI missing mux routes: %s", strings.Join(missing, ", "))
	}
	var extra []string
	for op := range spec {
		if _, ok := mux[op]; !ok {
			extra = append(extra, op)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("OpenAPI extra (not on mux): %s", strings.Join(extra, ", "))
	}
}

var handleFuncRe = regexp.MustCompile(`HandleFunc\("(GET|POST|PUT|PATCH|DELETE|HEAD) (/api/v1/[^"]+)"`)

func muxAPIOps(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]struct{}{}
	for _, m := range handleFuncRe.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]+" "+stripAPIv1(m[2])] = struct{}{}
	}
	if len(out) == 0 {
		t.Fatal("no mux routes found")
	}
	return out
}

func stripAPIv1(p string) string {
	return strings.TrimPrefix(p, "/api/v1")
}

var pathRe = regexp.MustCompile(`^  (/[-a-zA-Z0-9_{}/]+):`)
var methodRe = regexp.MustCompile(`^    (get|post|put|patch|delete|head):`)

func openAPIOps(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]struct{}{}
	var current string
	inPaths := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if strings.HasPrefix(line, "components:") {
			break
		}
		if m := pathRe.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if current == "" {
			continue
		}
		if m := methodRe.FindStringSubmatch(line); m != nil {
			out[strings.ToUpper(m[1])+" "+current] = struct{}{}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no OpenAPI ops in %s", path)
	}
	return out
}

func mergeOps(dst, src map[string]struct{}) {
	for k, v := range src {
		dst[k] = v
	}
}
