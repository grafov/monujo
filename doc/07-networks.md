# 07. Networks, endpoints, and contracts

## Supported networks

`monujo` works with two TRON networks:

| `--network`              | Purpose                 | HTTP TronGrid                    | gRPC                            | USDT contract                                                       |
|--------------------------|-------------------------|----------------------------------|---------------------------------|---------------------------------------------------------------------|
| `mainnet` (default)      | Main public network     | `https://api.trongrid.io`        | `grpc.trongrid.io:50051`        | `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` (Tether)                       |
| `shasta`                 | Shasta test network     | `https://api.shasta.trongrid.io` | `grpc.shasta.trongrid.io:50051` | `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (test USDT-like token)         |

Endpoints can be overridden with global flags:

- `--http-url` — replaces the TronGrid HTTP base;
- `--grpc-url` — replaces the `host:port` address for gRPC.

This is useful if you want to use your own full Tron node or a TronGrid
proxy on a custom domain. The program does not “know” any other
networks (Nile, private chain) — but via `--http-url`/`--grpc-url` they
can in principle be connected if they are fully API-compatible.

## USDT addresses

The USDT contract is selected by `USDTContractForNetwork` in
`internal/tron/constants.go`:

- **Mainnet** — `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`. This is the
  canonical Tether USDT on TRON, 6 fractional digits. Used in
  `watch balance`, `watch txs` (filter by contract), and
  `hot send-usdt`.
- **Shasta** — `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`. A test
  USDT-like token, **not real Tether**. Convenient for debugging, but
  must not be used in production.

monujo does not show other contracts: `watch balance` and
`watch txs` filter by this single address. If your wallet holds other
TRC-20 tokens, they will not appear in the output.

## Address format

monujo accepts Tron addresses in standard **Base58Check** (starts with
`T`, length 34 characters). Examples:

- mainnet/shasta address: `TLcVUPaYnQp1XzYSZ6h4kRxJZ6Ld9gP1aN`
- USDT contract: `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`

The same private key yields the same address on **both** networks
(mainnet and Shasta). Therefore the keystore is not “bound” to a
network — it only “remembers” the network in `networks` metadata to
warn about a switch.

The address is validated via `gotron-sdk` (`Base58ToAddress`). On error
`invalid Tron address: <reason>` is printed.

## Fees and resources

On TRON payment differs from Ethereum:

- **Bandwidth** — spent on simple transfers (TRX, activation). Each
  account gets a free daily allowance. If it is exhausted, the shortfall
  is paid by “burning” a small amount of TRX.
- **Energy** — spent on contract calls, including `transfer` on
  TRC-20 (USDT). If energy is insufficient, TRX is burned at the node
  rate.

`monujo` does not show a cost estimate before sending. For TRC-20
calls the code hard-codes `feeLimit = 100_000_000 sun` (100 TRX). If
the actual call cost exceeds that, the node returns an error (e.g.
`OUT_OF_ENERGY`).

Before sending USDT make sure:

- The sending address has **TRX** (at least 5–10 TRX in reserve for
  energy/bandwidth).
- The recipient address exists (if not — TronGrid still accepts the
  transaction, the account is created, but in rare cases there may be
  extra costs for account creation).

## Network comparison

| Property           | Mainnet         | Shasta                                          |
|--------------------|-----------------|-------------------------------------------------|
| Real money         | Yes             | No (test)                                       |
| Where to get TRX   | Exchange, P2P   | [Shasta faucet](https://www.trongrid.io/faucet) |
| USDT contract      | Tether          | Test token                                      |
| Node stability     | High            | Sometimes degrades, no SLA                      |
| Used for           | Production ops  | Debugging, learning, CI tests                   |

## Overriding endpoints

### Custom HTTP node

```bash
monujo --http-url https://my-tron-node.example.com \
       watch balance --addr T...
```

The node must support at least the following for **reads** (TronGrid
v1 API):

- `GET /v1/accounts/{addr}` — for `watch balance` / `hot balance`;
- `GET /v1/accounts/{addr}/transactions/trc20` — for `watch txs`.

For **writes** by default (`--write-transport rest`) on the same
`--http-url` the Wallet HTTP methods are needed:
`/wallet/createtransaction`, `/wallet/triggersmartcontract`,
`/wallet/broadcasthex`, `/wallet/getsignweight`.

The API key is passed in the `TRON-PRO-API-KEY` header. If your node
does not use keys, `--api-key` can be omitted.

### Custom gRPC node (opt-in)

```bash
monujo --write-transport grpc --grpc-url my-tron-grpc.example.com:50051 \
       --grpc-insecure \
       hot send-trx --from myhot --to T... --amount 1
```

Usually `--grpc-insecure` (plaintext gRPC) is needed. mTLS is not yet
supported via flags.

When present, the API key is passed in gRPC metadata via gotron-sdk.

### TronGrid API key

A free TronGrid key substantially raises the rate limit (especially for
`watch txs`). Register at [trongrid.io](https://www.trongrid.io). After
obtaining one:

```bash
export MONUJO_TRONGRID_API_KEY="<your key>"
monujo watch txs --addr T...
```

or via flag:

```bash
monujo --api-key "<key>" watch balance --addr T...
```

## What is written to wallet metadata

On `wallet create`, `wallet import`, and a successful `hot send-trx` or
`hot send-usdt`, the current `--network` is saved in
`EncryptedWalletFile.networks`. This is so that on a later run with a
different `--network` the user sees a warning and does not accidentally
act on the wrong network (for example, run test operations on mainnet,
or send real money to Shasta).

See [06 — Keystore](06-keystore.md#networks-metadata) for details.
