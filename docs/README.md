# Руководство пользователя monujo

`monujo` — минималистичный CLI на Go для работы с сетью **TRON**: чтение
балансов и истории переводов **TRC‑20 USDT** для холодных адресов
(только публичные данные, без приватных ключей) и операции **горячего
кошелька** (создание/импорт ключа, отправка TRX и USDT) с **локальным
зашифрованным хранилищем ключей**.

В этой документации подробно описано, как устроен бинарник, какие у
него команды, флаги и переменные окружения, как хранятся ключи и какие
типичные сценарии использования предусмотрены.

## Содержание

| Раздел                                              | О чём                                                                                                                                                 |
|-----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| [01 — Обзор и архитектура](01-overview.md)          | Что делает программа, из каких компонентов состоит, модель угроз.                                                                                     |
| [02 — Сборка и установка](02-installation.md)       | Требования к окружению, `make build`, установка бинарника, версии.                                                                                    |
| [03 — Глобальная конфигурация](03-configuration.md) | Все глобальные флаги (`--network`, `--output`, `--http-url`, `--grpc-url`, `--api-key`, `--keystore-dir`) и переменные окружения.                     |
| [04 — Команды `watch`](04-watch-commands.md)        | Подробный справочник по `watch balance` и `watch txs`.                                                                                                |
| [05 — Команды `hot`](05-hot-commands.md)            | Подробный справочник по `hot list`, `hot create`, `hot import`, `hot activate`, `hot balance`, `hot send-trx`, `hot send-usdt`.                       |
| [06 — Хранилище ключей](06-keystore.md)             | Формат v2 (Argon2id) и legacy v1 (scrypt) с автомиграцией, KDF/шифрование, метаданные сетей, соответствие стандартам, резервное копирование.          |
| [07 — Сети и контракты](07-networks.md)             | Mainnet и Shasta, эндпойнты по умолчанию, адреса контрактов USDT, лимиты комиссий.                                                                    |
| [08 — Типовые сценарии](08-scenarios.md)            | Пошаговые рецепты: проверка холодного адреса, создание горячего кошелька, активация, отправка USDT, тестирование на Shasta, скриптовое использование. |
| [09 — Устранение неполадок](09-troubleshooting.md)  | Расшифровка ошибок и способы их обхода.                                                                                                               |
| [10 — Команды `offline`](10-offline-commands.md)    | Air-gap сценарий: `offline build` (онлайн), `offline sign` (офлайн), `offline broadcast` (онлайн); формат конверта, HMAC, изоляция keystore.          |

## Краткая шпаргалка

```bash
# Сборка
make build                                    # бинарник build/monujo

# Watch‑only (без ключей)
./build/monujo watch balance --addr T...
./build/monujo watch txs     --addr T... --limit 50

# Управление кошельками
./build/monujo hot list
./build/monujo hot create --name myhot
./build/monujo hot import --name imported --key-hex <64_HEX>

# Операции на сети (требуют пароля от keystore)
./build/monujo hot balance   --name myhot
./build/monujo hot activate  --from funding --to T... --amount 1
./build/monujo hot send-trx  --name myhot --to T... --amount 12.5
./build/monujo hot send-usdt --name myhot --to T... --amount 10.25

# Тестовая сеть
./build/monujo --network shasta watch balance --addr T...

# Машинно-читаемый вывод
./build/monujo --output json watch balance --addr T...

# Air-gap (оффлайн-подпись)
./build/monujo offline build      --from-address T...cold --to T...rcpt --amount 12.5 --kind trx --out tx.unsigned.json
./build/monujo offline sign       --in tx.unsigned.json --name cold --out tx.signed.json   # на офлайн-ПК
./build/monujo offline broadcast  --in tx.signed.json
```

## Важные предупреждения

- `monujo` — **не аппаратный кошелёк**. Безопасность держится на: доверенной
  сборке, доверенной машине, правах `0600` на файлах keystore, надёжной
  парольной фразе и (опционально) API‑ключе TronGrid.
- **Бэкап keystore обязателен.** Копируйте `~/.config/monujo/keystore/<имя>.json`
  на отдельный носитель, пароль храните отдельно. Подробная процедура и
  рекомендации — в [06-keystore.md → «Резервное копирование keystore»](06-keystore.md#резервное-копирование-keystore).
- **FIPS / compliance.** monujo не сертифицирован как криптомодуль и
  использует Argon2id (не FIPS-approved). Для регулируемых сред см.
  [06-keystore.md → «Соответствие стандартам»](06-keystore.md#соответствие-стандартам).
- Никогда не коммитьте файлы keystore, парольные фразы или hex‑ключи в
  репозиторий. Переменная `MONUJO_IMPORT_KEY` поддерживается только для
  скриптовых сценариев в контролируемом окружении.
- `hot create --export-private-key` печатает приватный ключ в открытом
  виде один раз — используйте только если осознанно принимаете риск.
- Контракт USDT на Shasta (`TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`) — это
  тестовый токен, **не настоящий Tether**. Реальный USDT существует
  только на mainnet.
