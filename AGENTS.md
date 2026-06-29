# GoshPanel

A linux webserver control panel skeleton. The repo is an early-stage prototype made of
three **independent Go modules** (each has its own `go.mod`):

- `./` (root, `github.com/schmorrison/goshpanel`) — contains the `server` package (HTTP
  reverse-proxy/control-plane backend). Depends on `github.com/gorilla/mux`.
- `./frontend` (module `main`) — the WASM web UI built with `github.com/gopherjs/vecty`
  (Materialize styling). This is the runnable product surface.
- `./cmd` (module `main`) — intended server entrypoint, uses `github.com/spf13/viper`.

## Cursor Cloud specific instructions

Go 1.22 is preinstalled and is the correct toolchain. Dependencies are resolved per-module
and committed in each `go.mod`/`go.sum`; the startup update script just runs `go mod download`
in each module. Build/lint/test/run commands per module:

| Module | Lint / Test | Build | Run |
| --- | --- | --- | --- |
| root (`server`) | `go vet ./...` / `go test ./...` (no tests yet) | `go build ./...` | library only — see caveat |
| `frontend` | `cd frontend && go vet ./...` | `cd frontend && GOOS=js GOARCH=wasm go build -o frontend.wasm .` | serve the wasm (see below) |
| `cmd` | n/a | does NOT compile — see caveat | n/a |

Non-obvious caveats (these are pre-existing code-level issues, NOT environment problems):

- **Pinned dependency versions matter.** `frontend` pins `gopherjs/vecty v0.5.0` on purpose:
  v0.6.0 renamed its module path to `github.com/hexops/vecty` and breaks the
  `github.com/gopherjs/vecty` imports. It pins `golang.org/x/net v0.17.0` because newer
  releases require a newer Go toolchain (auto-downloads Go 1.25). `cmd` pins
  `spf13/viper v1.7.1` for the same toolchain reason. Do not blindly `go mod tidy` to latest.
- **Build the frontend with the package path `.`, not `./...`.** `./...` fails with
  "cannot write multiple packages to non-directory" because it tries to combine the main
  package with the `components/*` subpackages into one output.
- **`cmd` does not build.** Two pre-existing code bugs: `cmd/config.go` declares `package cmd`
  while `cmd/main.go` declares `package main` in the same directory, and `main.go` calls the
  unexported `server.serve()` without importing the `server` package. Fixing these requires
  code changes, not environment changes.
- **The root `server` package is a library** (`serve()` is unexported) with no working `main`,
  so it cannot be run standalone without code changes.
- **Running the frontend WASM UI:** build `frontend.wasm`, place it next to an `index.html`
  loader and `wasm_exec.js` (copy from `$(go env GOROOT)/misc/wasm/wasm_exec.js` for Go 1.22),
  then serve via a static server that returns `Content-Type: application/wasm` for `.wasm`
  (Go's `http.FileServer` does this automatically; `python -m http.server` may need the mime
  type registered). Then open the served `index.html` in a browser.
- **Frontend runtime bug:** the unmodified `frontend/home.go` `Render()` returns
  `elem.Section(...)`, but `vecty.RenderBody` requires the root component to render a `<body>`,
  so the app panics at startup until that returns `elem.Body(...)`. Also `card.go` only renders
  the card `Title` when `Image != ""`.
