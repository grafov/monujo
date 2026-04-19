# 08. Типовые сценарии

В этом разделе — пошаговые рецепты для основных задач. Все примеры
исходят из того, что бинарник лежит как `./build/monujo` (после
`make build`); если вы установили его в `PATH`, замените на просто
`monujo`.

## Сценарий 1. Проверить баланс холодного адреса

Подходит для аудита холодного хранилища с обычной онлайн‑машины.
Никаких ключей не требуется.

```bash
./build/monujo watch balance --addr TXYZ...
```

Если хочется машинно‑обрабатываемых данных:

```bash
./build/monujo --output json watch balance --addr TXYZ... | jq '.usdt'
```

Дополнительно — последняя история переводов USDT (до 50 записей):

```bash
./build/monujo watch txs --addr TXYZ... --limit 50
```

С API‑ключом TronGrid рейт‑лимит будет существенно выше:

```bash
export MONUJO_TRONGRID_API_KEY="..."
./build/monujo watch txs --addr TXYZ... --limit 200
```

## Сценарий 2. Создать новый горячий кошелёк и активировать его

```bash
# 1. Создаём кошелёк (mainnet по умолчанию).
./build/monujo hot create --name myhot
# New keystore passphrase: ******
# Confirm passphrase: ******
# Created wallet "myhot" -> TLcVUPa...
```

Вариант с одноразовой печатью приватного ключа (нужно, если хотите
переложить кошелёк в другой инструмент):

```bash
./build/monujo hot create --name myhot --export-private-key
# WARNING: printing private key is unsafe; ...
# Private key (hex): 7b8a...
```

Активация — в TRON аккаунт должен получить первую TRX:

```bash
./build/monujo hot activate \
  --from funding --to TLcVUPa... --amount 1
# (просит парольную фразу funding)
# Broadcast OK ...
```

После этого `watch balance` и `hot balance` для нового адреса начнут
показывать его осмысленно (раньше отвечали нулями).

## Сценарий 3. Импорт существующего ключа

```bash
./build/monujo hot import \
  --name imported --key-hex 7b8a...c9d0
# Парольная фраза дважды.
```

Безопаснее не указывать ключ в командной строке (он остаётся в
истории shell). Используйте переменную:

```bash
read -rs MONUJO_IMPORT_KEY    # ввод без эха
export MONUJO_IMPORT_KEY
./build/monujo hot import --name imported
unset MONUJO_IMPORT_KEY
history -d -1 -2              # удалить недавние строки истории (zsh/bash)
```

## Сценарий 4. Проверить баланс именованного кошелька

```bash
./build/monujo hot balance --name myhot
# Passphrase: ******
# TRX: ...
# USDT: ...
```

В отличие от `watch balance` здесь не нужно помнить адрес — он
извлекается из расшифрованного keystore. Минус — требуется парольная
фраза (программа дешифрует ключ только чтобы убедиться в целостности
файла, после чего сразу затирает его в памяти).

## Сценарий 5. Отправить TRX

```bash
./build/monujo hot send-trx \
  --name myhot --to TXYZ... --amount 12.5
# Passphrase: ...
# Broadcast OK
# Network: mainnet
# TxID:    abc123...
```

Если у `myhot` ещё нет TRX, broadcast завершится ошибкой. Проверить
баланс перед отправкой:

```bash
./build/monujo hot balance --name myhot
```

## Сценарий 6. Отправить USDT

```bash
./build/monujo hot send-usdt \
  --name myhot --to TXYZ... --amount 10.25
```

Условия:

- На `myhot` есть **TRX** (в идеале хотя бы 5–10 TRX про запас на
  energy/bandwidth);
- На `myhot` есть USDT в нужном объёме;
- Контракт USDT в текущей `--network` доступен (mainnet — Tether,
  Shasta — тестовый токен).

## Сценарий 7. Тестирование на Shasta

Сначала «прокачаем» новый адрес TRX через
[Shasta‑faucet](https://www.trongrid.io/faucet) (без активации
вручную):

```bash
./build/monujo --network shasta hot create --name testhot
# Адрес TLcVUPa...
# Скопируйте его и запросите TRX на faucet через браузер.
```

Проверка после поступления:

```bash
./build/monujo --network shasta hot balance --name testhot
```

Отправка тестовых TRX/USDT:

```bash
./build/monujo --network shasta hot send-trx \
  --name testhot --to TXYZ... --amount 1

./build/monujo --network shasta hot send-usdt \
  --name testhot --to TXYZ... --amount 1
```

При первом запуске `hot send-*` после `hot create` сеть совпадает
(в файле уже записан `shasta`), подтверждения не будет.

## Сценарий 8. Один и тот же кошелёк на mainnet и Shasta

Поскольку приватный ключ дает один и тот же адрес в обеих сетях,
один файл keystore обслуживает обе сети. Метаданные `networks` нужны
только для того, чтобы предупреждать вас о смене.

При первой попытке использовать в новой сети:

```bash
./build/monujo --network shasta hot balance --name myhot
# Warning: wallet "myhot" was previously used on network(s): mainnet
# The current operation uses network "shasta", which is not in that list.
# Continue? [y/N]: y
# Passphrase: ...
# ...
```

После успешного `hot send-*` `shasta` будет добавлен в `networks`, и
предупреждение исчезнет.

## Сценарий 9. Скриптовое использование без TTY

В неинтерактивных режимах:

- Парольная фраза будет считана из STDIN (читается строка), но без
  скрытия ввода (в `passphrase.go` это запасной путь).
- Подтверждение сети **не сработает интерактивно**. Если файл уже
  записан в одну сеть, а вы запускаете в другой, и не задали
  `--force`, программа упадёт с
  `network mismatch: use --force ...`.
- Решение — добавлять `--force`, когда вы уверены, что сеть выбрана
  правильно.

Пример скрипта, читающего пароль из переменной окружения:

```bash
#!/usr/bin/env bash
set -euo pipefail

: "${MONUJO_PASS:?need MONUJO_PASS}"

printf '%s\n' "$MONUJO_PASS" | \
  ./build/monujo --output json --network shasta \
    hot send-trx --name testhot --to TXYZ... --amount 1 --force \
  | jq '.tx_id'
```

> **Внимание**: программа дважды читает строку при `hot create` и
> `hot import` (новая фраза + подтверждение). Подавайте её в STDIN
> два раза подряд:
> ```bash
> printf '%s\n%s\n' "$MONUJO_PASS" "$MONUJO_PASS" | \
>   ./build/monujo hot create --name myhot
> ```

## Сценарий 10. Перенос keystore между машинами

1. На исходной машине найдите файл: `~/.config/monujo/keystore/<имя>.json`
   (или то, что задано `--keystore-dir`).
2. Скопируйте его на целевую машину **по защищённому каналу** (scp/rsync
   через ssh, USB‑накопитель, gpg‑зашифрованный архив).
3. Поместите в `~/.config/monujo/keystore/` (или укажите свой каталог).
4. Установите/соберите monujo на целевой машине.
5. Используйте обычные команды: парольная фраза та же.

При желании можно сначала переименовать файл — главное, чтобы имя
файла совпадало с тем, как вы будете указывать в `--name` (с учётом
санитизации `safeName`).

## Сценарий 11. Список всех кошельков на машине

```bash
./build/monujo hot list
```

Машинно:

```bash
./build/monujo --output json hot list \
  | jq '.wallets[] | {name, address, networks}'
```

Удобно, чтобы быстро освежить, какие у вас локальные ключи и какие
сети они уже видели.

## Сценарий 12. Проверка работоспособности после установки

```bash
# 1. Версия — печатается то, что вшито Makefile через -ldflags.
./build/monujo --version

# 2. Help для всех ключей.
./build/monujo --help
./build/monujo watch --help
./build/monujo hot --help

# 3. Watch‑запрос к публичному адресу (например, USDT‑контракт).
./build/monujo watch balance --addr TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t
```

Если последний шаг возвращает осмысленные баланс/нули — сеть
доступна, бинарник работает.
