# monujo

Minimal **Go CLI** for **Tron / TRC-20 / USDT**: watch-only queries for cold addresses (no keys) and **hot wallet** send flows with a **local encrypted keystore**. Solana/BEP are out of scope for this version; the layout leaves room for a future offline-signing step.

## Build

```bash
make build
# binary: build/monujo
```

Requires Go 1.24+ (see `go.mod`).

## Configuration

| Env / flag | Purpose |
|------------|---------|
| `MONUJO_TRONGRID_API_KEY` / `--api-key` | Optional TronGrid API key (recommended for higher rate limits). |
| `MONUJO_KEYSTORE_DIR` / `--keystore-dir` | Hot wallet files directory (default: `~/.config/monujo/keystore`). |
| `MONUJO_IMPORT_KEY` | Hex private key for `hot import` if `--key-hex` is omitted. |
| `--network` | `mainnet` (default) or `shasta` (testnet; USDT contract differs). |
| `--output` | `table` or `json`. |
| `--http-url` / `--grpc-url` | Override TronGrid HTTP base URL and gRPC `host:port`. |

## Watch-only (cold address, online PC)

No private keys; uses public TronGrid HTTP APIs.

```bash
./build/monujo watch balance --addr <TRON_BASE58_ADDRESS>
./build/monujo watch txs   --addr <TRON_BASE58_ADDRESS> [--limit 20]
```

## Hot wallet (online PC)

1. **Import** a 64-hex-character secp256k1 private key into an encrypted file (scrypt + AES-GCM):

```bash
./build/monujo hot import --name myhot --key-hex <64_HEX_CHARS>
# passphrase prompts; or set MONUJO_IMPORT_KEY instead of --key-hex
```

2. **Balance** (decrypts locally, then same public queries as watch):

```bash
./build/monujo hot balance --name myhot
```

3. **Send TRX** (native, amount in sun/TRX decimals):

```bash
./build/monujo hot send-trx --name myhot --to <ADDR> --amount 12.5
```

4. **Send USDT** (TRC-20; mainnet contract is fixed in code):

```bash
./build/monujo hot send-usdt --name myhot --to <ADDR> --amount 10.25
```

Hot sends use **gRPC** to a Tron full node (default `grpc.trongrid.io:50051` on mainnet) and sign in-process. You need enough **TRX** for bandwidth/energy; USDT transfers consume energy — fund the hot address accordingly.

## Security notes

- This is **not** a hardware wallet. Threat model: trusted build, trusted machine, encrypted keystore file permissions (`0600`), strong passphrase, and optional TronGrid API key for reliability.
- **Do not** commit keystores, passphrases, or hex keys. `hot import` supports env `MONUJO_IMPORT_KEY` for scripting — prefer interactive `--key-hex` or env only in controlled environments.
- Amount parsing avoids `float64` for token math; USDT/TRX amounts are handled as integers / decimals via strings.
- USDT contract addresses: **mainnet** `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`; **Shasta** uses the test token address baked into `internal/tron/constants.go` (not official mainnet Tether).

## Planned next: offline signer

Versioned JSON envelopes live in `internal/transfer` (`UnsignedTransferRequest`, `SignedTransferResponse`) for a future air-gap flow: build unsigned tx online → sign JSON offline → broadcast signed tx online. Not implemented in this milestone.

## License

See repository root / upstream dependencies (gotron-sdk is LGPL/Apache mix; go-ethereum LGPL). Ensure compliance if you redistribute binaries.
