# 11. `wallet` commands

The `wallet` group manages the local **keystore** directory:
encrypted wallets that hold a private key, and **external**
address-only entries (no key) for convenient naming of third-party
addresses. It lists entries and verifies the “address in file —
passphrase” pair for local wallets. **These commands make no network
calls** — so they work the same on an online PC and on an air-gap
machine. The same files are later used by `monujo hot` (online
operations) and `monujo offline sign` (signing on an isolated
machine).

```text
monujo wallet
├── list               — show keystore entries (local and external)
├── create             — create a new wallet (random key)
├── import             — import an existing key (hex)
├── import-external    — store a Tron address under a name (no private key)
└── check              — verify a local wallet passphrase (no network)
```

Global flags (`--network`, `--output`, `--keystore-dir`, and so on)
apply to all subcommands. See
[03 — Global configuration](03-configuration.md) for details.

## Common principles

- **Passphrase**. `wallet create` and `wallet import` prompt for a
  passphrase via TTY with no echo — twice in a row for
  confirmation. Alternatively, `--generate-passphrase` generates a
  random phrase and prints it once to stderr (see
  [06 — Keystore → “Passphrase generator”](06-keystore.md#passphrase-generator)).
  **`wallet import-external` does not ask for a password** — the file
  holds no key material.
- **Secret wiping**. The passphrase and private-key material are
  zeroed (`ZeroPassphrase`, `Wipe`) immediately after use.
- **Metadata write**. On create/import, the current `--network` is
  written into the file metadata `networks` array. That entry is
  later used by `hot` commands to check network consistency.
- **Wallet name** (`--name`) is sanitized: only `a-z`, `A-Z`, `0-9`,
  `-`, and `_` are allowed. Other characters are replaced with `_`.
  An empty name becomes `default` (so `--name " "` creates
  `default.json`).
- **The name must not look like a Tron base58 address**. If the
  trimmed string passes the same heuristic used for `--from` /
  `--to` / `--addr`, `wallet create`, `wallet import`, and
  `wallet import-external` exit with an error (before the passphrase
  prompt, `--overwrite`, and any file write) so a file name is not
  confused with an address in other commands.
- **`--addr` in `wallet check`**. The value is interpreted the same
  way as in `hot balance` and `offline sign`: a string that looks like
  a Tron base58 address is looked up among keystore entries by the
  `address` field in JSON; otherwise the string is treated as a wallet
  **name**. Verification requires a **local encrypted** wallet:
  **`import-external` (external) entries are rejected** with a clear
  error, because the file has no private key.

---

## `wallet list`

Shows all entries in the keystore directory: name, **type** (`local` —
encrypted key, `external` — address only), Tron address, and the list
of networks from metadata. **No decryption is performed**; a
passphrase is not required.

### Usage

```bash
monujo wallet list
monujo --output json wallet list
```

### Flags

There are no local flags. The directory is read from
`--keystore-dir`/`MONUJO_KEYSTORE_DIR`/`~/.config/monujo/keystore`.

### What happens

1. The keystore directory is opened. If it does not exist, an empty
   list is printed (no error).
2. All `*.json` files are iterated (subdirectories are skipped).
3. Each file is parsed as wallet JSON: either an encrypted local
   file (v1/v2 with KDF), or a minimal external file with
   `"external": true` (see [06 — Keystore](06-keystore.md)). Invalid
   files are skipped without error.
4. Entries are sorted by name.

### Example: `table` format

```text
$ monujo wallet list
Keystore: /home/user/.config/monujo/keystore
NAME      TYPE      ADDRESS               NETWORKS
funding   local     TFG7NgT...            mainnet,shasta
custodian external  TXYZ...               mainnet
imported  local     TXYZ...               -
myhot     local     TLcVUPa...            shasta
```

Columns are tab-separated. If `networks` is empty, `-` is printed.
If the directory is empty — `No wallets.`.

### Example: `json` format

```json
{
  "keystore_dir": "/home/user/.config/monujo/keystore",
  "wallets": [
    {"name": "funding", "address": "TFG7NgT...", "external": false, "networks": ["mainnet", "shasta"]},
    {"name": "custodian", "address": "TXYZ...", "external": true, "networks": ["mainnet"]},
    {"name": "imported", "address": "TXYZ...", "external": false},
    {"name": "myhot", "address": "TLcVUPa...", "external": false, "networks": ["shasta"]}
  ]
}
```

The **`external`** field is always present in JSON (`true` / `false`).
`networks` is omitted from JSON when empty (`omitempty` tag).

---

## `wallet create`

Creates a **new** wallet: generates a random secp256k1 key, derives an
Argon2id key from the passphrase, encrypts the private key with
AES‑256‑GCM, and writes the file with mode `0600`. The current
`--network` is automatically stored in the new file’s `networks`
metadata.

### Usage

```bash
monujo wallet create --name <NAME> [--export-private-key --allow-private-key-export]
monujo --network shasta wallet create --name testhot
monujo --output json wallet create --name myhot --export-private-key --allow-private-key-export
```

### Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--name` | string | yes | — | Wallet name. Only ASCII letters, digits, `-`, and `_` are allowed; the name must not be a valid Tron base58 address. |
| `--export-private-key` | bool | no | `false` | Print the generated private key once. **Dangerous**: the key will appear in terminal output or a JSON stream. |
| `--allow-private-key-export` | bool | no | `false` | Required risk acknowledgement for `--export-private-key`. |
| `--overwrite` | bool | no | `false` | **DANGEROUS**: allow replacing an existing wallet file. The old private key is permanently lost. On a TTY an extra `[y/N]` confirmation is requested; without a TTY the overwrite proceeds without that extra prompt. |
| `--generate-passphrase` | bool | no | `false` | Generate a random passphrase and print it once instead of interactive input. |
| `--generate-words` | int | no | `7` | Number of words in the generated phrase (≥ 4). |
| `--generate-dict` | string | no | `english` | Dictionary: `english`, `english-short`, `esperanto-x`, `esperanto`, `russian`. |
| `--generate-separator` | string | no | ` ` | Separator between words. |
| `--generate-capitalize` | bool | no | `false` | Capitalize the first letter of each word. |
| `--generate-digits` | int | no | `0` | Append N random decimal digits to the end of the phrase. |
| `--generate-allow-non-tty` | bool | no | `false` | Allow printing the generated phrase to non-TTY stderr (**unsafe**). |

### What happens

1. A new passphrase is requested twice (or generated with
   `--generate-passphrase`). On mismatch — error
   `passphrases do not match`.
2. Key generation: `crypto.GenerateKey()` from go‑ethereum.
3. The Tron address is derived from the public key.
4. Encryption: `argon2id(passphrase, random_salt)` →
   `AES-256-GCM(nonce=random_12, plaintext=raw_priv_32)`.
5. File `<keystore_dir>/<safeName>.json` is created with mode `0600`.
   If the directory does not exist yet, it is created with mode `0700`.

### Example: `table` format

```text
$ monujo wallet create --name myhot
New keystore passphrase: ********
Confirm passphrase: ********
Created wallet "myhot" -> TLcVUPa...
Keystore: /home/user/.config/monujo/keystore
```

With `--export-private-key --allow-private-key-export`:

```text
$ monujo wallet create --name myhot --export-private-key --allow-private-key-export
...
WARNING: printing private key is unsafe; anyone with this hex can spend funds.
Private key (hex): 7b8a...c9d0
```

### Example: `json` format

```json
{
  "name": "myhot",
  "address": "TLcVUPa...",
  "path": "/home/user/.config/monujo/keystore"
}
```

With `--export-private-key --allow-private-key-export`:

```json
{
  "name": "myhot",
  "address": "TLcVUPa...",
  "path": "/home/user/.config/monujo/keystore",
  "private_key_hex": "7b8a...c9d0"
}
```

### Important notes

- The address derived from the key is valid on **both** networks
  (mainnet and Shasta). The `--network` label only controls which
  network is written into the `networks` metadata and which is used
  by default for subsequent commands.
- The file is created via a direct `os.WriteFile` (not atomic). If
  the process is interrupted between key generation and writing the
  file, the key is lost, because the private key was only just
  generated in memory and is not stored anywhere else.
- If a file with that name **already exists**, the command by default
  exits with error `wallet "<name>" already exists at <path>;
  use --overwrite to replace (DANGEROUS, irreversible)` before the
  passphrase prompt. Replacement is allowed only with an explicit
  `--overwrite`: on a TTY an extra `[y/N]` is requested; in
  non-interactive mode (script, CI, redirected stdin) the overwrite
  proceeds without that extra confirmation — the flag itself is
  treated as confirmation. A private key lost this way is **not
  recoverable**.

---

## `wallet import`

Imports an existing private key (64 hex characters) into a new
keystore file. The current `--network` is written into metadata.

### Usage

```bash
monujo wallet import --name <NAME> --key-hex <64_HEX>
# or
MONUJO_IMPORT_KEY=<64_HEX> monujo wallet import --name <NAME>
monujo --network shasta wallet import --name testhot --key-hex 0x...
```

### Flags

| Flag          | Type   | Required | Default | Description                                                                                                                                                           |
|---------------|--------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--name`      | string | yes      | —       | Keystore file name. Must not be a string recognized as a valid Tron base58 address (see common principles).                                                           |
| `--key-hex`   | string | no       | —       | 64 hex characters of a secp256k1 private key. Optional `0x` prefix.                                                                                                   |
| `--overwrite` | bool   | no       | `false` | **DANGEROUS**: allow replacing an existing wallet file. The old private key is permanently lost. On a TTY — with `[y/N]` confirmation; without a TTY — without.       |
| `--generate-passphrase` and related `--generate-*` | — | no | — | Same passphrase-generation flags as `wallet create`. |

If `--key-hex` is not set, the program takes the key from the
`MONUJO_IMPORT_KEY` environment variable. If neither is present —
error:

```
provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key
```

### What happens

1. The hex key is parsed: length (exactly 64 characters) and
   validity (valid ECDSA secp256k1, not all zeros) are checked.
2. If a file with that name already exists in the keystore, the
   command refuses to continue without `--overwrite` (same as
   `wallet create`; see the corresponding section above).
3. The password is requested twice (or generated via
   `--generate-passphrase`).
4. Encryption and storage work the same as in `wallet create`.
5. The address in the file is derived deterministically from the key.

### Example: `table` format

```text
$ monujo wallet import --name imported --key-hex 7b8a...c9d0
New keystore passphrase: ********
Confirm passphrase: ********
Imported wallet "imported" -> TXYZ...
Keystore: /home/user/.config/monujo/keystore
```

### Example: `json` format

```json
{
  "name": "imported",
  "address": "TXYZ...",
  "path": "/home/user/.config/monujo/keystore"
}
```

### Security

- A key from an environment variable may end up in logs and shell
  history. Use `MONUJO_IMPORT_KEY` only in a controlled environment
  (one-shot assignment, no write to `.bash_history`).
- In `table` mode the key itself is never printed; in `json` mode
  neither — `import` has no `--export-private-key` equivalent.

---

## `wallet import-external`

Stores **only** a Tron base58 address under the chosen file name in
the keystore. There is no private key in the file; the entry is marked
`"external": true` and appears as **external** in `wallet list`.

Such an alias can be used wherever the CLI accepts a wallet name or
address for address substitution (for example `hot balance --addr`,
`hot send-* --to`, `offline build --from` / `--to`). Commands that
need a private key (`hot send-* --from`, `offline sign`, `multisig cosign`,
`wallet check`) exit with an error for an external entry **before**
the passphrase prompt.

The current `--network` is written into the metadata `networks` array
(as with `wallet create` / `wallet import`) so that `hot` can apply
the same network-mismatch check as for local wallets.

### Usage

```bash
monujo wallet import-external --name <NAME> --addr <TRON_BASE58>
monujo --network shasta wallet import-external --name payer --addr T...
```

### Flags

| Flag          | Type   | Required | Default | Description |
|---------------|--------|----------|---------|-------------|
| `--name`      | string | yes      | —       | File name in the keystore (`<safeName>.json`). Must not be a string recognized as a valid Tron base58 address (see common principles). |
| `--addr`      | string | yes      | —       | Tron base58 address (validated locally before writing). |
| `--overwrite` | bool   | no       | `false` | **DANGEROUS**: replace an existing file. If it was a local wallet, the private key is permanently lost. |

### Example: `json` format

```json
{
  "name": "custodian",
  "address": "TXYZ...",
  "path": "/home/user/.config/monujo/keystore",
  "external": true
}
```

---

## `wallet check`

Verifies **offline** (no RPC and no transaction broadcast) that the
entered passphrase decrypts a local wallet file and that the
decrypted key yields the same Tron address as stored in that file’s
`address` field. Useful for checking a backup: that the file name,
recorded address, and passphrase are not mixed up.

**`import-external` entries are not supported**: they have no key to
decrypt — the command exits with an error before the passphrase
prompt.

Unlike `wallet list`, the command **performs a full decryption**
(the key exists briefly in memory, then is wiped). Unlike
`hot balance`, it does not contact the network.

If the file is still in v1 format (scrypt), a successful check may
**trigger automatic migration** to v2 (argon2id), like any other
operation that uses `LoadPrivateKey` — see
[06 — Keystore → Migration v1 → v2](06-keystore.md#migration-v1--v2).

### Usage

```bash
monujo wallet check --addr <WALLET_NAME>
monujo wallet check --addr T...   # if the address appears in wallet list
monujo --output json wallet check --addr myhot
```

### Flags

| Flag     | Type   | Required | Default | Description |
|----------|--------|----------|---------|-------------|
| `--addr` | string | yes      | —       | Name of a **local** wallet in the keystore **or** its Tron base58 address. External (`import-external`) entries are rejected. |

### What happens

1. The keystore entry for `--addr` is found (list metadata read only).
   If the entry is **external** — immediate error.
2. The passphrase is requested once (TTY with no echo; in non-TTY
   the phrase can be passed on stdin; the trailing newline is
   trimmed).
3. `LoadPrivateKey` runs: KDF, AES-GCM, and a check that the derived
   address matches the `address` field in JSON.
4. On success — a short message on stdout (or JSON with fields
   `ok`, `wallet`, `address` when `--output json`).

On a wrong phrase or a corrupted file the command exits with an error
(for example, `decryption failed (wrong passphrase or corrupt file)`).

---

## Summary table of all `wallet` flags

| Subcommand | Local flags                                                | Asks for password                                 |
|------------|------------------------------------------------------------|---------------------------------------------------|
| `list`     | —                                                          | no                                                |
| `create`   | `--name`, `--export-private-key`, `--allow-private-key-export`, `--overwrite`, `--generate-*` | yes (× 2 — new phrase, or generation)          |
| `import`   | `--name`, `--key-hex`, `--overwrite`, `--generate-*`       | yes (× 2 — new phrase, or generation)             |
| `import-external` | `--name`, `--addr`, `--overwrite` | no |
| `check`    | `--addr`                                                   | yes (once — existing phrase)                      |
