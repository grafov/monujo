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

Базовые URL и хосты gRPC зависят от `--network` (`mainnet` или
`shasta`). Их можно заменить флагами `--http-url` и `--grpc-url` на
совместимый TronGrid или свой полный узел.

| Команда                         | Протокол               | Домен по умолчанию (mainnet / shasta)                      | Эндпойнты или RPC                                                                                                                                             |
|---------------------------------|------------------------|------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `watch balance`                 | HTTPS (REST, TronGrid) | `api.trongrid.io` / `api.shasta.trongrid.io`               | `GET /v1/accounts/{address}`                                                                                                                                  |
| `watch txs`                     | HTTPS (REST, TronGrid) | то же                                                      | `GET /v1/accounts/{address}/transactions/trc20` с query-параметрами `only_confirmed=true`, `limit`, `contract_address` (канонический USDT для выбранной сети) |
| `hot balance`                   | HTTPS (REST, TronGrid) | то же                                                      | Те же запросы, что у `watch balance` (чтение баланса через HTTP API)                                                                                          |
| `hot send-trx`                  | gRPC over TLS          | `grpc.trongrid.io:50051` / `grpc.shasta.trongrid.io:50051` | Сборка: `Wallet.CreateTransaction2`; отправка: `Wallet.BroadcastTransaction`                                                                                  |
| `hot send-usdt`                 | gRPC over TLS          | то же                                                      | Сборка TRC-20: `Wallet.TriggerContract`; отправка: `Wallet.BroadcastTransaction`                                                                              |
| `offline build` (`--kind=trx`)  | gRPC over TLS          | то же                                                      | `Wallet.CreateTransaction2` (только неподписанная транзакция, без broadcast)                                                                                  |
| `offline build` (`--kind=usdt`) | gRPC over TLS          | то же                                                      | `Wallet.TriggerContract` (только сборка, без broadcast)                                                                                                       |
| `offline broadcast`             | gRPC over TLS          | то же                                                      | `Wallet.BroadcastTransaction`                                                                                                                                 |

Имена RPC приведены в терминах **Tron Wallet gRPC API**, к которому
подключается зависимость `gotron-sdk` (`internal/tron/write_client.go`).

### Полные URL для HTTPS

При значениях по умолчанию HTTP-база такова:

- mainnet: `https://api.trongrid.io`
- shasta: `https://api.shasta.trongrid.io`

К ней добавляются пути из таблицы, например:
`https://api.trongrid.io/v1/accounts/T...`.

### API-ключ TronGrid

Если задан `--api-key` или переменная окружения `MONUJO_TRONGRID_API_KEY`,
к **HTTPS**-запросам добавляется заголовок `TRON-PRO-API-KEY`. Для
**gRPC** ключ передаётся через клиент SDK тем же конфигом (см. код
`NewWriteClientFromConfig`).

## Переопределение узлов

- `--http-url` — полная база для REST (например `https://api.trongrid.io`);
  используется только чтение (`ReadClient`).
- `--grpc-url` — адрес вида `host:port` для операций записи и сборки
  транзакций (`WriteClient`).
- `--grpc-insecure` — отключает TLS для gRPC. Используйте только для
  доверенного локального узла, приватной сети или TLS-терминации снаружи
  процесса.

Оба параметра могут указывать на **другой домен**, если узел реализует
тот же HTTP- и Wallet gRPC-интерфейс, что ожидает monujo.

## Риски и жёсткая конфигурация

1. **TLS по умолчанию.** Запросы к TronGrid REST идут по **HTTPS**, а
   gRPC-клиент использует TLS по умолчанию. Plaintext gRPC доступен только
   через `--grpc-insecure` для доверенных локальных или приватных узлов.

2. **Доверие к узлу.** Узел видит адреса отправителя, суммы и при
   broadcast — подписанные транзакции. Выбирайте поставщика endpoint’ов
   осознанно.

3. **Нет посторонней телеметрии.** В составе monujo нет отдельных
   запросов к аналитике, CDN или сторонним API: исходящий трафик сводится
   к настроенным HTTP- и gRPC-эндпойнтам Tron (см. таблицу выше).
