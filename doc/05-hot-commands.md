# 05. `hot` commands

The `hot` group covers **online operations** on an existing wallet from
the local keystore: checking balance, sending TRX and USDT. All
commands in this group make network calls (HTTPS TronGrid: reads via
`/v1/*`, writes via `/wallet/*` by default).

> Keystore file lifecycle (create, import, list) lives in the separate
> [`monujo wallet`](11-wallet-commands.md) group and does not make
> network calls; the same files are later used by both `hot` and
> `offline sign`.

```text
monujo hot
├── balance       — TRX and USDT balance by wallet name or address (no password)
├── send-trx      — send TRX
└── send-usdt     — send USDT (TRC-20)
```

Global flags (`--network`, `--output`, `--api-key`,
`--keystore-dir`, `--http-url`, `--write-transport`, `--grpc-url`,
`--grpc-insecure`, `--allow-insecure-http`) apply to all subcommands.
See [03 — Configuration](03-configuration.md) for details.

## Common principles

- **`--addr` and `--from` / `--to`**. For `hot balance` the subject is
  set with **`--addr`**: a keystore file name (without `.json`) or a
  Tron base58 address. For `hot send-trx` and `hot send-usdt` the
  sender is **`--from`**, the recipient is **`--to`**. The value is
  either a keystore name or a base58 address (T..., 34 characters).
  Recognition is automatic: anything that passes base58 validation is
  treated as an address, otherwise as a name; resolution goes through
  `keystore.ListWallets` without decryption.
    - **`--addr` in `hot balance`** accepts an arbitrary address: the
      command reads balances from the network and no keystore is
      required.
    - **`--from` in `hot send-trx`/`hot send-usdt`** accepts an address
      only if it matches a **local** (not external) wallet in the
      keystore. An external entry name (`import-external`) yields an
      error: sending requires a private key.
    - **`--to`** accepts any valid base58 address or a keystore entry
      name (local or **external** — address only).
- **Passphrase input**. Commands that need a signature (`send-trx`,
  `send-usdt`) prompt for a passphrase via TTY without echo. `hot
  balance` does **not** ask for a password: it reads the address from
  the file's open metadata.
- **Wiping secrets**. The passphrase and private key material are
  zeroed (`ZeroPassphrase`, `Wipe`) immediately after use.
- **Network confirmation**. If the wallet file already has a non-empty
  `networks` list and the current `--network` is not in it, a warning
  is printed and confirmation `[y/N]` is requested before the
  operation. To skip this confirmation in scripts — `--force`. Without
  a TTY and without `--force`, the error `network mismatch: use
  --force to proceed without a terminal, or run interactively to
  confirm` is returned.
- **Writing metadata**. After a successful TRX or USDT send, the
  program appends the current `--network` to the wallet file's
  `networks` array (without re-encrypting; only metadata changes,
  via atomic replace with `rename`).

---

## `hot balance`

Shows TRX and USDT balance for a keystore **name** or a **raw** base58
address. For a name, the address is read from file metadata (including
external `import-external` entries); **no passphrase is requested**.

### Usage

```bash
monujo hot balance --addr <NAME>
monujo --network shasta hot balance --addr testhot --force
```

### Flags

| Flag      | Type   | Required | Default | Description                                                                                      |
|-----------|--------|----------|---------|--------------------------------------------------------------------------------------------------|
| `--addr`  | string | yes      | —       | Keystore wallet name OR Tron base58 address (arbitrary).                                         |
| `--force` | bool   | no       | `false` | Skip interactive confirmation if the current `--network` is not recorded in the file's `networks`. |

### What happens

1. The `--addr` value is resolved via `keystore.ListWallets` (name or
   base58 address).
2. **If** it is a name or an address found in the keystore:
   - on mismatch between the current `--network` and the file's
     `networks` array, a warning is printed and (without `--force`)
     confirmation is requested — same as for local wallets, **including
     external** entries with a non-empty `networks`;
   - then balances are requested for the stored address (for an
     external entry the same read-only path is used as for a raw
     address).
3. **If** it is an arbitrary base58 not in the keystore — the
   `networks` check step is **skipped**, and an HTTP balance request
   is made immediately.

> Key decryption for `hot balance` is **not** performed for either
> local or external entries.

### Example

```text
$ monujo hot balance --addr myhot
Address: TLcVUPa...
TRX:     50.123456 (50123456 sun)
USDT:    100 (minimal units 100000000)
```

The JSON structure is the same as for `watch balance` (see
[04](04-watch-commands.md#example-json-format)).

---

## `hot send-trx`

Sends native TRX from a local wallet to any address. The first inbound
TRX to a new address on Tron makes the account visible to the read API
— a normal `send-trx` with a small amount to the target address (or a
faucet on Shasta) is enough for that.

### Usage

```bash
monujo hot send-trx --from <NAME> --to <ADDR> --amount <TRX>
```

### Flags

| Flag       | Type   | Required | Default | Description                                                                                     |
|------------|--------|----------|---------|-------------------------------------------------------------------------------------------------|
| `--from`   | string | yes      | —       | Source wallet name OR its base58 address (must match a local keystore file).                    |
| `--to`     | string | yes      | —       | Recipient: base58 address OR local wallet name.                                                 |
| `--amount` | string | yes      | —       | TRX amount (decimal number, up to 6 fractional digits).                                         |
| `--force`  | bool   | no       | `false` | Skip interactive network confirmation.                                                          |

### What happens

1. Network confirmation (see common principles above).
2. Password prompt.
3. Amount parsing: `domain.ParseTRXDecimal`. Up to 6 digits after the
   decimal point are allowed. Negative numbers, empty string, extra
   characters, and `int64` overflow cause an error.
4. Recipient address validation.
5. Keystore decryption.
6. Transaction construction via HTTPS REST (`/wallet/createtransaction`
   by default; opt-in gRPC — `gotron-sdk` `Transfer`), ECDSA signing,
   broadcast (`/wallet/broadcasthex` or gRPC `Broadcast`).
7. If broadcast returns a non-zero code — error `broadcast: <message>`.
8. Append `--network` to metadata.

### Example: `table` format

```text
$ monujo hot send-trx --from myhot --to TXYZ... --amount 12.5
Passphrase: ********
Broadcast OK
Network: mainnet
TxID:    abc123...
From:    TLcVUPa...
To:      TXYZ...
TRX: 12.5
```

### Example: `json` format

```json
{
  "tx_id": "abc123...",
  "from": "TLcVUPa...",
  "to": "TXYZ...",
  "kind": "TRX",
  "amount": "12.5",
  "network": "mainnet"
}
```

### Notes

- The transaction ID is computed locally as
  `sha256(proto.Marshal(tx.GetRawData()))` — Tron uses exactly this
  hash as `txID`. Therefore `tx_id` is available immediately after a
  successful broadcast, even before inclusion in a block.
- Tron pays either from **bandwidth** (free within the daily limit) or
  by “burning” TRX if bandwidth is exhausted. monujo does not request
  an explicit fee estimate; for native transfers this is usually safe.
  If the address has no TRX at all, broadcast will fail.
- On success the current `--network` is added to the file metadata (if
  not already recorded). If the file could not be updated for some
  reason, the program still reports a successful broadcast and prints
  a metadata error:

  ```
  broadcast ok (tx <id>) but could not update wallet networks metadata: <reason>
  ```

---

## `hot send-usdt`

Sends USDT (TRC-20) from a local wallet to any address. Uses the
contract selected by `--network`:

- mainnet: `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` (Tether),
- Shasta: `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (test token,
  **not real** USDT).

### Usage

```bash
monujo hot send-usdt --from <NAME> --to <ADDR> --amount <USDT>
```

### Flags

| Flag       | Type   | Required | Default | Description                                                               |
|------------|--------|----------|---------|---------------------------------------------------------------------------|
| `--from`   | string | yes      | —       | Source wallet name OR its base58 address (must match a local keystore file). |
| `--to`     | string | yes      | —       | Recipient: base58 address OR local wallet name.                           |
| `--amount` | string | yes      | —       | USDT amount (decimal number, up to 6 fractional digits). Must be positive. |
| `--force`  | bool   | no       | `false` | Skip interactive network confirmation.                                    |

### What happens

1. Network confirmation.
2. Password prompt.
3. Amount parsing: `domain.ParseUSDTDecimal` → `*big.Int` of minimal
   units. Up to 6 digits after the decimal point. The amount must be
   strictly positive (`amount must be positive` otherwise).
4. Recipient address validation.
5. Keystore decryption.
6. REST: `POST /wallet/triggersmartcontract` (USDT `transfer(address,uint256)`,
   `fee_limit=100_000_000` sun) or opt-in gRPC `TRC20Send`.
7. ECDSA signing, broadcast.
8. Append `--network` to metadata.

### Example: `table` format

```text
$ monujo hot send-usdt --from myhot --to TXYZ... --amount 10.25
Passphrase: ********
Broadcast OK
Network: mainnet
TxID:    def456...
From:    TLcVUPa...
To:      TXYZ...
USDT: 10.25 USDT
```

### Example: `json` format

```json
{
  "tx_id": "def456...",
  "from": "TLcVUPa...",
  "to": "TXYZ...",
  "kind": "USDT",
  "amount": "10.25 USDT",
  "network": "mainnet"
}
```

### Notes

- TRC-20 transfers consume **energy**. If the address lacks enough
  staked `energy`, the shortfall is covered by burning TRX, but not
  more than `feeLimit = 100 TRX`. If your TRX balance is below what is
  needed, broadcast fails with `OUT_OF_ENERGY` or `OUT_OF_TIME` from
  the node.
- The fee limit (`100 TRX`) is hard-coded (`config.defaultFeeLimitSun`).
  In the current version it cannot be changed via a flag.
- On Shasta the test USDT contract may behave differently from real
  Tether (decimals, events, methods — the program assumes the
  `transfer(address,uint256)` standard and 6 fractional digits). Before
  using on Shasta, make sure this is the contract for which that call
  is valid.

---

## Summary of all `hot` flags

| Subcommand  | Local flags                                         | Asks for password |
|-------------|-----------------------------------------------------|-------------------|
| `balance`   | `--addr`, `--force`                                 | no                |
| `send-trx`  | `--from`, `--to`, `--amount`, `--force`             | yes               |
| `send-usdt` | `--from`, `--to`, `--amount`, `--force`             | yes               |
