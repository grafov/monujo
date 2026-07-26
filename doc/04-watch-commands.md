# 04. `watch` commands

The `watch` group is **read‑only** operations: the program talks
only to the public TronGrid HTTP API and does not touch private
keys. No files are written. Suitable for:

- checking a cold wallet balance from an online machine;
- viewing incoming/outgoing USDT transfers;
- monitoring scripts.

```text
monujo watch
├── balance      — TRX and USDT for an address
├── txs          — recent USDT (TRC-20) transfers
└── permissions  — owner/witness/active permissions (multisig structure)
```

Global flags (`--network`, `--output`, `--http-url`, `--api-key`,
`--keystore-dir`, `--write-transport`) apply to all subcommands
in the group. `--keystore-dir` is needed by `watch permissions` to
highlight local keys; `--grpc-url` / `--grpc-insecure` are not used
by `watch` (only with `--write-transport grpc` for write commands).

---

## `watch balance`

Requests from TronGrid the TRX balance in sun and the TRC‑20 USDT
balance in minimal units (1 USDT = 1 000 000 minimal units).

### Usage

```bash
monujo watch balance --addr <TRON_BASE58_ADDRESS>
```

### Flags

| Flag     | Type   | Required | Default | Description                                                                                           |
|----------|--------|----------|---------|-------------------------------------------------------------------------------------------------------|
| `--addr` | string | yes      | —       | Tron address in Base58Check (starts with `T`). Validated via `gotron-sdk` before any network request. |

This command has no other local flags.

### What happens

1. The address is validated (`ValidateBase58Address`). On error —
   immediate exit with `invalid Tron address: ...`.
2. An HTTP‑GET is made to `<HTTPBaseURL>/v1/accounts/<addr>` (with
   the `TRON-PRO-API-KEY` header if set).
3. From the response:
   - `data[0].balance` — TRX balance in sun;
   - `data[0].trc20[*]` — map `<contract>: <minimal_units>`.
     The program looks in it for the USDT contract matching
     `--network` (see [07 — Networks](07-networks.md)). If no such
     entry exists, the USDT balance is treated as `0`.
4. The result is formatted per the chosen `--output`.

If the address is not yet activated on‑chain, TronGrid returns an empty
`data: []`. In that case the program prints zero balances (this is
not an error, but the normal state of an unactivated account).

### Example: `table` format

```bash
$ monujo watch balance --addr TXYZ...
Address: TXYZ...
TRX:     12.345678 (12345678 sun)
USDT:    250 (minimal units 250000000)
```

- The TRX amount is printed both as a decimal (6 places of
  precision, see `domain.FormatSunAsTRX`) and in sun.
- For USDT — a decimal representation (6 places) and minimal
  units.

### Example: `json` format

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

JSON fields are stable and can be consumed by scripts:

| Field                | Type    | Description                                                            |
|----------------------|---------|------------------------------------------------------------------------|
| `address`            | string  | Address from the request.                                              |
| `trx_sun`            | integer | Balance in sun.                                                        |
| `trx`                | string  | The same balance as a decimal number (no trailing zeros).              |
| `usdt_minimal_units` | string  | USDT balance in minimal units (string, to avoid losing precision).     |
| `usdt`               | string  | Decimal representation of the same number.                             |

### Possible errors

- `invalid Tron address: ...` — `--addr` is not valid.
- `trongrid accounts: HTTP 4xx: ...` — the TronGrid node returned an error.
  HTTP 401/403 — API-key problem; 429 — rate limit.
- `trongrid: <message>` — successful HTTP status, but the response is marked
  unsuccessful (`success=false` and empty `data`).
- `decode accounts: ...` — unexpected JSON in the response.

---

## `watch txs`

Returns the latest confirmed TRC‑20 USDT transfers for an address
(both directions).

### Usage

```bash
monujo watch txs --addr <TRON_BASE58_ADDRESS> [--limit N]
```

### Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--addr` | string | yes | — | Tron address in Base58Check. |
| `--limit` | int | no | `20` | Maximum transactions. Allowed range **1..200**; values ≤ 0 are replaced with 20, > 200 with 200. |

### What happens

1. Address validation.
2. The USDT contract for `--network` is selected.
3. An HTTP‑GET is made to `<HTTPBaseURL>/v1/accounts/<addr>/transactions/trc20`
   with parameters:
   - `only_confirmed=true` (confirmed only),
   - `limit=<N>`,
   - `contract_address=<USDT address>`.
4. Each record is converted to the internal
   `TRC20TransferRow` model and formatted as `table` or `json`.

Behaviour by transfer “direction”: TronGrid returns all
transfers where the address is the sender **or** the recipient; direction
is visible from the `from` and `to` fields. The `type` field arrives as a
string from TronGrid (e.g. `"Transfer"`); monujo does not interpret it.

### Example: `table` format

```text
$ monujo watch txs --addr TXYZ... --limit 3
2026-04-19T08:11:23Z  abc123...  TFROM... -> TXYZ...   1500000000 USDT  [Transfer]
2026-04-18T10:02:11Z  def456...  TXYZ... -> TANOTHER...  250000000 USDT  [Transfer]
2026-04-17T22:55:01Z  789aaa...  TFROM2... -> TXYZ...    50000000 USDT  [Transfer]
```

Fields in order: time in RFC3339, transaction id, sender,
recipient, amount in **minimal units** (not divided by 10^6),
token symbol, and in square brackets — `type`.

> Note: amounts in `table` output are not divided by 10^6. To get a
> “human” USDT quantity, divide by 1 000 000.

### Example: `json` format

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

Fields:

| Field           | Type   | Description                                                   |
|-----------------|--------|---------------------------------------------------------------|
| `tx_id`         | string | Transaction hash.                                             |
| `time`          | string | Block time in RFC3339 (`time.UnixMilli`).                     |
| `from`          | string | Sender address in Base58.                                     |
| `to`            | string | Recipient address.                                            |
| `value_minimal` | string | Amount in token minimal units.                                |
| `token`         | string | Token symbol (if empty from TronGrid — `USDT` is substituted). |
| `contract`      | string | Token contract address.                                       |
| `type`          | string | Event category from the TronGrid API.                         |

### Possible errors

- `invalid Tron address: ...` — invalid address.
- `trongrid trc20 txs: HTTP 4xx: ...` — API call error.
- `decode trc20 txs: ...` — invalid JSON.
- Empty array (`[]` for JSON, no lines for table) —
  there were no transfers, or the address is not yet activated.

### Notes

- `watch txs` shows **only transfers of the selected USDT contract**,
  not any TRC‑20. For the Shasta test token,
  `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` is used (`Tron.constants.go`), which,
  recall, **is not** real Tether.
- The command does not paginate: one call — one window of at most
  200 latest confirmed records. For deeper history you need
  your own tooling (e.g. direct TronGrid calls with
  `fingerprint` from the response).

---

## `watch permissions`

Shows the account multisig structure: `owner_permission` (id=0),
optional `witness_permission` (id=1), and all `active_permission`
(id=2..9). For each permission it prints id, name, threshold, the list
of allowed contract types (operations bitmap decoded via
`core.Transaction_Contract_ContractType_name`), and the list of signers
`keys: {address, weight}`.

If a key address matches a record in `--keystore-dir`, it is
marked **`[local: <name>]`** for encrypted wallets or
**`[external: <name>]`** for `wallet import-external` records (address
only) — so you can see which keys in the list match your names.

### Usage

```bash
monujo watch permissions --addr <TRON_BASE58_ADDRESS>
```

### Flags

| Flag     | Type   | Required | Default | Description                                                  |
|----------|--------|----------|---------|--------------------------------------------------------------|
| `--addr` | string | yes      | —       | Tron base58 address. Validated locally before the HTTP request. |

Global `--keystore-dir` is used only to resolve
local wallets; the keystore itself is not decrypted.

### Example: `table` format

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

### Example: `json` format

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

In `keys` objects, on a keystore match the `local_wallet` field holds
the record name; for external (`import-external`) aliases,
`"external_wallet": true` is added.

### Possible errors

- `invalid Tron address: ...` — invalid address.
- `trongrid permissions: HTTP 4xx: ...` — TronGrid call error.
- Empty `permissions` (all fields `null`) — the address is not yet
  activated on the network, or it has no custom permissions (equivalent
  to the default 1-of-1 scheme).

See also [13. `multisig` commands](13-multisig.md).
