# 02. Сборка и установка

## Требования

- **Go 1.25** или новее (см. `go.mod`, директива `go 1.25.0`).
- **GNU Make** (для удобства; собирать можно и напрямую `go build`).
- **Git** (Makefile использует `git describe` и `git rev-parse` для
  записи версии в бинарник).
- Доступ в интернет к узлам TronGrid (`api.trongrid.io`,
  `api.shasta.trongrid.io`) и gRPC‑эндпойнтам (`grpc.trongrid.io:50051`,
  `grpc.shasta.trongrid.io:50051`) — иначе сетевые операции работать не
  будут.

Дополнительные инструменты (для разработки, не нужны для эксплуатации):

- `golangci-lint` — `make lint`.
- `gosec` — `make sec`.
- `ginkgo` — установлен как `go tool`, используется в `make test*`.

## Сборка

```bash
git clone https://github.com/grafov/monujo.git
cd monujo
make build
```

Результат: бинарник `build/monujo`. Makefile проходит по всем
подкаталогам `cmd/*` и для каждого делает `go build` с флагами:

```
-buildvcs=false -ldflags "-X main.version=<git describe ...> -X main.gitCommit=<git rev-parse --short HEAD>"
```

Это значит, что версия и коммит впечатываются прямо в бинарник, и их
можно увидеть в выводе `monujo --version`.

### Сборка вручную, без make

```bash
go build -o build/monujo \
  -ldflags "-X main.version=v0.1.0 -X main.gitCommit=$(git rev-parse --short HEAD)" \
  ./cmd/monujo
```

Без `-ldflags` версия будет показана как `dev (unknown)` — это
значения по умолчанию, заданные в `cmd/monujo/main.go`.

### Сборка в Docker

В Makefile есть цель `docker-build`, требующая `Dockerfile` (в
текущей версии репозитория его нет — соберите образ собственным
рецептом или используйте локальный бинарник).

```bash
GITHUB_TOKEN=... make docker-build
```

## Установка

В Makefile нет цели `install`, бинарник собирается в `build/`. Можно:

```bash
sudo install -m 0755 build/monujo /usr/local/bin/monujo
# или
cp build/monujo ~/.local/bin/monujo
```

Проверка:

```bash
monujo --version
# например: v0.1.0 (a1b2c3d)
```

## Структура запусков

Все команды имеют форму:

```
monujo [глобальные флаги] <группа> <подкоманда> [флаги подкоманды]
```

Группы команд:

- `watch` — read‑only операции по произвольному адресу.
- `hot` — операции с локальным зашифрованным кошельком.

См. также [03 — Глобальная конфигурация](03-configuration.md), а
полный справочник по подкомандам — в файлах
[04](04-watch-commands.md) и [05](05-hot-commands.md).

## Полезные `make`‑цели

| Цель                    | Что делает                                                                                                      |
|-------------------------|-----------------------------------------------------------------------------------------------------------------|
| `make build`            | Собирает все `cmd/*` в `build/`.                                                                                |
| `make test`             | Запускает Ginkgo‑тесты по всему репозиторию.                                                                    |
| `make test-race`        | Запускает тесты Tron‑пакета с детектором гонок.                                                                 |
| `make test-coverage`    | Считает покрытие по `internal/tron`.                                                                            |
| `make test-integration` | Включает интеграционные тесты Tron (`MONUJO_TRON_E2E=1`, тег `integration`). Требует доступа к настоящим узлам. |
| `make lint`             | `golangci-lint run ./...`.                                                                                      |
| `make lint-fix`         | То же, но с автофиксом.                                                                                         |
| `make sec`              | `gosec ./...`.                                                                                                  |
| `make tidy`             | `go mod tidy`.                                                                                                  |
| `make clean`            | `go clean` + удаление `build/`.                                                                                 |

## Где искать справку самой программы

```bash
monujo --help                # перечень глобальных флагов и групп
monujo watch --help          # подкоманды watch
monujo watch balance --help  # флаги конкретной подкоманды
monujo hot --help
monujo hot send-usdt --help
```

`cobra`‑справка генерируется из исходного кода и всегда соответствует
сборке. Эта документация дублирует справку, дополняя её пояснениями
поведения и побочных эффектов.
