# 12. Network activity and traffic security

This section lists **only those `monujo` subcommands** that initiate
outbound network requests, with protocol, default domains, and paths
or gRPC methods. For flags and environment variables see
[03. Global configuration](03-configuration.md); for networks and
USDT contract addresses — [07. Networks and contracts](07-networks.md).

## Commands with no network access

The following subcommands **do not** open connections to the Tron API
and do not contact other remote hosts (local disk and user input
only):

- `wallet list`, `wallet create`, `wallet import`, `wallet check`
- `offline sign`
- `tool genpass`
- `tool vault pack`, `tool vault unpack`

## Table: commands with network traffic

Base URLs depend on `--network` (`mainnet` or `shasta`). They can be
replaced with `--http-url` pointing at a compatible TronGrid or your
own full node.

By default **writes** (building and broadcasting transactions) go
through **HTTPS REST** (`--write-transport rest`). Optionally:
`--write-transport grpc` and `--grpc-url` (see the gRPC section
below).

| Command                         | Protocol (default)      | Domain (mainnet / shasta)            | Endpoints                                                                 |
|---------------------------------|-------------------------|--------------------------------------|---------------------------------------------------------------------------|
| `watch balance`                 | HTTPS (REST, TronGrid)  | `api.trongrid.io` / `api.shasta.trongrid.io` | `GET /v1/accounts/{address}`                                              |
| `watch txs`                     | HTTPS (REST, TronGrid)  | same                         | `GET /v1/accounts/{address}/transactions/trc20`                           |
| `watch permissions`             | HTTPS (REST, TronGrid)  | same                         | `GET /v1/accounts/{address}` (owner/witness/active permission fields)      |
| `hot balance`                   | HTTPS (REST, TronGrid)  | same                         | same requests as `watch balance`                                          |
| `hot send-trx`                  | HTTPS (REST)            | same                         | `POST /wallet/createtransaction`, `POST /wallet/broadcasthex`             |
| `hot send-usdt`                 | HTTPS (REST)            | same                         | `POST /wallet/triggersmartcontract`, `POST /wallet/broadcasthex`          |
| `offline build` (`--kind=trx`)  | HTTPS (REST)            | same                         | `POST /wallet/createtransaction` (no broadcast)                            |
| `offline build` (`--kind=usdt`) | HTTPS (REST)            | same                         | `POST /wallet/triggersmartcontract` (no broadcast)                         |
| `offline broadcast`             | HTTPS (REST)            | same                         | `POST /wallet/broadcasthex`                                               |
| `multisig info`                 | HTTPS (REST)            | same                         | `POST /wallet/getsignweight`                                              |
| `multisig broadcast`            | HTTPS (REST)            | same                         | `POST /wallet/getsignweight`, `POST /wallet/broadcasthex`                 |

### Full HTTPS URLs

With default values the HTTP base is:

- mainnet: `https://api.trongrid.io`
- shasta: `https://api.shasta.trongrid.io`

Paths from the table are appended, for example:
`https://api.trongrid.io/v1/accounts/T...`.

### TronGrid API key

If `--api-key` or the `MONUJO_TRONGRID_API_KEY` environment variable is
set, the `TRON-PRO-API-KEY` header is added to **HTTPS** requests.

## Optional gRPC transport (`--write-transport grpc`)

For compatibility with `java-tron` gRPC or for debugging you can
explicitly select gRPC via gotron-sdk:

| Command (same as above) | Protocol (opt-in) | Default host                               | RPC (Wallet API)                    |
|-------------------------|-------------------|--------------------------------------------|-------------------------------------|
| `hot send-*`, `offline build/broadcast`, `multisig info/broadcast` | gRPC (often **plaintext**) | `grpc.trongrid.io:50051` / `grpc.shasta.trongrid.io:50051` | `CreateTransaction2`, `TriggerContract`, `BroadcastTransaction`, `GetTransactionSignWeight` |

Flags:

- `--write-transport grpc` — enable gRPC instead of REST.
- `--grpc-url` — `host:port` of a full node.
- `--grpc-insecure` — plaintext gRPC (required for public TronGrid and most local full nodes).

With `--write-transport grpc --grpc-insecure` against known TronGrid
hosts the CLI prints a **warning to stderr**: public TronGrid gRPC
does not use TLS; the default `--write-transport rest` is preferable.

## Overriding nodes

- `--http-url` — base for REST (reads and writes when `rest`).
- `--grpc-url` — only with `--write-transport grpc`.

Both parameters may point at a **different domain** if the node
implements the same HTTP and (when needed) Wallet gRPC interface.

## Risks and hard configuration

1. **HTTPS REST by default.** Reads and writes via TronGrid use
   **HTTPS** on `api.*.trongrid.io`. gRPC is an explicit opt-in only.
   `http://` for REST is forbidden without `--allow-insecure-http`.

2. **Plaintext gRPC.** TronGrid and a typical `java-tron` on `:50051`
   accept gRPC **without TLS**. Do not enable `--write-transport grpc`
   on an untrusted network without understanding the MITM risk. Flags
   `--grpc-url` / `--grpc-insecure` without `--write-transport grpc`
   are an error (no silent fallback to REST).

3. **Trust in the node.** The node sees addresses, amounts, and signed
   transactions.

4. **No third-party telemetry.** Outbound traffic is limited to the
   configured HTTP/gRPC endpoints (see the tables above).
