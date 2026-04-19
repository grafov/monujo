# monujo

Minimal **Go CLI** for **Tron / TRC-20 / USDT**: watch-only queries for cold addresses (no keys), **hot wallet** send flows with a **local encrypted keystore**, and an **air-gapped offline signer** (`monujo offline build|sign|broadcast`) for cold-storage workflows. Solana/BEP are out of scope.

## Build

```bash
make build
# binary: build/monujo
```

Requires Go 1.24+ (see `go.mod`).

## Configuration

| Env / flag                               | Purpose                                                                                                                                                                                           |
|------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `MONUJO_TRONGRID_API_KEY` / `--api-key`  | Optional TronGrid API key (recommended for higher rate limits).                                                                                                                                   |
| `MONUJO_KEYSTORE_DIR` / `--keystore-dir` | Hot wallet files directory (default: `~/.config/monujo/keystore`).                                                                                                                                |
| `MONUJO_IMPORT_KEY`                      | Hex private key for `hot import` if `--key-hex` is omitted.                                                                                                                                       |
| `--network`                              | `mainnet` (default) or `shasta` (testnet; USDT contract differs).                                                                                                                                 |
| `--output`                               | `table` or `json`.                                                                                                                                                                                |
| `--http-url` / `--grpc-url`              | Override TronGrid HTTP base URL and gRPC `host:port`.                                                                                                                                             |
| `--force`                                | On `hot balance`, `hot send-trx`, `hot send-usdt`, `hot activate` only: skip the interactive confirmation when `--network` differs from networks already recorded in the wallet file (see below). |

## Watch-only (cold address, online PC)

No private keys; uses public TronGrid HTTP APIs.

```bash
./build/monujo watch balance --addr <TRON_BASE58_ADDRESS>
./build/monujo watch txs   --addr <TRON_BASE58_ADDRESS> [--limit 20]
```

## Hot wallet (online PC)

Each wallet file stores optional **`networks`** metadata: which Tron networks (`mainnet`, `shasta`) this wallet has already been used with. **`hot create`** and **`hot import`** record the current `--network`. After a successful **`hot send-trx`** or **`hot send-usdt`**, the current network is appended if missing.

If you run **`hot balance`**, **`hot send-trx`**, **`hot send-usdt`**, or **`hot activate`** with a `--network` that is **not** in that list while the list is **non-empty**, the CLI prints the previously recorded networks and asks for confirmation before the passphrase prompt. In scripts or without a TTY, pass **`--force`** to acknowledge the mismatch, or run interactively and answer `y`/`yes`.

List wallets (names, addresses, and recorded networks):

```bash
./build/monujo hot list [--output json]
```

1. **Create** a new wallet locally (random secp256k1 key, encrypted keystore). The Tron address is derived from the key; it is valid on both mainnet and Shasta — pick `--network` when you query or send; it is stored in the wallet file metadata.

```bash
./build/monujo hot create --name myhot [--network shasta]
# optional: --export-private-key (prints raw hex once; unsafe)
# JSON with optional key field: --output json --export-private-key
```

2. **Import** a 64-hex-character secp256k1 private key into an encrypted file (scrypt + AES-GCM):

```bash
./build/monujo hot import --name myhot --key-hex <64_HEX_CHARS>
# passphrase prompts; or set MONUJO_IMPORT_KEY instead of --key-hex
```

3. **Activate** a new address on-chain: Tron accounts become visible after the first incoming TRX. Send a small amount from an already-funded hot wallet:

```bash
./build/monujo hot activate --from funding --to <NEW_ADDR> --amount 1 --network shasta
```

(On testnet, you can use a [Shasta faucet](https://www.trongrid.io/faucet) instead of `activate` if you prefer.)

4. **Balance** (decrypts locally, then same public queries as watch):

```bash
./build/monujo hot balance --name myhot
```

5. **Send TRX** (native, amount in sun/TRX decimals):

```bash
./build/monujo hot send-trx --name myhot --to <ADDR> --amount 12.5
```

6. **Send USDT** (TRC-20; mainnet contract is fixed in code):

```bash
./build/monujo hot send-usdt --name myhot --to <ADDR> --amount 10.25
```

Hot sends use **gRPC** to a Tron full node (default `grpc.trongrid.io:50051` on mainnet) and sign in-process. You need enough **TRX** for bandwidth/energy; USDT transfers consume energy — fund the hot address accordingly.

## Security notes

- This is **not** a hardware wallet. Threat model: trusted build, trusted machine, encrypted keystore file permissions (`0600`), strong passphrase, and optional TronGrid API key for reliability.
- **Do not** commit keystores, passphrases, or hex keys. `hot import` supports env `MONUJO_IMPORT_KEY` for scripting — prefer interactive `--key-hex` or env only in controlled environments. Avoid `hot create --export-private-key` unless you accept the risk of exposing the key in the terminal or JSON output.
- Amount parsing avoids `float64` for token math; USDT/TRX amounts are handled as integers / decimals via strings.
- USDT contract addresses: **mainnet** `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`; **Shasta** uses the test token address baked into `internal/tron/constants.go` (not official mainnet Tether).

## Offline signing (air-gap)

For cold-storage workflows the keystore can live exclusively on an **offline** machine. Three commands carry a transfer across the air gap:

```bash
# Online PC #1 (no keystore needed): fetch a fresh ref-block, build unsigned tx
./build/monujo offline build \
    --from-address T...cold --to T...recipient \
    --amount 12.5 --kind trx \
    --out tx.unsigned.json \
    [--hmac-passphrase-env MONUJO_HMAC]

# Offline PC (keystore + private key live HERE only): sign the file
./build/monujo offline sign \
    --in tx.unsigned.json --name cold --out tx.signed.json \
    [--hmac-passphrase-env MONUJO_HMAC]

# Online PC #2 (no keystore needed): broadcast
./build/monujo offline broadcast --in tx.signed.json
```

Both TRX (`--kind trx`) and USDT (`--kind usdt`) are supported. Tron's `ref_block` / `expiration` give a ~60 second window between `build` and `broadcast`; an expired file must be rebuilt online.

Two layers protect the file in transit:

- **Visual confirmation.** Before signing and before broadcasting, monujo recomputes all human-readable fields (`from`, `to`, `amount`, `contract`, `expiration`, `tx_id`) from the embedded `raw_data` and prints them along with the file SHA-256, then asks `[y/N]` (use `--yes` for scripts).
- **Optional HMAC-SHA256.** Pass `--hmac-passphrase` (or `--hmac-passphrase-env`) on both ends; the key is derived via scrypt and the envelope is rejected on mismatch.

See [docs/10-offline-commands.md](docs/10-offline-commands.md) for a step-by-step walk-through and the on-disk envelope format.

## License

See repository root / upstream dependencies (gotron-sdk is LGPL/Apache mix; go-ethereum LGPL). Ensure compliance if you redistribute binaries.
