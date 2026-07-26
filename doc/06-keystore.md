# 06. Keystore

`monujo` stores private keys in a local **encrypted keystore**. Each
wallet is a separate JSON file in the directory set by
`--keystore-dir` / `MONUJO_KEYSTORE_DIR` / default
`~/.config/monujo/keystore`.

## Location and permissions

- The keystore directory is created automatically
  (`os.MkdirAll(... 0o700)`) on first write (`wallet create`/`wallet
  import`).
- Files are created with mode `0600` (`os.WriteFile(... 0o600)`).
- The file name is built from `--name` via the `safeName` function:
  - allowed characters: `a-z A-Z 0-9 _ -`,
  - all others are replaced with `_`,
  - an empty name becomes `default`.
  For example, `--name "My Cold #1"` creates `My_Cold__1.json`.
- When writing metadata (see below), an atomic replace is used via a
  temporary file `.monujo-keystore-*.tmp` and `os.Rename`.
- Key reading goes through `os.OpenRoot(keystoreDir)`: a file can be
  opened **only** inside the given directory; an escape attempt via
  symlink or `..` is guaranteed to fail at the syscall level, even if
  a hole appears in `safeName`.

## File format v2 (current)

The current format version is `2` with KDF `argon2id`. The program also
reads version `1` files (legacy, scrypt) and on the first successful
unlock **automatically rewrites them to v2** (see “Migration v1 → v2”
below). Versions outside `{1, 2}` yield
`unsupported keystore version <N>`.

Minimal v2 file example:

```json
{
  "version": 2,
  "address": "TLcVUPaYnQp...",
  "networks": ["mainnet"],
  "kdf": "argon2id",
  "salt_hex": "8a7b...e1f2",
  "argon2_time": 3,
  "argon2_memory": 65536,
  "argon2_threads": 4,
  "argon2_key_len": 32,
  "cipher": "aes-256-gcm",
  "nonce_hex": "5c1d...90ab",
  "data_hex": "fa12...34cd"
}
```

### External entry (address only, no key)

A file with `"external": true` stores a **named** Tron address without
KDF and without key ciphertext. It is created by `monujo wallet
import-external`. Minimal form:

```json
{
  "version": 2,
  "external": true,
  "address": "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
  "networks": ["mainnet"]
}
```

Such files participate in `wallet list` and in name resolution in
commands that need only an address; `LoadPrivateKey`, `wallet check`,
sending with `--from`, `offline sign`, and `multisig cosign` are
**unavailable** for them. The `networks` field for an external file is
set at import time and is **not** extended automatically after
transactions (`EnsureNetworkRecorded` is forbidden for external
entries).

### v2 fields

| Field             | Type            | Description                                                                                                                                           |
|-------------------|-----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| `version`         | int             | Always `2`.                                                                                                                                           |
| `address`         | string          | Tron address in Base58Check. Checked on decrypt (if the decrypted key yields a different address — error `address mismatch in keystore`).             |
| `networks`        | array of string | Optional. Networks on which the wallet has already been used. Unique, sorted. `omitempty`.                                                            |
| `kdf`             | string          | Always `"argon2id"` for v2.                                                                                                                           |
| `salt_hex`        | string          | Random salt, 16 bytes (128 bits) in hex. NIST SP 800-132 §5.1 requirement.                                                                            |
| `argon2_time`     | uint32          | Argon2id parameter `t` (passes). Default `3`.                                                                                                         |
| `argon2_memory`   | uint32          | Argon2id parameter `m` (memory cost) in KiB. Default `65536` = 64 MiB.                                                                                |
| `argon2_threads`  | uint8           | Parameter `p` (parallelism). Default `4`.                                                                                                             |
| `argon2_key_len`  | uint32          | Derived key length in bytes. Always `32` (for AES-256).                                                                                               |
| `cipher`          | string          | Cipher name: `"aes-256-gcm"`.                                                                                                                         |
| `nonce_hex`       | string          | Random GCM nonce, 12 bytes in hex.                                                                                                                    |
| `data_hex`        | string          | Private key ciphertext (32 bytes plaintext + 16 bytes GCM tag), in hex.                                                                               |

### File format v1 (legacy, auto-migrated)

Old format before the argon2id migration. Fields `n, r, p` hold scrypt
parameters (`N=32768, r=8, p=1`), `kdf="scrypt"`, 32-byte salt. On the
first successful `LoadPrivateKey` the file is rewritten to v2; a
backup `<name>.scrypt.bak` remains beside it and can be deleted after
the first successful check of the new version.

## Cryptography (v2)

```
DK         = argon2id(passphrase, salt=16 rand, t=3, m=65536 KiB, p=4, dkLen=32)
ciphertext = AES-256-GCM(key=DK, nonce=12 rand, plaintext=raw_priv_key_32)
```

- **argon2id**: `golang.org/x/crypto/argon2`, RFC 9106. Parameters
  `t=3, m=64 MiB, p=4` match the “server” profile from the OWASP
  Password Storage Cheat Sheet. Values are stored in the file, so when
  defaults rise in future versions existing files remain readable.
- **AES-256-GCM**: `crypto/aes` + `crypto/cipher.NewGCM`. The
  authentication tag (16 bytes) is appended to `ciphertext`.
- **Randomness source**: `crypto/rand.Reader` for salt (16 bytes) and
  nonce (12 bytes). Strength matches the OS.
- **Wiping**: after decrypt and signing the program zeroes:
  - `passphrase` (`ZeroPassphrase`);
  - the derived argon2id/scrypt key (`zeroBytes`);
  - the private key plaintext buffer (`zeroBytes`);
  - ECDSA key material (`zeroPriv` zeroes `D.Bits()`).

  Protection against leaks via swap/core dumps is **not guaranteed**
  and depends on the OS.

## Migration v1 → v2

Automatic one-shot procedure:

1. You run any command that decrypts the key: `hot send-trx`, `hot
   send-usdt`, `offline sign`, or `wallet check`. `hot balance` does
   not decrypt the key (it reads the address from open metadata), so
   it does **not** trigger migration.
2. monujo successfully decrypts the v1 file with the scrypt key and
   verifies the address.
3. **Only after a successful check** the original `<name>.json` is
   renamed to `<name>.scrypt.bak`, and a fresh v2/argon2id file takes
   its place (atomic write via tmpfile + `rename`).
4. A line is printed to stderr: `note: migrated wallet "..." from
   scrypt to argon2id; legacy backup saved as <name>.scrypt.bak`.

Invariants:

- If migration fails after step 2, the program still returns the key
  to the calling operation (it already decrypted successfully). The
  next run will try to migrate again.
- On a wrong password — no migration and no backup.
- If `<name>.scrypt.bak` already exists (for example you cancelled an
  operation and ran it again after editing the file), the **second
  backup does not overwrite the first** — it gets a suffix
  `<name>.scrypt.bak.<unix-ms>`.
- `networks` from v1 are carried into v2 unchanged.

When to delete `<name>.scrypt.bak`: after you have confirmed that any
signing operation (`hot send-trx`, `hot send-usdt`, or `offline sign`)
on the new v2 file works as expected — that alone proves the
ciphertext is correct. **Without a network** the same is proven by
`wallet check --addr ...`: it decrypts the key and verifies the
address without sending transactions. `wallet list` shows only open
metadata and does not guarantee validity of the encrypted contents.
Until then `.bak` is your insurance if external conditions (disk, FS)
corrupted the v2 file during write.

Manual rollback: if for some reason you need the old version, delete
`<name>.json`, rename `<name>.scrypt.bak` back to `<name>.json`, and
run an old monujo build. The new build will not open mixed
combinations (`v2+scrypt` / `v1+argon2id`) — that guards against an
accidentally corrupted file.

## Compatibility with non-ASCII passwords before NFKC normalization

Starting with the version that added NFKC normalization (see
“Standards compliance” below), all passphrases are brought to a
uniform form before the KDF. For ASCII passwords this change is
**transparent**: NFKC is identity on ASCII, bytes do not change.

If you created a wallet on a previous monujo build with a **non-ASCII**
password (Cyrillic, umlauts, ligatures, full-width, emoji), one of two
things may apply:

- The original byte form already matches NFKC — everything works
  unchanged (typical on Linux/Windows, where the terminal already
  delivers a composed form).
- The original byte form differs from NFKC — for example macOS
  delivers NFD (`"café"` as `e`+`\u0301`), and old monujo passed those
  bytes into argon2id “as is”. After the update the same keystrokes on
  the same machine become NFC form, and the old wallet **stops
  unlocking**.

Two ways to fix it:

1. **Roll back to the old build**, open the wallet, obtain the private
   key (for example via a script using the old
   `keystore.LoadPrivateKey`), and recreate via `wallet import` on the
   new build with any password. Key and address stay the same.
2. If you have no old build — manually “assemble” the password from
   the expected bytes and pipe it into monujo without terminal
   normalization. This is a one-time hack; the correct path is option
   #1.

**ASCII passwords do not have this problem.** If you are unsure whether
you previously used a non-ASCII password — after updating, verify
wallet unlock: **`wallet check --addr ...`** (no network) or a signing
operation (`hot send-trx --from ...` with a minimal amount to a test
address, or `offline sign`). `wallet list` and `hot balance` do not
ask for a password and do not diagnose this issue.

## Standards compliance

- **NIST SP 800-132 — Password-Based Key Derivation.** 128-bit salt
  from `crypto/rand`, unique per file; KDF parameters are stored in
  the file. Salt requirements are met. SP 800-132 lists PBKDF2 as an
  approved KDF; monujo uses Argon2id as a modern alternative, which is
  **not** equivalent to the FIPS-approved list (see below).
- **OWASP Password Storage Cheat Sheet.** v2 parameters (`t=3, m=64
  MiB, p=4, keyLen=32`) match the “server” profile. The lower
  acceptable threshold in the cheat sheet is `t=2, m=19 MiB, p=1`; if
  under your conditions the OWASP default is excessive for
  performance, parameters can be lowered by editing the file directly
  — they are stored in the `argon2_*` fields.
- **NIST SP 800-63B §5.1.1.2 — Memorized Secrets.** monujo accepts an
  arbitrary UTF-8 passphrase, does not force rotation, and does not
  cap length from above. **Unicode NFKC normalization** is applied
  before every KDF call (Argon2id v2, scrypt v1, scrypt for offline
  HMAC). That means:
  - `"café"` (NFC, single code point `U+00E9`) and `"cafe\u0301"`
    (NFD, `e`+`\u0301`) yield the same key;
  - ligatures such as `"ﬁle"` (`U+FB01`+`le`) are equivalent to
    `"file"` (NFKC compatibility mapping);
  - full-width variants `"ｐａｓｓ"` are equivalent to ASCII `"pass"`.

  Boundary: NFKC does **not** equate visually similar but code-point-
  different characters (Cyrillic `а` ≠ Latin `a`, Greek `ο` ≠ Latin
  `o`). Use one keyboard layout when entering the password.
- **NIST SP 800-108 — KDF from PRF.** Governs deriving keys from other
  keys (e.g. HKDF); it does not apply to password KDFs and does not
  cover monujo.
- **FIPS 140-3 / ISO/IEC 19790 — Requirements for Cryptographic
  Modules.** monujo is **not** a certified cryptographic module.
  Argon2id is **not** on the FIPS-approved KDF list (FIPS 140-3 via SP
  800-140C / SP 800-132 allows only PBKDF2-HMAC-SHA2 for passwords).
  For environments that require FIPS 140-3 or ISO/IEC 19790
  compliance, monujo in its current form is **not suitable** — use a
  certified wallet or wait for a separate PBKDF2 mode (absent in this
  version).

Final positioning: a retail/dev utility that follows OWASP parameters
and SP 800-132 salt requirements, but is **not** FIPS-compatible and
**not** certified.

## Keystore backup

The `<name>.json` file is self-contained: it holds the address, salt,
KDF parameters, nonce, and ciphertext. The passphrase is **not**
stored in the file. Backup = file + password, and they must be kept
separately.

### Minimal procedure

```bash
mkdir -p /path/to/backup
cp ~/.config/monujo/keystore/<name>.json \
   /path/to/backup/<name>.$(date +%F).json
chmod 600 /path/to/backup/<name>.*.json
```

Verify restore on another medium:

```bash
monujo --keystore-dir /path/to/backup wallet list
```

The address in the output should match the original. For a stronger
check — run any signing operation (`hot send-trx --from <name>` with
a minimal amount to a test address, or `offline sign`) with that
`--keystore-dir`: only that proves the encrypted material decrypts
with the same password (zero amounts for send-trx are fine; what
matters is the ability to unlock).

### `tool vault` utility (recommended path)

`monujo tool vault pack` packs the **entire keystore directory** into
one encrypted age archive (scrypt passphrase recipient, RFC 9106-
style); `monujo tool vault unpack` does the reverse. This is a thin
wrapper over the same primitives as the age CLI — no exec of an
external binary, pure Go dependency `filippo.io/age`.

```bash
# Pack the whole keystore into one file (password prompted twice).
monujo tool vault pack --out backup-$(date +%F).age

# Unpack into the target directory (default --keystore-dir).
monujo tool vault unpack --in backup-2026-04-21.age --out /restore/keystore

# Verify that addresses match.
monujo --keystore-dir /restore/keystore wallet list
```

What goes into the archive: only `*.json` and
`*.scrypt.bak[.<unix-ms>]`. Everything else (subdirectories, dotfiles,
`.monujo-keystore-*.tmp`) is silently ignored. Names inside the tar are
“flat”, without a directory prefix, so unpacking is portable.

Flags:

- `--armor` (pack) — ASCII armor (PEM-like text), convenient for
  copying via email/git/printout.
- `--overwrite` (pack) — allow overwriting an existing `--out` file.
  Replacement is atomic: `monujo` writes to a sibling
  `.monujo-vault-*.tmp` and after a successful pack renames it over the
  old archive; a failed pack does not damage the previous backup. On a
  TTY the flag still asks `[y/N]` once more.
- `--overwrite` (unpack) — allow overwriting existing files in
  `--out`. By default unpack refuses if at least one target file
  already exists (check before any writes; no partial unpack). On a
  TTY the flag still asks `[y/N]` once more.
- `--dry-run` (unpack) — full validation (decrypt, tar parse,
  allowlist, conflict check) without a single write to disk. Suitable
  as “vault verify”.

Limits:

- By default `--out` for `pack` must not exist (protection against
  clobbering a ready backup); replacement only with explicit
  `--overwrite`. Stdout is allowed via `--out -`, where `--overwrite`
  is not needed.
- The vault password is a **separate entity** from the passwords of
  keystore files inside. Lose the vault password — lose the archive;
  lose a single keystore password — lose that wallet even with an
  intact archive. Store the vault password at the same security level
  as keystore passwords (manager / paper envelope), **not** on the
  same medium as the archive.
- Age compatibility: the file can theoretically be opened with plain
  `age -d` (you get tar inside), but that is an implementation detail,
  not a format-stability promise; use `monujo tool vault unpack` for
  reliable parsing with allowlist checks.

### Keep the password separate

- Password manager (1Password, KeePass, Bitwarden) — primary
  recommended option.
- Paper envelope in a safe — for long-term cold backups.
- **Never** put the password on the same medium as the `.json`, and do
  not save it in cloud sync without an extra encryption layer.

### Extra encryption layer (recommended)

On top of the monujo cipher you can wrap the file with another
password. Example with `age`:

```bash
age -p -o wallet.json.age ~/.config/monujo/keystore/<name>.json
age -d -o /path/to/restored/<name>.json wallet.json.age
```

Or GPG:

```bash
gpg -c --cipher-algo AES256 ~/.config/monujo/keystore/<name>.json
gpg -d <name>.json.gpg > /path/to/restored/<name>.json
```

Such a backup is safer to put in the cloud / email / shared network
drives: even if the file and monujo metadata leak, an external
password remains. If you need to wrap the **whole directory**, not
each file separately — use `monujo tool vault pack` (see above): the
same primitive (age scrypt-mode), but as one archive with allowlist
checks on unpack.

### Geographic distribution

At least **two** copies:

- primary working medium (external SSD/USB, separate from the machine
  running monujo);
- cold storage (USB in a safe / remote physical location / cloud with
  an external age/GPG password).

For an air-gap scenario (see `10-offline-commands.md`) a keystore
backup is **mandatory** and must not overlap with online machines
(`build`/`broadcast`).

### What is NOT a backup

- `privHex` from `wallet create --export-private-key
  --allow-private-key-export` without a password — this is a “bare”
  private key, not a backup: without wrapping, its leak equals theft
  of funds. If you wrote it down, store it like a password (manager /
  paper), remove it from `~/.bash_history` / clipboard.
- A screenshot with a password or hex key.
- A copy of only the password without the `.json`.
- A copy of only the `.json` without the password (but that is a
  cipher, not “no backup”: the file exists, access to the key does
  not).

### Rotation and migration

Moving to a new machine: copy the `.json`, enter the same password. If
the file is still v1 — auto-migration runs on first unlock, after which
sync backups of the new v2 version. The v1 file is not needed after
successful migration, but `<name>.scrypt.bak` can be kept as an
emergency copy until the first successful operation on v2.

### Recovering access if the password is lost

**Impossible.** Argon2id is designed so that password brute-force is
computationally impractical at a normal passphrase length. This is a
stated operational risk (FIPS 140-3 / ISO/IEC 19790 and SP 800-63B
explicitly anticipate it): the only remedy is a reliable backup of the
**password** itself. monujo does not solve this problem and does not
claim to.

## `networks` metadata

The `networks` field is a “diary” of wallet uses, helping avoid
confusing mainnet with a test network.

### When it is filled

- On `wallet create --network <X>` and `wallet import --network <X>` —
  recorded as `[X]`.
- After a successful `hot send-trx`/`hot send-usdt` with
  `--network <X>` — if `X` is missing, it is added
  (`EnsureNetworkRecorded`). The cipher is not rebuilt; only metadata
  changes; the file is rewritten atomically.

### When it is checked

In three commands: `hot balance`, `hot send-trx`, `hot send-usdt`.
Logic: if the `networks` array is **non-empty** and the current
`--network` is **absent** from it, the program prints a warning to
`stderr`:

```
Warning: wallet "myhot" was previously used on network(s): mainnet
The current operation uses network "shasta", which is not in that list.
This wallet has already been used in operations on the listed network(s).
```

Then:

- if `--force` is set — continues without questions;
- if STDIN is a TTY — asks `Continue? [y/N]:`. Answer `y`/`yes`
  (case-insensitive) — continue; anything else — error `aborted`;
- if STDIN is not a TTY — immediate error `network mismatch: use
  --force to proceed without a terminal, or run interactively to
  confirm`.

### Normalization

- Any input is lowercased and mapped to one of the canonical values
  `mainnet`/`shasta`.
- Duplicates are collapsed.
- The list is sorted (`slices.Sort`).
- Empty/unknown values are dropped.

There is no explicit command to remove an entry from `networks` — the
array only grows. If you need to “forget” a previous network, edit the
JSON manually (the `networks` field).

## How the program finds a file by name or address

`hot balance`, `hot send-*`, `offline build`/`sign`, and `wallet check`
look up the keystore via **`--addr`** (subject without `--to` in the
CLI) or **`--from`/`--to`** (transfers and unsigned build). Commands
`wallet create`, `wallet import`, and `wallet import-external` do not
allow a `--name` that after normalization looks like a valid Tron
base58 address — otherwise that name would later collide with the
“address or name” heuristic in the same flags.

For **`import-external`** entries (`"external": true`) substituting an
address by name works wherever an ordinary wallet needs only the
`address` field in JSON; operations with a **private key**
(`LoadPrivateKey`, `EnsureNetworkRecorded` to append a network after
broadcast) are forbidden for such files.

1. If the value passes Tron base58 validation — it is treated as an
   address.
   - For **`--from`** in `hot send-*`, for **`--addr`** in
     `offline sign`/`wallet check` the program looks up a match via
     `keystore.ListWallets` (by the `address` field in plaintext
     metadata, without decryption); the address must correspond to a
     local file, otherwise the command fails (a private key is
     required).
   - For **`--addr`** in `hot balance`, for **`--from`** in
     `offline build`, and for **`--to`** everywhere, mismatch with the
     local list is allowed: `balance` may query an arbitrary address,
     `offline build` on an online machine does not decrypt the key,
     and `--to` is an arbitrary recipient address.
2. If the value does NOT look like an address, it is treated as a
   name:
   - `safeName` is applied (conversion to a safe file name).
   - File `<safeName>.json` is opened via `os.OpenRoot(keystoreDir)`.
   - JSON is parsed. If `version` is not in `{1, 2}` — error
     `unsupported keystore version`.
   - For `--to` only the plaintext `address` field is taken from this
     file; no private key decryption occurs.

The same applies to `wallet *` (via `--name`). That means: if you
originally created a wallet named `My Cold`, the file will be
`My_Cold.json`, and you can address it both as `--from "My Cold"`
(in `send-trx` / `offline build`) and as `--addr "My Cold"` (in
`hot balance` / `wallet check` / `offline sign`): the string is
normalized to the same file name.

## Passphrase generator

monujo includes a built-in passphrase generator on `crypto/rand` with
uniform sampling without modulo bias. Command:

```bash
./build/monujo tool genpass [--words N] [--dict DICT] [--separator S]
                            [--capitalize] [--digits K]
                            [--allow-non-tty]
```

The same parameters are available as options of `wallet create` and
`wallet import` via the `--generate-*` prefix; with
`--generate-passphrase` a new phrase is generated, printed once to
stderr, and immediately used as the password of the new keystore file.
The `--generate-allow-non-tty` flag is the analogue of
`--allow-non-tty` for the built-in case.

### Invariants

- Randomness source — **only** `crypto/rand.Reader`. Uniform sampling
  via `rand.Int(rand.Reader, big.NewInt(N))`.
- The phrase is NFKC-normalized on output: the bytes you see on screen
  match bit-for-bit what goes into Argon2id.
- Output is **only** to stderr and **only** once. The phrase is never
  written to stdout (convenient in pipes), nor saved to a file /
  clipboard / env.
- If stderr is not a terminal, the CLI refuses to print the phrase
  without `--allow-non-tty` (or `--generate-allow-non-tty`) — protection
  against accidentally landing a password in CI logs.
- After printing, the buffer is cleared via `ZeroPassphrase`.

### Dictionaries

| `--dict` flag    | Size  | Characters     | Bits/word | When to choose                                            |
|------------------|-------|----------------|-----------|-----------------------------------------------------------|
| `english`        | 7776  | ASCII          | 12.92     | Default, EFF Long Wordlist, canonical.                    |
| `english-short`  | 1296  | ASCII          | 10.34     | Convenient to dictate aloud; short phrases.               |
| `esperanto-x`    | 14851 | ASCII          | 13.86     | Esperanto in x-system; safe on any keyboard.              |
| `esperanto`      | 14851 | UTF-8 (ĉĝĥĵŝŭ) | 13.86     | Only if input is configured on all unlock machines.       |
| `russian`        | 15213 | UTF-8 (а-я,ёй) | 13.89     | Only with a guaranteed Cyrillic IME everywhere.           |

The `esperanto` and `esperanto-x` dictionaries are a merge of the
Wiktionary/Wikipedia 2023 frequency list (15 000 lemmas) and the
earlier Tekstaro list (see
[internal/passphrase/wordlists/SOURCES.md](../internal/passphrase/wordlists/SOURCES.md)):
after `strings.ToLower` only lowercase letters `a–z` and `ĉ ĝ ĥ ĵ ŝ ŭ`
remain, length 3–12 runes; both flags share the same content (x-system
is a mechanical diacritic replacement).

The `russian` dictionary is the Lyashevskaya–Sharov frequency list
(Russian National Corpus, 2009), 20 004 lemmas in the Wiktionary
mirror; after `strings.ToLower` only lowercase letters `а-я ё й` of
length 3–10 runes remain — 15 213 words total. Spellings without `ё`
(`бобер`, `еще`) are preserved as in the source; NFKC distinguishes
`ё/е` and `й/и`, see below.

Minimum — 4 words, default — 7 words. When expected entropy is below
80 bits the CLI prints a warning but does not abort.

### Warnings for non-ASCII dictionaries

When choosing `--dict esperanto` or `--dict russian`, before the
phrase itself the CLI prints a separate WARNING about input
requirements:

- `esperanto` — warning about needing to type `ĉ ĝ ĥ ĵ ŝ ŭ` on every
  unlock machine; `esperanto-x` is recommended in scenarios without a
  configured Esperanto layout.
- `russian` — warning about a Cyrillic IME, `ё/е` and `й/и`
  differences after NFKC, and the danger of Latin visual clones
  (`a/а`, `o/о`, `e/е`).

### What NFKC does and does not solve

NFKC normalization is applied to the phrase before it enters Argon2id,
so different ways of entering **the same code point** yield the same
key:

- `ĉ` (U+0109) ≡ `c`+U+0302 (decomposition).
- `й` (U+0439) ≡ `и`+U+0306.
- `ё` (U+0451) ≡ `е`+U+0308.
- Ligatures `ﬁ` (U+FB01) → `fi`, full-width `ｐａｓｓ` → `pass`, etc.

NFKC does **not** equate:

- `cx` and `ĉ` — these are different Esperanto orthographic systems
  (x-system vs supersignoj); pick one dictionary once and do not mix.
- `ё` and `е` — different code points; phrases `ещё` and `еще` yield
  **different** keys. The same holds for `й` and `и`.
- Visual clones: Cyrillic `а` (U+0430) and Latin `a` (U+0061) are
  different letters. Keep one layout when entering the password.

If the phrase is lost — it is lost permanently (see §“Recovering
access if the password is lost”).

### Examples

```bash
# 7-word EFF Long, space separator (default, ~90 bits).
./build/monujo tool genpass

# 8 words + 2 trailing digits, first letters capitalized.
./build/monujo tool genpass --words 8 --capitalize --digits 2

# Russian dictionary, 7 words, hyphen separator.
./build/monujo tool genpass --dict russian --separator -

# Create a new wallet immediately with a generated password.
./build/monujo wallet create --name cold --generate-passphrase \
    --generate-words 8 --generate-dict english
```

## Changing the passphrase

There is no direct CLI mechanism to change the password yet. Temporary
workaround: decrypt the key (via a `LoadPrivateKey` equivalent —
impossible from the shell, that is an API), or when creating a new
wallet save `privHex` once, recreate via `wallet import` with a new
phrase, and securely delete the old file. A separate `wallet rekey`
command is a planned follow-up.

## What is **not** in the keystore

- The passphrase. The program never saves it to disk.
- Third-party addresses (recipients). The program does not keep an
  “address book”.
- Operation logs. monujo writes no journals except updating
  `networks`.
- Any non-Tron keys (Solana/BEP — out of scope for the current
  version).

## Compatibility with other keystores

The monujo format is **not** directly compatible with the go-ethereum
keystore or with TronLink (own format). For migration you can:

- from go-ethereum: decrypt there, get the hex key, import via
  `wallet import --key-hex`;
- from TronLink/mnemonics: export the private key of the needed
  account (via extension / wallet CLI), import.

`monujo` intentionally works only with “bare” secp256k1 private keys
— no BIP39, no HD derivation.
