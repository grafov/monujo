# 12. Сетевая активность и безопасность трафика

Этот раздел перечисляет **только те подкоманды `monujo`**, которые
инициируют исходящие сетевые запросы, с указанием протокола, доменов по
умолчанию и путей или gRPC-методов. Для флагов и переменных окружения
см. [03. Глобальная конфигурация](03-configuration.md); для сетей и
адресов контрактов USDT — [07. Сети, эндпойнты и контракты](07-networks.md).

## Команды без обращения к сети

Следующие подкоманды **не** открывают соединений к Tron API и не
обращаются к другим удалённым хостам (только локальный диск и ввод
пользователя):

- `wallet list`, `wallet create`, `wallet import`, `wallet check`
- `offline sign`
- `tool genpass`
- `tool vault pack`, `tool vault unpack`

## Таблица: команды с сетевым трафиком

Базовые URL зависят от `--network` (`mainnet` или `shasta`). Их можно
заменить флагом `--http-url` на совместимый TronGrid или свой полный узел.

По умолчанию **запись** (сборка и broadcast транзакций) идёт через
**HTTPS REST** (`--write-transport rest`). Опционально: `--write-transport grpc`
и `--grpc-url` (см. раздел gRPC ниже).

| Команда                         | Протокол (по умолчанию) | Домен (mainnet / shasta)     | Эндпойнты                                                                 |
|---------------------------------|-------------------------|------------------------------|---------------------------------------------------------------------------|
| `watch balance`                 | HTTPS (REST, TronGrid)  | `api.trongrid.io` / `api.shasta.trongrid.io` | `GET /v1/accounts/{address}`                                              |
| `watch txs`                     | HTTPS (REST, TronGrid)  | то же                        | `GET /v1/accounts/{address}/transactions/trc20`                           |
| `hot balance`                   | HTTPS (REST, TronGrid)  | то же                        | те же запросы, что у `watch balance`                                      |
| `hot send-trx`                  | HTTPS (REST)            | то же                        | `POST /wallet/createtransaction`, `POST /wallet/broadcasthex`             |
| `hot send-usdt`                 | HTTPS (REST)            | то же                        | `POST /wallet/triggersmartcontract`, `POST /wallet/broadcasthex`          |
| `offline build` (`--kind=trx`)  | HTTPS (REST)            | то же                        | `POST /wallet/createtransaction` (без broadcast)                            |
| `offline build` (`--kind=usdt`) | HTTPS (REST)            | то же                        | `POST /wallet/triggersmartcontract` (без broadcast)                         |
| `offline broadcast`             | HTTPS (REST)            | то же                        | `POST /wallet/broadcasthex`                                               |
| `multisig info`                 | HTTPS (REST)            | то же                        | `POST /wallet/getsignweight`                                              |
| `multisig broadcast`            | HTTPS (REST)            | то же                        | `POST /wallet/getsignweight`, `POST /wallet/broadcasthex`                 |

### Полные URL для HTTPS

При значениях по умолчанию HTTP-база такова:

- mainnet: `https://api.trongrid.io`
- shasta: `https://api.shasta.trongrid.io`

К ней добавляются пути из таблицы, например:
`https://api.trongrid.io/v1/accounts/T...`.

### API-ключ TronGrid

Если задан `--api-key` или переменная окружения `MONUJO_TRONGRID_API_KEY`,
к **HTTPS**-запросам добавляется заголовок `TRON-PRO-API-KEY`.

## Опциональный транспорт gRPC (`--write-transport grpc`)

Для совместимости с `java-tron` gRPC или отладки можно явно выбрать gRPC
через gotron-sdk:

| Команда (те же, что выше) | Протокол (opt-in) | Хост по умолчанию                          | RPC (Wallet API)                    |
|---------------------------|-------------------|--------------------------------------------|-------------------------------------|
| `hot send-*`, `offline build/broadcast`, `multisig info/broadcast` | gRPC (часто **plaintext**) | `grpc.trongrid.io:50051` / `grpc.shasta.trongrid.io:50051` | `CreateTransaction2`, `TriggerContract`, `BroadcastTransaction`, `GetTransactionSignWeight` |

Флаги:

- `--write-transport grpc` — включить gRPC вместо REST.
- `--grpc-url` — `host:port` полного узла.
- `--grpc-insecure` — plaintext gRPC (нужен для публичного TronGrid и большинства локальных full node).

При `--write-transport grpc --grpc-insecure` на известных хостах TronGrid
CLI выводит **предупреждение в stderr**: публичный TronGrid gRPC не
использует TLS; предпочтительнее дефолтный `--write-transport rest`.

## Переопределение узлов

- `--http-url` — база для REST (чтение и запись при `rest`).
- `--grpc-url` — только при `--write-transport grpc`.

Оба параметра могут указывать на **другой домен**, если узел реализует
тот же HTTP- и (при необходимости) Wallet gRPC-интерфейс.

## Риски и жёсткая конфигурация

1. **HTTPS REST по умолчанию.** Чтение и запись через TronGrid идут по
   **HTTPS** на `api.*.trongrid.io`. gRPC — только явный opt-in.

2. **Plaintext gRPC.** TronGrid и типичный `java-tron` на `:50051` принимают
   gRPC **без TLS**. Не включайте `--write-transport grpc` в недоверенной
   сети без понимания риска MITM.

3. **Доверие к узлу.** Узел видит адреса, суммы и подписанные транзакции.

4. **Нет посторонней телеметрии.** Исходящий трафик сводится к настроенным
   HTTP/gRPC эндпойнтам (см. таблицы выше).
