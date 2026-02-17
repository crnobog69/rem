# rem

`rem` је приватни build алат у Go-у, са `Remfile` форматом у TOML-у.
Практична алтернатива за `make`/`cmake`, са фокусом на брз локални рад.

## Циљ

- Једноставан CLI (`rem build`, `rem run`, `rem list`, `rem graph`)
- Сопствени build формат (`Remfile` у TOML-у)
- Променљиве за поновну употребу (`[vars]`)
- Извршавање dependency графа са паралелизмом
- Up-to-date провера преко `inputs`/`outputs`
- Ажурирање преко GitHub Releases

## Инсталација из сорса

```bash
go build -o rem ./cmd/rem
./rem version
```

## `Remfile` синтакса (TOML)

```text
default = "build"

[vars]
APP_NAME = "rem"
VERSION = "dev"

[task.gen]
desc = "Генерисање фајлова"
cmds = ["go generate ./..."]

[task.build]
desc = "Компилација бинарног фајла"
deps = ["gen"]
inputs = ["cmd/rem/main.go", "internal/*/*.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/rem",
]
```

Правила:

- Root кључ: `default = "task_name"`
- Табела променљивих: `[vars]` са `NAME = "value"`
- Task табеле: `[task.<name>]`
- Поља task-а: `desc`, `deps`, `inputs`, `outputs`, `dir`, `cmds`
- Опционо `cmd` и даље ради као алијас за једну команду
- Подржана је експанзија `${VAR}` и `${VAR:-fallback}`

## Команде

```bash
rem init
rem doctor
rem list -D VERSION=v0.1.0
rem graph -D APP_NAME=rem
rem format
rem format --check
rem run build
rem run -D VERSION=v1.0.0 production
rem run -D RELEASE_VERSION=v1.0.0 release
rem build
rem build -j 8
```

`rem format` уписује канонски TOML формат и не чува коментаре.
`rem init` креира `Remfile`, `REM.md` и `REM.sr-Cyrl.md`.
CLI излаз користи боје на TTY; искључивање: `NO_COLOR=1`.

## VS Code екстензија

Почетна екстензија за `Remfile` је у:

- `vscode/remfile`

Садржи language registration, syntax highlight и snippet-е.
Додаје и лаку дијагностику за Remfile TOML фајлове.
Упутство је у `vscode/remfile/README.md`.

## Пример миграције са Makefile-а

Превод Makefile-а који си послао је у:

- `examples/Remfile.gitcrn`

## Провера нове верзије (GitHub Releases)

- Искључивање: `REM_NO_UPDATE_CHECK=1`
- Репо преко env: `REM_UPDATE_REPO=owner/repo`
- Или compile-time:

```bash
go build -ldflags "-X main.version=v0.1.0 -X main.updateRepo=owner/repo" -o rem ./cmd/rem
```

## Release

```bash
./scripts/release.sh --version v0.1.0
```

Генерише:

- `dist/rem-linux-amd64`
- `dist/rem-linux-arm64`
- `dist/rem-windows-amd64.exe`
- `dist/rem-windows-arm64.exe`
- `dist/checksums.txt`
