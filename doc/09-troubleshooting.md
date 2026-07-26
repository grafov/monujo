# 09. Troubleshooting

Below are the most likely errors and warnings you may see. All
messages are in English (as in the sources); next to each is an
explanation and a workaround.

## Configuration errors

### `unknown network "..." (use mainnet or shasta)`

A `--network` value other than `mainnet`/`shasta` was passed (and not
an empty string). Check case and typos.

### `unknown output format "..." (use table or json)`

Invalid `--output` value. Only `table` and `json` are allowed.

### `provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key`

`wallet import` was run without a key. Pass `--key-hex` or export the
`MONUJO_IMPORT_KEY` variable.

### `private key must be 64 hex characters`

Input that is too short or too long. A `0x` prefix is allowed;
characters must be strictly `0-9a-fA-F`. Leading/trailing whitespace is
trimmed.

### `decode private key hex: ...`

The string contains non-hex characters. Double-check that you copied
the hex representation.

### `invalid private key length`

After decoding you did not get 32 bytes. This can happen if the hex
string has an “extra” character that did not break the length check
(for example an unseparated `0x` prefix left an odd length).

### `refusing weak private key`

An all-zero key is forbidden. It cannot be imported — this is an
explicit error/injection safeguard.

## Passphrase errors

### `passphrases do not match`

For `wallet create`/`wallet import` the two entered passphrases differ.
Run the command again.

### `decryption failed (wrong passphrase or corrupt file)`

When trying to use an existing keystore (`hot send-*`, `offline sign`,
`wallet check`; `hot balance` does not ask for a password and never
shows this error):

- Most likely the password was entered incorrectly. Case matters.
- The file may be corrupt (check `cat ~/.config/monujo/keystore/<name>.json`
  — it should be valid JSON v1 or v2).
- If the password contains **non-ASCII** characters (Cyrillic, umlauts,
  ligatures, full-width, emoji) and the wallet was created with a
  previous monujo build without NFKC normalization.
  ASCII passwords do not have this problem.

### `address mismatch in keystore`

A very rare case: decryption succeeded, but the private key produced
an address different from the one stored in `address`. This indicates
file corruption (or manual editing). Do not use such a keystore —
recreate it.

### `invalid decrypted key length`

The decrypted buffer is not 32 bytes. The file is corrupt.

## Amount errors

### `empty amount`

`--amount ""` was passed. Specify a number.

### `negative amount not allowed`

An amount cannot start with `-`. The program refuses negative numbers
at parse time.

### `invalid character in amount` / `invalid character in fraction`

Only digits and a single dot are allowed. `1,5` (comma) or `1e3`
(scientific notation) are not supported.

### `multiple decimal points`

More than one dot in the number.

### `too many fractional digits (max 6)`

Both TRX and USDT have 6 fractional digits. For example, `1.1234567`
is rejected; use `1.123456`.

### `trx amount overflows int64`

TRX in sun must not exceed `int64`. This is an astronomically large
amount (more than all TRX in existence), but the check exists.

### `amount must be positive` (USDT only)

For `hot send-usdt`, zero and negative amounts are forbidden by a
separate check after parsing.

## Address errors

### `invalid Tron address: ...`

The address fails Base58Check validation. Common causes:

- extra spaces or `\n`;
- an address from another network was copied (e.g. Ethereum `0x...`);
- the string was truncated (an address is exactly 34 characters).

## Network errors

### `trongrid accounts: HTTP 4xx: ...` / `trongrid trc20 txs: HTTP 4xx: ...`

The TronGrid node returned an error:

- `401`/`403` — check the API key.
- `429` — rate limit exceeded. Set an API key or wait.
- `5xx` — temporary node problem. Retry the request.

### `trongrid: <error>`

The node returned HTTP 200, but the JSON has `success=false` and a
non-empty `error` field. The message is quoted verbatim.

### `decode accounts: ...` / `decode trc20 txs: ...`

JSON does not parse. The node may be responding in a non-TronGrid-
compatible format (for example you overrode `--http-url` and hit the
wrong endpoint).

### `broadcast: <message>`

The write transport (HTTPS REST or gRPC) accepted the request, but the
node returned a non-zero code. Typical messages:

- `OUT_OF_ENERGY` / `OUT_OF_TIME` — for a USDT transfer there was not
  enough energy/bandwidth, and the TRX fee exceeded `feeLimit=100 TRX`.
  Top up the address with TRX (or stake energy).
- `BANDWITH_ERROR` — insufficient bandwidth.
- `CONTRACT_VALIDATE_ERROR` — usually: the recipient is invalid, the
  sender is not activated, or the USDT contract was not found on the
  selected network.
- `DUP_TRANSACTION_ERROR` — a duplicate transaction with the same
  `ref_block_hash`. Wait 1–2 blocks.

### `empty transaction`

(Specific to the optional gRPC transport) `gotron-sdk` returned a nil
transaction — usually this means the gRPC node is unreachable or
returned an error that the SDK wrapped as `nil`. Check `--grpc-url` /
node availability.

### `connection refused`, `context deadline exceeded` / `timeout`

Network problems. The write HTTP/gRPC client timeout is 60 seconds;
for `ReadClient` it is 30 seconds (`HTTP`). These limits cannot be
relaxed in the current version.

## Network confirmation errors

### `network mismatch: use --force to proceed without a terminal, or run interactively to confirm`

In the wallet file `networks` is non-empty and does not contain the
current `--network`, and STDIN is not a TTY (e.g. in CI). Solutions:

- Run interactively and answer `y`;
- Pass `--force`, aware that a **different** network was chosen than
  the one the wallet was previously used on;
- Remove the stale entry from `networks` (edit the JSON).

### `aborted`

The user answered `n` (or pressed Enter) to the
`Continue? [y/N]:` prompt. Nothing broke; the operation was cancelled.

## Keystore errors

### `unsupported keystore version <N>`

The file was written in a format outside `{1, 2}`. Most likely it comes
from another program or a future version. The current monujo release
can read v1 (legacy scrypt) and v2 (argon2id); v1 is migrated to v2 on
the first successful unlock.

### `keystore v1 requires kdf "scrypt", got "..."` / `keystore v2 requires kdf "argon2id", got "..."`

An inconsistent `version` and `kdf` combination was found in the file
(e.g. `version: 2, kdf: "scrypt"`). This is either a manually edited
file or an interrupted migration. If you still have a
`<name>.scrypt.bak` backup — rename it to `<name>.json` and run any
hot command: it will migrate again.

### `note: migrated wallet "..." from scrypt to argon2id; legacy backup saved as <...>.scrypt.bak`

An informational message on stderr (not an error): your old v1 file
was successfully migrated to v2. A `<name>.scrypt.bak` backup appeared
alongside it. You can delete it after you have once confirmed the v2
file works by performing a signing operation
(`hot send-trx --from <name>` with a minimal amount to a test address,
or `offline sign`). `wallet list` and `hot balance` only show public
metadata and do not prove ciphertext validity.

### `warning: failed to migrate wallet "..." to argon2id: ...`

Migration failed (e.g. no write permission in the directory). The
original operation still **completed successfully** — the key was
decrypted and used. On the next run monujo will try to migrate again.

### Manual rollback v2 → v1

If after migration you need to go back to v1 (e.g. an older monujo
binary without argon2id support):

1. Delete `<name>.json` (this is the new v2).
2. Rename `<name>.scrypt.bak` to `<name>.json`.
3. Run the old build. The new build will migrate the file again on the
   first unlock, so do not use both versions against the same directory
   at once.

### `empty network` (internal)

Raised in `EnsureNetworkRecorded` when `--network` normalizes to an
empty string. Should not happen in normal use.

### `broadcast ok (tx <id>) but could not update wallet networks metadata: ...`

The transaction went out, but updating the keystore file failed (e.g.
permissions changed). The operation itself succeeded — the funds were
sent. The file will try to update again on the next successful send.

## Symptoms and diagnostics

### “The program hangs for a long time”

- Is `--api-key` empty and you hit the TronGrid rate limit? Try with
  a key.
- Is HTTPS REST (the primary transport) unreachable? Check
  `curl https://api.trongrid.io/v1/accounts/T...` or change `--http-url`.
- If optional `--write-transport grpc` is used: is gRPC unreachable
  (firewall/proxy/regional block)? Try `--grpc-url` with a different
  public node.

### “Balance shows 0, but the explorer shows funds”

- Wrong network. The address is the same on mainnet and Shasta, but
  the balances differ. Check `--network`.
- The account is not activated — `data: []` in the TronGrid response.
  The program prints zeros. Send the first TRX via `hot send-trx` (or
  wait for an incoming transfer).
- In `watch txs` you may not see transactions if they are not for the
  canonically embedded USDT contract. The command does not show other
  TRC-20 tokens.

### “Other TRC-20 tokens are visible, but not in monujo output”

This is by design: `watch balance` and `watch txs` filter only USDT
for the selected network. There is no `--contract-address` flag in the
current version.

### “`wallet list` is empty even though files exist”

Check:

- `--keystore-dir` points at the correct directory;
- files have the `.json` extension (`.scrypt.bak` backups are not
  listed — that is expected);
- `version` inside the JSON is `1` or `2`;
- the `address` field is non-empty;
- permissions do not prevent your user from reading them.

Files that fail these checks are silently skipped.

### “`--version` shows `dev (unknown)`”

The binary was built without `-ldflags` (e.g. via `go build` without
the Makefile). Building with `make build` embeds real values from
`git describe`/`git rev-parse`.

## When to file a bug

If you observe:

- a panic from the standard library or third-party packages;
- destruction/overwrite of another wallet (in `safeName` or
  `EnsureNetworkRecorded`);
- failed decryption of a file that was definitely used before with
  the same password;
- a spurious “network mismatch” warning when the network matches,

— attach to the bug report:

1. Version: `monujo --version`;
2. The full command without sensitive data;
3. The error message;
4. An anonymized keystore JSON (without `data_hex`, `nonce_hex`,
   `salt_hex` — leave `version`, `kdf`, `n/r/p` or
   `argon2_*`, `address`, `networks`).

And be sure to state the OS and the Go version used to build
(`go version`).
