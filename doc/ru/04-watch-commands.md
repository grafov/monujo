# 04. Команды `watch`

Группа `watch` — это **read‑only** операции: программа обращается
только к публичному HTTP API TronGrid и не затрагивает приватные
ключи. Никаких файлов не пишется. Подходит для:

- проверки баланса холодного кошелька с онлайн‑машины;
- просмотра входящих/исходящих переводов USDT;
- скриптов мониторинга.

```text
monujo watch
├── balance      — TRX и USDT для адреса
├── txs          — последние переводы USDT (TRC-20)
└── permissions  — owner/witness/active permissions (multisig-структура)
```

Глобальные флаги (`--network`, `--output`, `--http-url`, `--api-key`,
`--keystore-dir`, `--write-transport`) применимы ко всем подкомандам
группы. `--keystore-dir` нужен `watch permissions` для подсветки
локальных ключей; `--grpc-url` / `--grpc-insecure` для `watch` не
используются (только при `--write-transport grpc` для команд записи).

---

## `watch balance`

Запрашивает у TronGrid баланс TRX в санах и баланс TRC‑20 USDT в
минимальных единицах (1 USDT = 1 000 000 минимальных единиц).

### Использование

```bash
monujo watch balance --addr <TRON_BASE58_ADDRESS>
```

### Флаги

| Флаг     | Тип    | Обязательный | По умолчанию | Описание                                                                                              |
|----------|--------|--------------|--------------|-------------------------------------------------------------------------------------------------------|
| `--addr` | string | да           | —            | Tron‑адрес в Base58Check (начинается на `T`). Проверяется через `gotron-sdk` ещё до сетевого запроса. |

Других локальных флагов у этой команды нет.

### Что происходит

1. Адрес валидируется (`ValidateBase58Address`). При ошибке —
   немедленный выход с `invalid Tron address: ...`.
2. Делается HTTP‑GET `<HTTPBaseURL>/v1/accounts/<addr>` (с
   заголовком `TRON-PRO-API-KEY`, если задан).
3. Из ответа берутся:
   - `data[0].balance` — баланс TRX в санах;
   - `data[0].trc20[*]` — карта `<контракт>: <минимальные_единицы>`.
     Программа ищет в ней контракт USDT, соответствующий
     `--network` (см. [07 — Сети](07-networks.md)). Если такой
     записи нет, баланс USDT считается равным `0`.
4. Результат форматируется в выбранный `--output`.

Если адрес ещё не активирован on‑chain, TronGrid вернёт пустой
`data: []`. Программа в этом случае напечатает нулевые балансы (это
не ошибка, а нормальное состояние неактивированного аккаунта).

### Пример: формат `table`

```bash
$ monujo watch balance --addr TXYZ...
Address: TXYZ...
TRX:     12.345678 (12345678 sun)
USDT:    250 (minimal units 250000000)
```

- Сумма TRX выводится одновременно как десятичное число (с точностью
  до 6 знаков, см. `domain.FormatSunAsTRX`) и в санах.
- Для USDT — десятичное представление (6 знаков) и минимальные
  единицы.

### Пример: формат `json`

```bash
$ monujo --output json watch balance --addr TXYZ...
{
  "address": "TXYZ...",
  "trx_sun": 12345678,
  "trx": "12.345678",
  "usdt_minimal_units": "250000000",
  "usdt": "250"
}
```

Поля JSON стабильны и могут потребляться скриптами:

| Поле                 | Тип     | Описание                                                               |
|----------------------|---------|------------------------------------------------------------------------|
| `address`            | string  | Адрес из запроса.                                                      |
| `trx_sun`            | integer | Баланс в санах.                                                        |
| `trx`                | string  | Тот же баланс, представленный десятичным числом (без хвостовых нулей). |
| `usdt_minimal_units` | string  | Баланс USDT в минимальных единицах (строка, чтобы не терять точность). |
| `usdt`               | string  | Десятичное представление того же числа.                                |

### Возможные ошибки

- `invalid Tron address: ...` — `--addr` не валиден.
- `trongrid accounts: HTTP 4xx: ...` — узел TronGrid вернул ошибку.
  При HTTP 401/403 — проблема с API‑ключом, при 429 — рейт‑лимит.
- `trongrid: <message>` — успешный HTTP‑код, но ответ помечен как
  неуспешный (`success=false` и пустой `data`).
- `decode accounts: ...` — нештатный JSON в ответе.

---

## `watch txs`

Возвращает последние подтверждённые переводы TRC‑20 USDT для адреса
(в обе стороны).

### Использование

```bash
monujo watch txs --addr <TRON_BASE58_ADDRESS> [--limit N]
```

### Флаги

| Флаг | Тип | Обязательный | По умолчанию | Описание |
|------|-----|--------------|--------------|----------|
| `--addr` | string | да | — | Tron‑адрес в Base58Check. |
| `--limit` | int | нет | `20` | Максимум транзакций. Допустимый диапазон **1..200**; значения ≤ 0 заменяются на 20, > 200 — на 200. |

### Что происходит

1. Валидация адреса.
2. Подбирается контракт USDT для `--network`.
3. Делается HTTP‑GET `<HTTPBaseURL>/v1/accounts/<addr>/transactions/trc20`
   с параметрами:
   - `only_confirmed=true` (только подтверждённые),
   - `limit=<N>`,
   - `contract_address=<адрес USDT>`.
4. Каждая запись преобразуется во внутреннюю модель
   `TRC20TransferRow` и форматируется в `table` или `json`.

Поведение по «направлению» переводов: TronGrid отдаёт сразу все
переводы, где адрес — отправитель **или** получатель; направление
видно по полям `from` и `to`. Поле `type` приходит как строка от
TronGrid (например, `"Transfer"`), monujo его не интерпретирует.

### Пример: формат `table`

```text
$ monujo watch txs --addr TXYZ... --limit 3
2026-04-19T08:11:23Z  abc123...  TFROM... -> TXYZ...   1500000000 USDT  [Transfer]
2026-04-18T10:02:11Z  def456...  TXYZ... -> TANOTHER...  250000000 USDT  [Transfer]
2026-04-17T22:55:01Z  789aaa...  TFROM2... -> TXYZ...    50000000 USDT  [Transfer]
```

Поля по порядку: время в RFC3339, идентификатор транзакции, отправитель,
получатель, сумма в **минимальных единицах** (без деления на 10^6),
символ токена, в квадратных скобках — `type`.

> Внимание: суммы в `table`‑выводе не делятся на 10^6. Чтобы получить
> «человеческое» количество USDT, разделите на 1 000 000.

### Пример: формат `json`

```json
[
  {
    "tx_id": "abc123...",
    "time": "2026-04-19T08:11:23Z",
    "from": "TFROM...",
    "to": "TXYZ...",
    "value_minimal": "1500000000",
    "token": "USDT",
    "contract": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
    "type": "Transfer"
  }
]
```

Поля:

| Поле            | Тип    | Описание                                                      |
|-----------------|--------|---------------------------------------------------------------|
| `tx_id`         | string | Хеш транзакции.                                               |
| `time`          | string | Время блока в RFC3339 (`time.UnixMilli`).                     |
| `from`          | string | Адрес отправителя в Base58.                                   |
| `to`            | string | Адрес получателя.                                             |
| `value_minimal` | string | Сумма в минимальных единицах токена.                          |
| `token`         | string | Символ токена (если у TronGrid пусто — подставляется `USDT`). |
| `contract`      | string | Адрес контракта токена.                                       |
| `type`          | string | Категория события из API TronGrid.                            |

### Возможные ошибки

- `invalid Tron address: ...` — некорректный адрес.
- `trongrid trc20 txs: HTTP 4xx: ...` — ошибка вызова API.
- `decode trc20 txs: ...` — некорректный JSON.
- Пустой массив (`[]` для JSON, отсутствие строк для table) —
  переводов не было или адрес ещё не активирован.

### Примечания

- `watch txs` показывает **только переводы выбранного USDT‑контракта**,
  а не любые TRC‑20. Для тестового токена на Shasta используется
  `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (`Tron.constants.go`), который,
  напомним, **не является** настоящим Tether.
- Команда не делает пагинации: один вызов — одно окно из не более чем
  200 последних подтверждённых записей. Для глубокой истории нужны
  собственные средства (например, прямые вызовы TronGrid с
  `fingerprint` из ответа).

---

## `watch permissions`

Показывает multisig-структуру аккаунта: `owner_permission` (id=0),
опциональный `witness_permission` (id=1) и все `active_permission`
(id=2..9). Для каждой permission печатается id, имя, threshold, список
разрешённых контрактных типов (operations bitmap декодируется через
`core.Transaction_Contract_ContractType_name`) и список подписантов
`keys: {address, weight}`.

Если адрес ключа совпадает с адресом записи в `--keystore-dir`, она
помечается **`[local: <name>]`** для зашифрованных кошельков или
**`[external: <name>]`** для записей `wallet import-external` (только
адрес) — так видно, какие ключи в списке совпадают с вашими именами.

### Использование

```bash
monujo watch permissions --addr <TRON_BASE58_ADDRESS>
```

### Флаги

| Флаг     | Тип    | Обязательный | По умолчанию | Описание                                                     |
|----------|--------|--------------|--------------|--------------------------------------------------------------|
| `--addr` | string | да           | —            | Tron base58-адрес. Валидируется локально до HTTP-запроса.    |

Глобальный `--keystore-dir` используется только для резолвинга
локальных кошельков; сам keystore не расшифровывается.

### Пример: формат `table`

```text
$ monujo watch permissions --addr TXYZ...
permission owner  id=0  threshold=2
  keys:
    TAAA...  weight=1  [local: alice]
    TBBB...  weight=1
permission active0  id=2  threshold=2
  operations: TransferContract, TriggerSmartContract
  keys:
    TAAA...  weight=1  [local: alice]
    TCCC...  weight=1  [local: bob]
```

### Пример: формат `json`

```json
{
  "address": "TXYZ...",
  "owner": {
    "type": "owner",
    "id": 0,
    "permission_name": "owner",
    "threshold": 2,
    "keys": [
      {"address": "TAAA...", "weight": 1, "local_wallet": "alice"},
      {"address": "TBBB...", "weight": 1}
    ]
  },
  "active": [ ... ]
}
```

В объектах `keys` при совпадении с keystore поле `local_wallet` содержит
имя записи; для внешних (`import-external`) алиасов добавляется
`"external_wallet": true`.

### Возможные ошибки

- `invalid Tron address: ...` — некорректный адрес.
- `trongrid permissions: HTTP 4xx: ...` — ошибка вызова TronGrid.
- Пустой `permissions` (все поля `null`) — адрес ещё не активирован в
  сети или у него нет собственных permissions (что равнозначно
  дефолтной 1-of-1 схеме).

См. также [13. Команды `multisig`](13-multisig.md).
