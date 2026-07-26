# 07. Сети, эндпойнты и контракты

## Поддерживаемые сети

`monujo` работает с двумя сетями TRON:

| `--network`              | Назначение              | HTTP TronGrid                    | gRPC                            | USDT‑контракт                                                       |
|--------------------------|-------------------------|----------------------------------|---------------------------------|---------------------------------------------------------------------|
| `mainnet` (по умолчанию) | Основная публичная сеть | `https://api.trongrid.io`        | `grpc.trongrid.io:50051`        | `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` (Tether)                       |
| `shasta`                 | Тестовая сеть Shasta    | `https://api.shasta.trongrid.io` | `grpc.shasta.trongrid.io:50051` | `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (тестовый USDT‑подобный токен) |

Эндпойнты можно переопределять глобальными флагами:

- `--http-url` — заменяет HTTP‑базу TronGrid;
- `--grpc-url` — заменяет адрес `host:port` для gRPC.

Это полезно, если вы хотите использовать собственный полный узел Tron
или прокси TronGrid с особым доменом. Никаких других сетей (Nile,
private chain) программа не «знает» — но через `--http-url`/`--grpc-url`
их в принципе можно подключить, если они полностью совместимы по API.

## Адреса USDT

Контракт USDT определяется функцией `USDTContractForNetwork` в
`internal/tron/constants.go`:

- **Mainnet** — `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`. Это
  каноническая Tether USDT на TRON, 6 знаков после запятой.
  Используется в `watch balance`, `watch txs` (фильтр по контракту) и
  `hot send-usdt`.
- **Shasta** — `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`. Тестовый
  USDT‑подобный токен, **не настоящий Tether**. Удобен для
  отладки, но использовать его в продакшене нельзя.

Других контрактов monujo не показывает: `watch balance` и
`watch txs` фильтруют по этому единственному адресу. Если у вашего
кошелька есть другие TRC‑20 токены, они в выводе не появятся.

## Адресный формат

monujo принимает Tron‑адреса в стандартном **Base58Check** (начинается
на `T`, длина 34 символа). Примеры:

- mainnet/shasta‑адрес: `TLcVUPaYnQp1XzYSZ6h4kRxJZ6Ld9gP1aN`
- USDT‑контракт: `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`

Один и тот же приватный ключ даёт один и тот же адрес в **обеих** сетях
(mainnet и Shasta). Поэтому keystore не «привязан» к сети — он
«запоминает» сеть только в метаданных `networks`, чтобы предупреждать
о смене.

Адрес валидируется через `gotron-sdk` (`Base58ToAddress`). При ошибке
выводится `invalid Tron address: <причина>`.

## Комиссии и ресурсы

В TRON оплата отличается от Ethereum:

- **Bandwidth (полоса)** — расходуется на простые переводы (TRX,
  активация). Каждому аккаунту в сутки даётся бесплатный лимит.
  Если он исчерпан, недостаток оплачивается «сжиганием» небольшого
  количества TRX.
- **Energy (энергия)** — расходуется на вызов контрактов, в том
  числе `transfer` у TRC‑20 (USDT). Если energy не хватает,
  сжигается TRX по курсу узла.

`monujo` не показывает оценку расходов перед отправкой. Для
TRC‑20‑вызовов в коде вшит лимит `feeLimit = 100_000_000 sun`
(100 TRX). Если фактическая оплата за вызов окажется выше, узел
вернёт ошибку (например, `OUT_OF_ENERGY`).

Перед отправкой USDT убедитесь:

- На отправляющем адресе есть **TRX** (хотя бы 5–10 TRX про запас на
  energy/bandwidth).
- Адрес получателя существует (если нет — TronGrid всё равно примет
  транзакцию, аккаунт создастся, но в редких случаях бывают
  дополнительные расходы за создание аккаунта).

## Сравнение сетей

| Свойство           | Mainnet         | Shasta                                          |
|--------------------|-----------------|-------------------------------------------------|
| Реальные деньги    | Да              | Нет (тестовая)                                  |
| Где взять TRX      | Биржа, P2P      | [Shasta‑faucet](https://www.trongrid.io/faucet) |
| USDT‑контракт      | Tether          | Тестовый токен                                  |
| Стабильность узлов | Высокая         | Иногда деградирует, не несёт SLA                |
| Используется для   | Боевые операции | Отладка, обучение, тесты CI                     |

## Переопределение эндпойнтов

### Свой HTTP узел

```bash
monujo --http-url https://my-tron-node.example.com \
       watch balance --addr T...
```

Узел должен поддерживать минимум для **чтения** (TronGrid v1 API):

- `GET /v1/accounts/{addr}` — для `watch balance` / `hot balance`;
- `GET /v1/accounts/{addr}/transactions/trc20` — для `watch txs`.

Для **записи** по умолчанию (`--write-transport rest`) на том же
`--http-url` нужны Wallet HTTP методы: `/wallet/createtransaction`,
`/wallet/triggersmartcontract`, `/wallet/broadcasthex`,
`/wallet/getsignweight`.

API‑ключ передаётся в заголовке `TRON-PRO-API-KEY`. Если ваш узел
не использует ключи, `--api-key` можно опустить.

### Свой gRPC узел (opt-in)

```bash
monujo --write-transport grpc --grpc-url my-tron-grpc.example.com:50051 \
       --grpc-insecure \
       hot send-trx --from myhot --to T... --amount 1
```

Обычно нужен `--grpc-insecure` (plaintext gRPC). mTLS пока не
поддерживается флагами.

API‑ключ при наличии передаётся в метаданных gRPC через gotron-sdk.

### API‑ключ TronGrid

Бесплатный ключ TronGrid существенно увеличивает рейт‑лимит
(особенно для `watch txs`). Регистрация — на
[trongrid.io](https://www.trongrid.io). После получения:

```bash
export MONUJO_TRONGRID_API_KEY="<ваш ключ>"
monujo watch txs --addr T...
```

или флагом:

```bash
monujo --api-key "<ключ>" watch balance --addr T...
```

## Что записывается в метаданные кошелька

При запуске `wallet create`, `wallet import`, успешной отправке `hot send-trx`
или `hot send-usdt` текущий `--network` сохраняется в
`EncryptedWalletFile.networks`. Это нужно, чтобы при последующем
запуске с другим `--network` пользователь увидел предупреждение и
случайно не выполнил действие в неправильной сети (например, не
отправил тестовые операции на mainnet, или, наоборот, реальные
деньги на Shasta).

Подробнее см. [06 — Keystore](06-keystore.md#метаданные-networks).
