# 03. Global configuration

Global flags apply to any command and must be specified
**before** the subcommand name (or — thanks to `cobra` — between the
group and the subcommand). Most flags have an environment-variable
alternative; the command-line flag takes precedence over the
environment, and the environment over built-in defaults.

## Summary table

| Flag | Environment variable | Default | Purpose |
|------|---------------------|---------|---------|
| `--network <id>` | — | `mainnet` | Target TRON network: `mainnet` or `shasta`. |
| `--output <fmt>` | — | `table` | Output format: `table` (human-readable) or `json`. |
| `--http-url <url>` | — | per network | Base HTTPS URL (read and write when `rest`). |
| `--write-transport <t>` | — | `rest` | Write transport: `rest` (HTTPS) or `grpc` (opt-in). |
| `--grpc-url <host:port>` | — | per network | gRPC address; only with `--write-transport grpc`. |
| `--grpc-insecure` | — | `false` | Plaintext gRPC (needed for TronGrid and most full nodes). |
| `--allow-insecure-http` | — | `false` | Allow `http://` for REST (insecure; warning on stderr). Without the flag REST requires `https://`. |
| `--api-key <key>` | `MONUJO_TRONGRID_API_KEY` | empty | TronGrid API key for higher request limits. |
| `--keystore-dir <path>` | `MONUJO_KEYSTORE_DIR` | `~/.config/monujo/keystore` | Directory of encrypted wallet files. |
| `--version` | — | — | Show version and git commit embedded at build time. |
| `--help` / `-h` | — | — | `cobra` help for a command or subcommand. |

Additional environment variables with no matching flag:

| Variable | Where used | Purpose |
|----------|------------|---------|
| `MONUJO_IMPORT_KEY` | `wallet import` | Alternative to `--key-hex`: 64 hex characters of a private key. |
| `MONUJO_TRON_E2E` | Only in `make test-integration` | Enables real integration tests against live Tron nodes. Does not affect normal CLI use. |

## Description of each flag

### `--network`

Type: string. Allowed values:

- `mainnet` (default) — the main TRON network.
- `shasta` — the public Shasta testnet.

Case does not matter (`Mainnet`, `SHASTA` are also valid), but in the
keystore file the network is always stored in lowercase.

Error: `unknown network "..." (use mainnet or shasta)`.

This choice determines:

- the TronGrid HTTP endpoint (see [07 — Networks](07-networks.md));
- the gRPC node address;
- the USDT contract address;
- the write into keystore `networks` metadata on `wallet create`/`wallet import`
  and on a successful send via `hot send-*`;
- whether the interactive warning fires when an already used wallet's
  networks do not match.

### `--output`

Type: string. Allowed values:

- `table` (default) — human‑readable text.
- `json` — machine‑readable JSON suitable for `jq`/scripts.

Case does not matter. Error: `unknown output format "..." (use table or json)`.

More on formats — in each of the command sections.

### `--http-url`

Type: string (URL). By default one of the following is substituted:

- `https://api.trongrid.io` (mainnet)
- `https://api.shasta.trongrid.io` (shasta)

Used for `watch balance`, `watch txs`, `hot balance`, and for
write operations when `--write-transport rest` (`/wallet/*`). If
you pass an empty string — the default applies.

Leading and trailing whitespace is trimmed; a trailing `/` is also
trimmed. There is no auto‑prefix of `https://` — specify the URL
in full.

With `--write-transport rest` the scheme must be `https://`. For a
local node with `http://`, pass `--allow-insecure-http`
(warning on stderr).

### `--write-transport`

Type: string. Allowed values:

- `rest` (default) — TronGrid HTTPS Wallet HTTP API (`/wallet/createtransaction`, `/wallet/broadcasthex`, …).
- `grpc` — gotron-sdk to `--grpc-url` (legacy, own node, debugging).

Error: `unknown write transport "..." (use rest or grpc)`.

### `--grpc-url` and `--grpc-insecure`

Only meaningful with `--write-transport grpc`. If under the default
`rest` you explicitly pass `--grpc-url` or `--grpc-insecure`, the CLI
exits with an error (you need to add `--write-transport grpc`).

Defaults:

- `grpc.trongrid.io:50051` (mainnet)
- `grpc.shasta.trongrid.io:50051` (shasta)

Public TronGrid gRPC runs **without TLS**. With
`--write-transport grpc --grpc-insecure` on those hosts the CLI prints
a warning to stderr. For normal TronGrid use, stick with the
default `rest`.

### `--api-key`

Type: string. Passed:

- in HTTP requests to TronGrid in the `TRON-PRO-API-KEY` header;
- in the gRPC client via `gc.SetAPIKey(...)` from `gotron-sdk`.

If the key is set neither by flag nor via `MONUJO_TRONGRID_API_KEY`,
requests go without a key — that works, but the public TronGrid
rate limit is very low (better to get a free key at
[trongrid.io](https://www.trongrid.io)).

### `--keystore-dir`

Type: string (path). Resolution algorithm:

1. If `--keystore-dir` is set, use it (as-is).
2. Else if `MONUJO_KEYSTORE_DIR` is set, use `filepath.Abs`
   of it.
3. Else use `~/.config/monujo/keystore` (`os.UserHomeDir()`).

The directory is created automatically on `wallet create`/`wallet import`
with mode `0700`. If the directory is missing, `wallet list` silently
returns an empty list.

The directory holds wallet JSON files: encrypted local ones (v1/v2)
and optionally **external** address-only records (`wallet import-external`).
Subdirectories and other files are skipped by `wallet list` without errors.

### `--version`

Prints the version as embedded at Makefile build time. Output
format: `<version> (<commit>)`. Without a Makefile build this will be
`dev (unknown)` (see `cmd/monujo/main.go`).

```bash
monujo --version
# v0.1.0 (a1b2c3d)
```

### `--help` / `-h`

Standard `cobra`. At any level — root, group, and a specific
subcommand. See [02 — Installation](02-installation.md#where-to-find-the-programs-own-help).

## Environment variables in more detail

### `MONUJO_TRONGRID_API_KEY`

Alias for `--api-key`. Leading/trailing whitespace is trimmed.
Used only when the flag is not set.

### `MONUJO_KEYSTORE_DIR`

Alias for `--keystore-dir`. Before use the path is made
absolute (`filepath.Abs`).

### `MONUJO_IMPORT_KEY`

Used only in `wallet import`. If `--key-hex` is unset or
empty, the program takes the value from this variable (with `TrimSpace`).
If the variable is also missing — an error is printed:

```
provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key
```

Content — 64 hex characters (optionally with a `0x` prefix,
which will be stripped). Any extra characters or wrong length
cause a validation error; an all-zero key is rejected as
obviously weak.

## What is NOT configurable

- **scrypt parameters** (`N=32768, r=8, p=1`) — fixed in code.
- **Fee limit** for TRC‑20 transactions (`DefaultFeeLimitSun =
  100_000_000` sun, i.e. **100 TRX**) — fixed in code, not
  exposed as an external flag.
- **HTTP/gRPC timeouts** — `30s` for HTTP TronGrid and `60s` for gRPC,
  with no flags.
- **Encrypted keystore version** — written as `2` (argon2id);
  legacy `1` (scrypt) files are also read, with auto-migration on unlock.

## Flag compatibility across commands

The same global flags are valid for any command. Meanwhile,
local flags of a specific subcommand (e.g. `--addr`,
`--from`, `--name`, `--amount`, `--force`) are described only there and
must be specified after the subcommand name.

Full-line example:

```bash
monujo --network shasta --output json --api-key "$KEY" \
       hot send-usdt --from myhot --to T... --amount 10.25 --force
```
