# Сборка и прогон на Unix. На Windows те же действия — scripts\*.bat
# (build.bat, test.bat, run.bat); .bat остаются обёртками, а не вторым
# источником правды (план 42, волна 3).

GO ?= go
NPM ?= npm
BIN ?= dist/wynd
DATA_DIR ?= dev/data

.PHONY: build web test run docker licenses clean

## build — клиент (vite) и бинарь с вшитым клиентом
build: web
	$(GO) build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/wynd

web: web/node_modules
	cd web && $(NPM) run build

web/node_modules: web/package-lock.json
	cd web && $(NPM) ci
	@touch web/node_modules

## test — ворота релиза: то же, что scripts/test.bat и .gitea/workflows/ci.yaml.
## Порядок важен: web/dist вшивается через go:embed, без него падает go vet.
test: web
	cd web && $(NPM) run check
	cd web && $(NPM) run check:ui
	cd web && $(NPM) run test
	$(GO) vet ./...
	$(GO) test ./...
	$(GO) build -trimpath -o /dev/null ./cmd/wynd

## run — локальный запуск на данных из dev/data
run: build
	WYND_DATA_DIR=$(DATA_DIR) ./$(BIN)

## licenses — пересобрать web/static/THIRD_PARTY_LICENSES.txt после смены
## зависимостей; свежесть файла проверяет тест src/test/licenses.test.ts
licenses: web/node_modules
	cd web && $(NPM) run licenses

## docker — тот же образ, что собирает CI
docker:
	docker build -f deploy/docker/Dockerfile \
		--build-arg VERSION=$$(cat VERSION) -t wynd:$$(cat VERSION) .

clean:
	rm -rf dist web/dist web/.svelte-kit
