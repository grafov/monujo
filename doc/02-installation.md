# 02. Build and installation

## Requirements

- **Go 1.26.5** or newer (see `go.mod`).
- **GNU Make** (for convenience; you can also build directly with `go build`).
- **Git** (the Makefile uses `git describe` and `git rev-parse` to
  embed the version into the binary).
- Internet access to HTTPS TronGrid (`api.trongrid.io`,
  `api.shasta.trongrid.io`) — for read and write by default.
  gRPC endpoints are needed only with an explicit `--write-transport grpc`.

Additional tools (for development; not required for production use):

- `golangci-lint` — `make lint`.
- `gosec` — `make sec`.
- `ginkgo` — installed as `go tool`, used in `make test*`.

## Build

```bash
git clone https://github.com/grafov/monujo.git
cd monujo
make build
```

Result: the `build/monujo` binary. The Makefile walks all
`cmd/*` subdirectories and for each runs `go build` with flags:

```
-buildvcs=false -ldflags "-X main.version=<git describe ...> -X main.gitCommit=<git rev-parse --short HEAD>"
```

That means the version and commit are baked into the binary and can
be seen in the output of `monujo --version`.

### Manual build without make

```bash
go build -o build/monujo \
  -ldflags "-X main.version=v0.1.0 -X main.gitCommit=$(git rev-parse --short HEAD)" \
  ./cmd/monujo
```

Without `-ldflags` the version is shown as `dev (unknown)` — the
default values set in `cmd/monujo/main.go`.

## Installation

There is no `install` target in the Makefile; the binary is built into `build/`. You can:

```bash
sudo install -m 0755 build/monujo /usr/local/bin/monujo
# or
cp build/monujo ~/.local/bin/monujo
```

Check:

```bash
monujo --version
# e.g.: v0.1.0 (a1b2c3d)
```

## Invocation structure

All commands have the form:

```
monujo [global flags] <group> <subcommand> [subcommand flags]
```

Command groups:

- `watch` — read‑only operations for an arbitrary address.
- `hot` — operations with a local encrypted wallet.

See also [03 — Global configuration](03-configuration.md), and the
full subcommand reference in
[04](04-watch-commands.md) and [05](05-hot-commands.md).

## Useful `make` targets

| Target                  | What it does                                                                                                    |
|-------------------------|-----------------------------------------------------------------------------------------------------------------|
| `make build`            | Builds all `cmd/*` into `build/`.                                                                               |
| `make test`             | Runs Ginkgo tests across the whole repository.                                                                  |
| `make test-race`        | Runs all Ginkgo tests with the race detector.                                                                   |
| `make test-coverage`    | Computes coverage across the whole repository.                                                                  |
| `make test-integration` | Enables Tron integration tests (`MONUJO_TRON_E2E=1`, `integration` tag). Requires access to real nodes.         |
| `make lint`             | `golangci-lint run ./...`.                                                                                      |
| `make lint-fix`         | Same, but with autofix.                                                                                         |
| `make sec`              | `gosec ./...`.                                                                                                  |
| `make check`            | Full local gate: build, race‑tests, lint, and sec.                                                              |
| `make tidy`             | `go mod tidy`.                                                                                                  |
| `make clean`            | `go clean` + remove `build/`.                                                                                   |

## Where to find the program's own help

```bash
monujo --help                # list of global flags and groups
monujo watch --help          # watch subcommands
monujo watch balance --help  # flags of a specific subcommand
monujo hot --help
monujo hot send-usdt --help
```

`cobra` help is generated from the source and always matches the
build. This documentation duplicates that help and adds explanations
of behaviour and side effects.
