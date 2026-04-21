# monujo User Guide

`monujo` is a minimalist Go CLI for working with the **TRON** network:
reading balances and **TRC-20 USDT** transfer history for cold
addresses (public data only, no private keys), managing a **local
encrypted keystore** (create/import), **hot wallet** operations
(online signing and sending TRX/USDT) and **air-gap signing** for
cold storage.

This documentation describes in detail how the binary is organized,
what commands, flags and environment variables it exposes, how keys
are stored, and which typical usage scenarios are supported.

## Contents

| Section                                             | About                                                                                                                                                    |
|-----------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| [01 — Overview and architecture](01-overview.md)    | What the program does, which components it is made of, threat model.                                                                                     |
| [02 — Build and install](02-installation.md)        | Environment requirements, `make build`, binary installation, versions.                                                                                   |
| [03 — Global configuration](03-configuration.md)    | All global flags (`--network`, `--output`, `--http-url`, `--grpc-url`, `--api-key`, `--keystore-dir`) and environment variables.                         |
| [04 — `watch` commands](04-watch-commands.md)       | Detailed reference for `watch balance` and `watch txs`.                                                                                                  |
| [05 — `hot` commands](05-hot-commands.md)           | Detailed reference for `hot balance`, `hot send-trx`, `hot send-usdt` (online operations).                                               |
| [06 — Keystore](06-keystore.md)                     | v2 format (Argon2id) and legacy v1 (scrypt) with auto-migration, KDF/encryption, network metadata, standards compliance, backup.                         |
| [07 — Networks and contracts](07-networks.md)       | Mainnet and Shasta, default endpoints, USDT contract addresses, fee limits.                                                                              |
| [08 — Typical scenarios](08-scenarios.md)           | Step-by-step recipes: cold address checking, hot wallet creation, first TRX to an address, sending USDT, testing on Shasta, scripted usage.                           |
| [09 — Troubleshooting](09-troubleshooting.md)       | Decoding errors and ways to work around them.                                                                                                            |
| [10 — `offline` commands](10-offline-commands.md)   | Air-gap scenario: `offline build` (online), `offline sign` (offline), `offline broadcast` (online); envelope format, HMAC, keystore isolation.           |
| [11 — `wallet` commands](11-wallet-commands.md)     | Keystore file lifecycle: `wallet list`, `wallet create`, `wallet import`, `wallet check` (offline passphrase verification). These commands do not touch the network and work both online and on air-gap.   |

## Quick reference

```bash
# Build
make build                                    # binary at build/monujo

# Watch-only (no keys)
./build/monujo watch balance --addr T...
./build/monujo watch txs     --addr T... --limit 50

# Wallet management (offline)
./build/monujo wallet list
./build/monujo wallet create --name myhot
./build/monujo wallet import --name imported --key-hex <64_HEX>
./build/monujo wallet check --addr myhot   # verify passphrase matches keystore (no network)

# Network operations (passphrase required only for signing; balance needs none)
#
# --addr: hot balance, wallet check, offline sign (keystore name or base58;
#   balance allows any address; check/sign need a local wallet).
# --from / --to: hot send-trx, hot send-usdt, offline build (sender must map
#   to a local wallet for hot sends; build can use cold base58 without keystore).
./build/monujo hot balance   --addr myhot
./build/monujo hot balance   --addr T...                 # arbitrary address, no keystore needed
./build/monujo hot send-trx  --from funding --to T... --amount 1
./build/monujo hot send-trx  --from myhot --to T... --amount 12.5
./build/monujo hot send-trx  --from myhot --to mycold --amount 12.5   # recipient by wallet name
./build/monujo hot send-usdt --from myhot --to T... --amount 10.25

# Test network
./build/monujo --network shasta watch balance --addr T...

# Machine-readable output
./build/monujo --output json watch balance --addr T...

# Air-gap (offline signing)
./build/monujo offline build      --from T...cold --to T...rcpt --amount 12.5 --kind trx --out tx.unsigned.json
./build/monujo offline sign       --in tx.unsigned.json --addr cold --out tx.signed.json   # on the offline PC
./build/monujo offline broadcast  --in tx.signed.json

# Backup / transfer the whole keystore as a single encrypted archive (age + tar).
./build/monujo tool vault pack   --out backup.age
./build/monujo tool vault unpack --in backup.age --out /tmp/restore
```

## Important warnings

- `monujo` is **not a hardware wallet**. Its security depends on: a
  trusted build, a trusted machine, `0600` permissions on keystore
  files, a strong passphrase, and (optionally) a TronGrid API key.
- **Keystore backup is mandatory.** Copy `~/.config/monujo/keystore/<name>.json`
  to a separate medium and store the passphrase separately. To move
  the whole directory as a single encrypted archive use
  `monujo tool vault pack/unpack` (age scrypt-mode, no exec into an
  external binary). Detailed procedure and recommendations are in
  [06-keystore.md → «Резервное копирование keystore»](06-keystore.md#резервное-копирование-keystore).
- **FIPS / compliance.** monujo is not certified as a crypto module
  and uses Argon2id (not FIPS-approved). For regulated environments
  see [06-keystore.md → «Соответствие стандартам»](06-keystore.md#соответствие-стандартам).
- Never commit keystore files, passphrases or hex keys into the
  repository. The `MONUJO_IMPORT_KEY` variable is supported only for
  scripted scenarios in a controlled environment.
- `wallet create --export-private-key` prints the private key in
  plaintext once — use it only if you knowingly accept the risk.
- The USDT contract on Shasta (`TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`)
  is a test token, **not real Tether**. Real USDT exists on mainnet
  only.
