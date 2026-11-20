# 02. Konstruado kaj instalado

## Postuloj

- **Go 1.27.4** aŭ pli nova (vidu `go.mod`).
- **GNU Make** (por komforto; oni ankaŭ povas konstrui rekte per `go build`).
- **Git** (la Makefile uzas `git describe` kaj `git rev-parse` por
  enmeti la version en la binaraĵon).
- Interreta aliro al HTTPS TronGrid (`api.trongrid.io`,
  `api.shasta.trongrid.io`) — por lego kaj skribo defaŭlte.
  gRPC-finpunktoj bezonatas nur kun eksplicita `--write-transport grpc`.

Pliaj iloj (por evoluo; ne necesaj por produktada uzo):

- `golangci-lint` — `make lint`.
- `gosec` — `make sec`.
- `ginkgo` — instalita kiel `go tool`, uzata en `make test*`.

## Konstruado

```bash
git clone https://github.com/grafov/monujo.git
cd monujo
make build
```

Rezulto: la binaraĵo `build/monujo`. La Makefile trairas ĉiujn
subdosierujojn `cmd/*` kaj por ĉiu rulas `go build` kun flagoj:

```
-buildvcs=false -ldflags "-X main.version=<git describe ...> -X main.gitCommit=<git rev-parse --short HEAD>"
```

Tio signifas ke la versio kaj la komito estas bakitaj en la binaraĵon
kaj videblas en la eligo de `monujo --version`.

### Mana konstruado sen make

```bash
go build -o build/monujo \
  -ldflags "-X main.version=v0.1.0 -X main.gitCommit=$(git rev-parse --short HEAD)" \
  ./cmd/monujo
```

Sen `-ldflags` la versio montriĝas kiel `dev (unknown)` — la
defaŭltaj valoroj agorditaj en `cmd/monujo/main.go`.

## Instalado

En la Makefile ne estas celo `install`; la binaraĵo estas konstruita
en `build/`. Vi povas:

```bash
sudo install -m 0755 build/monujo /usr/local/bin/monujo
# or
cp build/monujo ~/.local/bin/monujo
```

Kontrolo:

```bash
monujo --version
# e.g.: v0.1.0 (a1b2c3d)
```

## Strukturo de voko

Ĉiuj komandoj havas la formon:

```
monujo [global flags] <group> <subcommand> [subcommand flags]
```

Komandogrupoj:

- `watch` — nur-legaj operacioj por arbitra adreso.
- `hot` — operacioj kun loka ĉifrita monujo.

Vidu ankaŭ [03 — Ĝenerala agordo](03-configuration.md), kaj la
plenan referencon de subkomandoj en
[04](04-watch-commands.md) kaj [05](05-hot-commands.md).

## Utilaj celoj de `make`

| Celo                    | Kion ĝi faras                                                                                                   |
|-------------------------|-----------------------------------------------------------------------------------------------------------------|
| `make build`            | Konstruas ĉiujn `cmd/*` en `build/`.                                                                            |
| `make test`             | Rulas Ginkgo-testojn tra la tuta deponejo.                                                                      |
| `make test-race`        | Rulas ĉiujn Ginkgo-testojn kun la race-detektilo.                                                               |
| `make test-coverage`    | Kalkulas kovron tra la tuta deponejo.                                                                           |
| `make test-integration` | Ŝaltas Tron-integrajn testojn (`MONUJO_TRON_E2E=1`, etikedo `integration`). Postulas aliron al realaj nodoj.    |
| `make lint`             | `golangci-lint run ./...`.                                                                                      |
| `make lint-fix`         | Same, sed kun aŭtomata korekto.                                                                                 |
| `make sec`              | `gosec ./...`.                                                                                                  |
| `make check`            | Plena loka pordego: konstruado, race-testoj, lint kaj sec.                                                      |
| `make tidy`             | `go mod tidy`.                                                                                                  |
| `make clean`            | `go clean` + forigo de `build/`.                                                                                |

## Kie trovi la propran helpon de la programo

```bash
monujo --help                # list of global flags and groups
monujo watch --help          # watch subcommands
monujo watch balance --help  # flags of a specific subcommand
monujo hot --help
monujo hot send-usdt --help
```

La helpo de `cobra` estas generita el la fonto kaj ĉiam kongruas kun
la konstruaĵo. Ĉi tiu dokumentaro duobligas tiun helpon kaj aldonas
klarigojn pri konduto kaj flankefikoj.
