# 01. Overview and architecture

## What is monujo

`monujo` is a Go command-line tool focused on the **TRON** blockchain
and the **USDT (TRC‑20)** token. It solves four independent tasks:

1. **Watch‑only** — fetch public information (TRX/USDT balance,
   TRC‑20 transfer history) for an arbitrary address. No keys are
   needed; the address may belong to anyone (including a cold
   store). Only the public TronGrid HTTP API is used.
2. **Wallet (keystore management)** — create new keys, import
   existing ones, and list local wallets. These commands work only
   with files on disk and **do not touch the network**, so they apply
   equally on an online PC and on an air‑gap machine.
3. **Hot wallet (hot)** — online operations: `hot balance` (no
   password; wallet name or arbitrary address via `--addr`),
   `hot send-trx`, `hot send-usdt`. Signing happens in‑process;
   broadcast goes through TronGrid **HTTPS REST** (gRPC is opt-in).
4. **Air-gap (offline)** — three commands for the classic physically
   isolated-key scenario: `offline build` prepares an unsigned
   transaction on an online PC, `offline sign` on an offline PC adds
   a signature from the local keystore, `offline broadcast` back on
   the online PC submits the result to the network. TRX and USDT are
   supported. Integrity protection when moving the file — mandatory
   visual check of parsed fields plus optional HMAC-SHA256 over a
   shared passphrase.

Solana and BEP are not supported in this version.

## Supported operations

- Read TRX and USDT balances for any address.
- Read USDT (TRC‑20) transfer history — the last N transactions.
- Create a new wallet (generate a random secp256k1 key).
- Import an existing key (64 hex characters).
- List local wallets with metadata.
- Activate a new address (send TRX so the account appears in the
  on‑chain registry).
- Send native TRX.
- Send USDT (TRC‑20) — mainnet uses the canonical Tether contract;
  Shasta uses a test token with similar semantics.
- Air-gap send of TRX and USDT: prepare an unsigned transaction
  online (`offline build`), sign on an isolated machine
  (`offline sign`), broadcast back online
  (`offline broadcast`). See [10 — `offline` commands](10-offline-commands.md).

## Architecture

The codebase is split into several internal packages:

```
cmd/monujo/main.go            # entry point; injects version from ldflags
internal/cli/                 # command and flag parsing (cobra)
  root.go                     # global flags and Execute()
  watch.go                    # watch subcommands
  wallet.go                   # wallet subcommands (keystore lifecycle, no network)
  hot.go                      # hot subcommands (online operations)
  offline.go                  # offline subcommands (build/sign/broadcast)
  passphrase_input.go         # readPassTwice + --generate-passphrase for wallet
  network_confirm.go          # interactive confirmation for --network and --overwrite
internal/config/config.go     # Network, OutputFormat types and Config
internal/app/                 # ties CLI to the domain
  watch_service.go            # formats watch output (table/json)
  hot_service.go              # runs hot operations
  offline_service.go          # ties keystore + envelope + write_client
internal/keystore/            # encryption/decryption, metadata
  keystore.go                 # file format v2 (argon2id + AES‑GCM),
                              # legacy v1 (scrypt), auto-migration v1→v2
  passphrase.go               # password input without echo
internal/domain/              # immutable data types, amount parsing
  asset.go                    # ParseTRXDecimal, ParseUSDTDecimal, formatters
  transfer.go                 # TRC20TransferRow
internal/tron/                # network clients
  read_client.go              # HTTP to TronGrid (account, trc20 history)
  http_write_client.go        # HTTPS Wallet API (write by default)
  write_client.go             # opt-in gRPC (gotron-sdk) + signing
  address.go                  # ValidateBase58Address
  constants.go                # USDT contract addresses
  offline.go                  # build/sign/broadcast helpers outside keystore
internal/transfer/            # JSON envelopes for the air-gap scenario
  envelope.go                 # UnsignedTransferPayload / SignedTransferPayload types
  envelope_seal.go            # outer envelope + scrypt-HMAC for integrity
```

### Execution flow

#### Command `watch ...`

1. `cmd/monujo/main.go` calls `cli.Execute()`.
2. `cli.newRootCmd()` registers global flags and subcommands,
   parses arguments.
3. `cli.loadConfig()` builds a `config.Config` from flags and
   environment variables.
4. `app.NewWatchService(cfg)` creates a `tron.ReadClient`.
5. `WatchService.PrintBalance()` or `PrintTransfers()` performs
   HTTP requests to TronGrid and prints the result in the chosen
   format (`table` or `json`).

#### Command `wallet ...`

1. Same `Execute()` → `loadConfig()`.
2. `wallet list` walks the keystore directory via `keystore.ListWallets()`
   and prints names/addresses/recorded networks — no network calls.
3. `wallet create` / `wallet import` prompt for a passphrase (or
   generate one via `--generate-passphrase`), call
   `keystore.GenerateAndStore()` / `keystore.Store()`, and write the
   current `--network` into the file metadata. Before writing, if a
   file already exists, `confirmWalletOverwrite()` is applied.

#### Command `hot ...`

1. Same `Execute()` → `loadConfig()`.
2. For every `hot` command,
   `confirmWalletNetworkForOperation()` runs first: wallet file
   `networks` metadata is read and, on mismatch with `--network`,
   confirmation is requested in interactive mode (or an error if
   there is no TTY and `--force` was not passed).
3. `hot balance` resolves `--addr` (name or base58) via
   `keystore.ListWallets()` and **does not ask for a password**; then
   calls `WatchService.PrintBalance()`.
4. `hot send-trx` / `hot send-usdt` prompt for a
   passphrase (no echo, via `golang.org/x/term`), then:
   - `keystore.LoadPrivateKey()` decrypts the keystore
     (argon2id → AES‑GCM for v2, or scrypt → AES‑GCM for legacy v1
     with auto-migration to v2), verifies the address.
   - A transaction is built via HTTPS REST TronGrid (default) or opt-in gRPC,
     signed locally, and broadcast on the same write transport.
   - On successful send,
     `keystore.EnsureNetworkRecorded()` is called so the current
     network appears in the file metadata if it was missing.
5. The result is printed (`table` or `json`).

### Where data lives

- **Configuration**: only in the environment and arguments. The
  program never creates yaml/json config files on disk.
- **Encrypted keys**: directory `~/.config/monujo/keystore/` by
  default, overridden via `--keystore-dir` or
  `MONUJO_KEYSTORE_DIR`. One key — one file of the form
  `<name>.json` with mode `0600`.
- **Logs**: none. The program prints results to `stdout`,
  diagnostics and warnings to `stderr`. Nothing is written to
  files except the keystore.

## Threat model

`monujo` is a **software** wallet, not a hardware one. It protects
the key from filesystem leaks (AES‑256‑GCM encryption with a key
derived via Argon2id from the passphrase; old scrypt-v1 files are
read and automatically migrate to v2 on first unlock), but **does
not resist** compromise of the machine itself: the process holds the
decrypted key in memory for the duration of the send; the screen and
clipboard are controlled by the OS.

`monujo` **is not** a certified cryptographic module
(FIPS 140-3 / ISO/IEC 19790) and uses Argon2id, which is not a
FIPS-approved KDF. For environments that require FIPS compliance
the tool is unsuitable. The passphrase is normalized to Unicode NFKC
before the KDF (NIST SP 800-63B §5.1.1.2 / OWASP Password Storage
Cheat Sheet) so the same password yields the same key across macOS,
Linux, and Windows. Details — in
[06-keystore.md → "Standards compliance"](06-keystore.md#standards-compliance).

Assumptions under which the system remains safe:

- The binary was built from trusted sources (you or your CI).
- The machine is not compromised (no keyloggers, foreign root
  access, swap readable by others).
- Keystore files have mode `0600`; the home directory is protected.
- The passphrase is strong enough and is not saved in shell
  history.
- The TronGrid API key (if used) does not leak into public
  repositories.

The program actively resists typical mistakes:

- Zero‑fills passphrases and private-key buffers immediately after
  use.
- Does not use `float64` for amounts (TRX and USDT are handled as
  `int64` or `*big.Int`) to avoid rounding errors.
- Keystore files are read through `os.Root` to prevent symlink
  escapes outside the keystore directory.
- File names are sanitized: only `[a-zA-Z0-9_-]` are allowed;
  other characters are replaced with `_`. An empty name becomes
  `default`.
- Atomic metadata writes (via a temporary file + `rename`) when
  appending a network to the keystore.

## What monujo does NOT do

- Does not manage hardware signers.
- Does not work with TRC‑10, NFTs, or contracts other than the
  explicitly built-in USDT (for TRC‑20 only `transfer(address,uint256)`
  on the canonical USDT contract of the current network is supported,
  including in offline mode).
- Does not do multi‑signature.
- Does not show balances of other TRC‑20 tokens.
- Does not do exchange/conversion.
- Does not send notifications, watch the network in real time, or
  run a daemon.

This is deliberately a minimal “one-shot action” CLI: invoke —
get a result — exit.
