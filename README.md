# rem
Language: English | [Српски (ћирилица)](README.sr-Cyrl.md)

`rem` is a private build tool in Go, with a `Remfile` build format in TOML.
It is designed as a practical alternative to `make`/`cmake` for fast local workflows.

## Goals

- Simple CLI (`rem build`, `rem run`, `rem list`, `rem graph`)
- Own build file (`Remfile` in TOML)
- Reusable variables (`[vars]`)
- Dependency graph execution with parallel jobs
- Up-to-date checks via `inputs`/`outputs`
- Release/update workflow via GitHub Releases

## Install (GitHub Releases)

Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/crnobog69/rem/master/scripts/install.sh | bash
```

Windows (PowerShell):

```powershell
iwr https://raw.githubusercontent.com/crnobog69/rem/master/scripts/install.ps1 -UseBasicParsing | iex
```

The installer adds `%USERPROFILE%\\bin` to user `PATH` automatically.
If `rem` is still not recognized, open a new terminal.

Optional override repo (for forks/private builds):

```bash
REM_UPDATE_REPO=owner/repo curl -fsSL https://raw.githubusercontent.com/crnobog69/rem/master/scripts/install.sh | bash
```

```powershell
$env:REM_UPDATE_REPO = "owner/repo"; iwr https://raw.githubusercontent.com/crnobog69/rem/master/scripts/install.ps1 -UseBasicParsing | iex
```

Installed paths:

- Linux: `~/.local/bin/rem`
- Windows: `%USERPROFILE%\\bin\\rem.exe`

Note: current release assets are published for Linux and Windows.

## Install from source

```bash
go build -o rem ./cmd/rem
./rem version
```

## Update rem

Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/crnobog69/rem/master/scripts/update.sh | bash
```

Windows (PowerShell):

```powershell
iwr https://raw.githubusercontent.com/crnobog69/rem/master/scripts/update.ps1 -UseBasicParsing | iex
```

Optional branch/ref override:

```bash
REM_UPDATE_REF=master curl -fsSL https://raw.githubusercontent.com/crnobog69/rem/master/scripts/update.sh | bash
```

## Remfile syntax (TOML)

`Remfile` uses TOML sections:

```text
default = "build"

[vars]
APP_NAME = "rem"
VERSION = "dev"

[task.gen]
desc = "Generate files"
cmds = ["go generate ./..."]

[task.build]
desc = "Build binary"
deps = ["gen"]
inputs = ["cmd/rem/main.go", "internal/*/*.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/rem",
]
```

Rules:

- Root key: `default = "task_name"`
- Variable table: `[vars]` with `NAME = "value"`
- Task tables: `[task.<name>]`
- Task fields: `desc`, `deps`, `inputs`, `outputs`, `dir`, `cmds`
- Optional `cmd` is still accepted as a single-command alias
- `${VAR}` and `${VAR:-fallback}` expansion is supported

## Everyday Remfile example (no GitHub release)

```toml
default = "build"

[vars]
APP_NAME = "myapp"
VERSION = "${VERSION:-dev}"

[task.build]
desc = "Build app"
inputs = ["cmd/myapp/main.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/myapp",
]

[task.test]
desc = "Run tests"
cmds = ["go test ./..."]

[task.clean]
desc = "Clean artifacts"
cmds = ["rm -rf bin dist"]
```

Common daily flow:

```bash
rem init
rem list -D VERSION=v0.1.0
rem build
rem run test
rem run clean
```

## CLI commands

```bash
rem doctor
rem graph -D APP_NAME=rem
rem format
rem format --check
rem build -j 8
```

`rem format` writes canonical TOML and does not preserve comments.
`rem init` creates `Remfile`, `REM.md`, and `REM.sr-Cyrl.md`.
CLI output uses colors on TTY; disable with `NO_COLOR=1`.
Task shell follows `$SHELL`; set `REM_SHELL=/path/to/shell` to force a specific shell.

## VS Code extension

A starter extension for `Remfile` syntax support is in:

- `vscode/remfile`

It includes language registration, grammar highlighting, and snippets.
It also includes lightweight diagnostics for Remfile TOML files.
See `vscode/remfile/README.md` for local run/package steps.

## Makefile migration example

Your provided `Makefile` translation is included at:

- `examples/Remfile.gitcrn`

## Update checks (GitHub Releases)

`rem` can check latest release metadata from GitHub:

- checked on every `rem` command startup
- disabled with `REM_NO_UPDATE_CHECK=1`
- default repo is `crnobog69/rem`
- override repo with `REM_UPDATE_REPO=owner/repo`
- override raw script ref with `REM_UPDATE_REF=master`
- or compile-time value via:

```bash
go build -ldflags "-X main.version=v0.1.0 -X main.updateRepo=owner/repo" -o rem ./cmd/rem
```

## Releases

Optional (for projects that use GitHub release flow).

Use:

```bash
./scripts/release.sh --version v0.1.0
rem run -D RELEASE_VERSION=v0.1.0 release-preflight
rem run -D RELEASE_VERSION=v0.1.0 github-release
```

`github-release` requires authenticated GitHub CLI (`gh auth login`).
It also adds update commands to the release description.
If preflight fails on local changes, either commit/stash first or run with `-D RELEASE_ALLOW_DIRTY=1`.

Outputs:

- `dist/rem-linux-amd64`
- `dist/rem-linux-arm64`
- `dist/rem-windows-amd64.exe`
- `dist/rem-windows-arm64.exe`
- `dist/checksums.txt`

## License

Apache-2.0. See [`LICENSE`](LICENSE).

## Serbian docs

See [`README.sr-Cyrl.md`](README.sr-Cyrl.md).
