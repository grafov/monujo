# Руководство пользователя monujo

[English](../README.md) | [Esperanto](../eo/README.md)

*Примечание: вся документация (и переводы) сгенерирована LLM!*

`monujo` — минималистичный Go CLI для работы с сетью **TRON**:
чтение балансов и истории переводов **TRC-20 USDT** для холодных
адресов (только публичные данные, без приватных ключей), управление
**локальным зашифрованным keystore** (создание/импорт; **имена не
должны быть валидными Tron-адресами**, чтобы не путаться с
`--addr`/`--from`/`--to`; опционально **внешние** именованные
адреса), операции **горячего кошелька** (онлайн-подпись и отправка
TRX/USDT) и **air-gap подпись** для холодного хранения.

Эта документация подробно описывает устройство бинарника, команды,
флаги и переменные окружения, хранение ключей и типовые сценарии
использования.

## Содержание

| Раздел                                             | О чём                                                                                                                                                    |
|----------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| [01 — Обзор и архитектура](01-overview.md)         | Что делает программа, из каких компонентов состоит, модель угроз.                                                                                        |
| [02 — Сборка и установка](02-installation.md)      | Требования к окружению, `make build`, установка бинарника, версии.                                                                                       |
| [03 — Глобальная конфигурация](03-configuration.md)| Все глобальные флаги (`--network`, `--output`, `--http-url`, `--write-transport`, `--grpc-url`, `--grpc-insecure`, `--allow-insecure-http`, `--api-key`, `--keystore-dir`) и переменные окружения. |
| [04 — Команды `watch`](04-watch-commands.md)       | Подробный справочник по `watch balance` и `watch txs`.                                                                                                   |
| [05 — Команды `hot`](05-hot-commands.md)           | Подробный справочник по `hot balance`, `hot send-trx`, `hot send-usdt` (онлайн-операции).                                                                |
| [06 — Keystore](06-keystore.md)                    | Формат v2 (Argon2id) и legacy v1 (scrypt) с автомиграцией, KDF/шифрование, **внешний** JSON только с адресом, метаданные сети, соответствие стандартам, бэкап. |
| [07 — Сети и контракты](07-networks.md)            | Mainnet и Shasta, эндпойнты по умолчанию, адреса контрактов USDT, лимиты комиссии.                                                                       |
| [08 — Типовые сценарии](08-scenarios.md)           | Пошаговые рецепты: проверка холодного адреса, создание hot-кошелька, первый TRX на адрес, отправка USDT, тесты на Shasta, скриптовое использование.     |
| [09 — Устранение неполадок](09-troubleshooting.md) | Разбор ошибок и способы обхода.                                                                                                                          |
| [10 — Команды `offline`](10-offline-commands.md)   | Air-gap сценарий: `offline build` (online), `offline sign` (offline), `offline broadcast` (online); формат конверта, HMAC, изоляция keystore.            |
| [11 — Команды `wallet`](11-wallet-commands.md)     | Жизненный цикл keystore: `wallet list` (local vs external), `wallet create`, `wallet import`, `wallet import-external`, `wallet check`. Без сети; работает online и на air-gap. |
| [12 — Сетевая активность и безопасность](12-network-security.md) | Какие подкоманды ходят в TronGrid HTTPS vs Tron gRPC, хосты и пути/RPC по умолчанию, API-ключ, переопределения и заметки по безопасности канала. |
| [13 — Команды `multisig`](13-multisig.md)          | Участие в multisig-транзакциях Tron: `multisig cosign`, `info`, `broadcast`; форматы файлов и non-goals.                                                 |

## Краткая шпаргалка

```bash
# Сборка
make build                                    # бинарник в build/monujo

# Watch-only (без ключей)
./build/monujo watch balance --addr T...
./build/monujo watch txs     --addr T... --limit 50

# Управление кошельками (offline)
./build/monujo wallet list
./build/monujo wallet create --name myhot
./build/monujo wallet import --name imported --key-hex <64_HEX>
./build/monujo wallet import-external --name custodian --addr T...
./build/monujo wallet check --addr myhot   # только локальный зашифрованный кошелёк (без сети)

# Сетевые операции (парольная фраза нужна только для подписи; баланс — без неё)
#
# --addr: hot balance, wallet check, offline sign (имя в keystore или base58;
#   balance допускает любой адрес; check/sign требуют локальный зашифрованный
#   кошелёк, не import-external).
# --from / --to: hot send-trx, hot send-usdt, offline build (--from/--to могут
#   использовать имена кошельков, включая внешние алиасы, где нужен только
#   адрес; hot send --from должен быть локальным зашифрованным кошельком).
./build/monujo hot balance   --addr myhot
./build/monujo hot balance   --addr T...                 # произвольный адрес, keystore не нужен
./build/monujo hot send-trx  --from funding --to T... --amount 1
./build/monujo hot send-trx  --from myhot --to T... --amount 12.5
./build/monujo hot send-trx  --from myhot --to mycold --amount 12.5   # получатель по имени кошелька
./build/monujo hot send-usdt --from myhot --to T... --amount 10.25

# Тестовая сеть
./build/monujo --network shasta watch balance --addr T...

# Машиночитаемый вывод
./build/monujo --output json watch balance --addr T...

# Air-gap (оффлайн-подпись)
./build/monujo offline build      --from T...cold --to T...rcpt --amount 12.5 --kind trx --out tx.unsigned.json
./build/monujo offline sign       --in tx.unsigned.json --addr cold --out tx.signed.json   # на оффлайн-ПК
./build/monujo offline broadcast  --in tx.signed.json

# Бэкап / перенос всего keystore одним зашифрованным архивом (age + tar).
./build/monujo tool vault pack   --out backup.age
./build/monujo tool vault unpack --in backup.age --out /tmp/restore
```

## Важные предупреждения

- `monujo` — **не аппаратный кошелёк**. Безопасность зависит от:
  доверенной сборки, доверенной машины, прав `0600` на файлы keystore,
  сильной парольной фразы и (опционально) API-ключа TronGrid.
- **Бэкап keystore обязателен.** Скопируйте
  `~/.config/monujo/keystore/<name>.json` на отдельный носитель и
  храните парольную фразу отдельно. Чтобы перенести весь каталог одним
  зашифрованным архивом, используйте `monujo tool vault pack/unpack`
  (age scrypt-mode, без exec во внешний бинарник). Подробная процедура и
  рекомендации — в
  [06-keystore.md → «Резервное копирование keystore»](06-keystore.md#резервное-копирование-keystore).
- **FIPS / compliance.** monujo не сертифицирован как криптомодуль и
  использует Argon2id (не в списке FIPS-approved). Для регулируемых
  сред см.
  [06-keystore.md → «Соответствие стандартам»](06-keystore.md#соответствие-стандартам).
- Никогда не коммитьте файлы keystore, парольные фразы или hex-ключи в
  репозиторий. Переменная `MONUJO_IMPORT_KEY` поддерживается только для
  скриптовых сценариев в контролируемой среде.
- `wallet create --export-private-key --allow-private-key-export`
  один раз печатает приватный ключ в открытом виде — используйте только
  если сознательно принимаете риск.
- Контракт USDT на Shasta (`TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`) —
  тестовый токен, **не настоящий Tether**. Настоящий USDT есть только
  на mainnet.
