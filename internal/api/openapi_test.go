package api_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Инвариант: каждый маршрут описан в своей спеке и наоборот.
// Спеки две и делятся по префиксу /admin/: общая сверка пропускала
// админский маршрут, описанный в участниковой спеке, и наоборот.
func TestOpenAPICoversMuxRoutes(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	dir := filepath.Dir(thisFile)

	mux := muxAPIOps(t, filepath.Join(dir, "server.go"))
	muxAdmin, muxParticipant := splitAdminOps(mux)

	for _, pair := range []struct {
		name string
		mux  map[string]struct{}
		spec map[string]struct{}
	}{
		{"admin", muxAdmin, openAPIOps(t, filepath.Join(dir, "openapi-admin.yaml"))},
		{"participant", muxParticipant, openAPIOps(t, filepath.Join(dir, "openapi-participant.yaml"))},
	} {
		var missing []string
		for op := range pair.mux {
			if _, ok := pair.spec[op]; !ok {
				missing = append(missing, op)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Fatalf("openapi-%s.yaml без маршрутов mux: %s", pair.name, strings.Join(missing, ", "))
		}
		var extra []string
		for op := range pair.spec {
			if _, ok := pair.mux[op]; !ok {
				extra = append(extra, op)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Fatalf("openapi-%s.yaml описывает чужое: %s", pair.name, strings.Join(extra, ", "))
		}
	}
}

// Инвариант: сверка видит все регистрации. Разбор идёт регулярным
// выражением по исходнику, и маршрут, записанный иначе, тихо выпадал бы из проверки.
func TestMuxRouteCountMatchesParsedOps(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "server.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	registrations := strings.Count(string(raw), "s.Mux.Handle")
	parsed := len(muxAPIOps(t, path))
	if registrations != parsed {
		t.Fatalf("регистраций s.Mux.Handle* — %d, распознано маршрутов — %d", registrations, parsed)
	}
}

// splitAdminOps делит маршруты по префиксу /admin/.
func splitAdminOps(ops map[string]struct{}) (admin, participant map[string]struct{}) {
	admin = map[string]struct{}{}
	participant = map[string]struct{}{}
	for op := range ops {
		_, path, _ := strings.Cut(op, " ")
		if strings.HasPrefix(path, "/admin/") || path == "/admin" {
			admin[op] = struct{}{}
			continue
		}
		participant[op] = struct{}{}
	}
	return admin, participant
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
