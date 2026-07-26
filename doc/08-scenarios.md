# 08. Typical scenarios

This section contains step-by-step recipes for common tasks. All
examples assume the binary is at `./build/monujo` (after `make build`);
if you installed it on `PATH`, replace that with plain `monujo`.

## Scenario 1. Check a cold address balance

Suitable for auditing cold storage from a normal online machine.
No keys are required.

```bash
./build/monujo watch balance --addr TXYZ...
```

If you want machine-readable data:

```bash
./build/monujo --output json watch balance --addr TXYZ... | jq '.usdt'
```

Additionally — recent USDT transfer history (up to 50 records):

```bash
./build/monujo watch txs --addr TXYZ... --limit 50
```

With a TronGrid API key the rate limit is substantially higher:

```bash
export MONUJO_TRONGRID_API_KEY="..."
./build/monujo watch txs --addr TXYZ... --limit 200
```

## Scenario 2. Create a new hot wallet and send it the first TRX

```bash
# 1. Create a wallet (mainnet by default). The `wallet` command is network-free.
./build/monujo wallet create --name myhot
# New keystore passphrase: ******
# Confirm passphrase: ******
# Created wallet "myhot" -> TLcVUPa...
```

Variant that prints the private key once (needed if you want to move
the wallet into another tool):

```bash
./build/monujo wallet create --name myhot --export-private-key --allow-private-key-export
# WARNING: printing private key is unsafe; ...
# Private key (hex): 7b8a...
```

On TRON an account must receive its first incoming TRX to appear in
the read API. Send a small amount from an already funded wallet:

```bash
./build/monujo hot send-trx \
  --from funding --to TLcVUPa... --amount 1
# (prompts for the funding passphrase)
# Broadcast OK ...
```

After that, `watch balance` and `hot balance` for the new address will
start showing meaningful values (previously they returned zeros).

## Scenario 3. Import an existing key

```bash
./build/monujo wallet import \
  --name imported --key-hex 7b8a...c9d0
# Passphrase twice.
```

It is safer not to put the key on the command line (it stays in shell
history). Use a variable:

```bash
read -rs MONUJO_IMPORT_KEY    # input without echo
export MONUJO_IMPORT_KEY
./build/monujo wallet import --name imported
unset MONUJO_IMPORT_KEY
history -d -1 -2              # delete recent history lines (zsh/bash)
```

## Scenario 4. Check a named wallet balance

```bash
./build/monujo hot balance --addr myhot
# TRX: ...
# USDT: ...
```

Unlike `watch balance`, you do not need to remember the address — it
is read from the keystore file’s public metadata. A passphrase is
**not required** for this command: `hot balance` is a read-only
operation; keystore decryption happens only when signing (`hot send-*`,
`offline sign`).

## Scenario 5. Send TRX

```bash
./build/monujo hot send-trx \
  --from myhot --to TXYZ... --amount 12.5
# Passphrase: ...
# Broadcast OK
# Network: mainnet
# TxID:    abc123...
```

If `myhot` has no TRX yet, broadcast will fail. Check the balance
before sending:

```bash
./build/monujo hot balance --addr myhot
```

## Scenario 6. Send USDT

```bash
./build/monujo hot send-usdt \
  --from myhot --to TXYZ... --amount 10.25
```

Requirements:

- `myhot` has **TRX** (ideally at least 5–10 TRX in reserve for
  energy/bandwidth);
- `myhot` has enough USDT;
- The USDT contract is available on the current `--network` (mainnet —
  Tether, Shasta — the test token).

## Scenario 7. Testing on Shasta

First “fund” the new address with TRX via the
[Shasta faucet](https://www.trongrid.io/faucet) (no manual activation):

```bash
./build/monujo --network shasta wallet create --name testhot
# Address TLcVUPa...
# Copy it and request TRX from the faucet in a browser.
```

Check after the funds arrive:

```bash
./build/monujo --network shasta hot balance --addr testhot
```

Sending test TRX/USDT:

```bash
./build/monujo --network shasta hot send-trx \
  --from testhot --to TXYZ... --amount 1

./build/monujo --network shasta hot send-usdt \
  --from testhot --to TXYZ... --amount 1
```

On the first `hot send-*` after `wallet create` the network already
matches (the file already has `shasta` recorded), so there will be no
confirmation prompt.

## Scenario 8. The same wallet on mainnet and Shasta

Because a private key yields the same address on both networks, one
keystore file serves both. The `networks` metadata exists only to warn
you about a change.

On the first attempt to use a new network:

```bash
./build/monujo --network shasta hot balance --addr myhot
# Warning: wallet "myhot" was previously used on network(s): mainnet
# The current operation uses network "shasta", which is not in that list.
# Continue? [y/N]: y
# ...
```

After a successful `hot send-*`, `shasta` is added to `networks` and
the warning goes away.

## Scenario 9. Scripted use without a TTY

In non-interactive modes:

- The passphrase is read from STDIN (one line), but without hidden
  input (in `passphrase.go` this is the fallback path).
- Network confirmation **will not work interactively**. If the file is
  already recorded for one network and you run on another without
  `--force`, the program exits with
  `network mismatch: use --force ...`.
- The fix is to add `--force` when you are sure the network is chosen
  correctly.

Example script that reads the password from an environment variable:

```bash
#!/usr/bin/env bash
set -euo pipefail

: "${MONUJO_PASS:?need MONUJO_PASS}"

printf '%s\n' "$MONUJO_PASS" | \
  ./build/monujo --output json --network shasta \
    hot send-trx --from testhot --to TXYZ... --amount 1 --force \
  | jq '.tx_id'
```

> **Caution**: the program reads the line twice for `wallet create` and
> `wallet import` (new passphrase + confirmation). Feed it on STDIN
> twice in a row:
> ```bash
> printf '%s\n%s\n' "$MONUJO_PASS" "$MONUJO_PASS" | \
>   ./build/monujo wallet create --name myhot
> ```

## Scenario 10. Moving a keystore between machines

1. On the source machine find the file: `~/.config/monujo/keystore/<name>.json`
   (or whatever `--keystore-dir` points to).
2. Copy it to the target machine **over a secure channel** (scp/rsync
   over ssh, a USB stick, a gpg-encrypted archive).
3. Place it in `~/.config/monujo/keystore/` (or point at your own directory).
4. Install/build monujo on the target machine.
5. Use the usual commands: the passphrase is the same.

If you like, you can rename the file first — the important part is that
the file name matches what you will pass to `--name` (subject to
`safeName` sanitization).

## Scenario 11. List all wallets on the machine

```bash
./build/monujo wallet list
```

Machine-readable:

```bash
./build/monujo --output json wallet list \
  | jq '.wallets[] | {name, address, networks}'
```

Handy for a quick reminder of which local keys you have and which
networks they have already seen.

## Scenario 12. Sanity check after installation

```bash
# 1. Version — prints what the Makefile embeds via -ldflags.
./build/monujo --version

# 2. Help for all commands.
./build/monujo --help
./build/monujo watch --help
./build/monujo hot --help

# 3. A watch query against a public address (e.g. the USDT contract).
./build/monujo watch balance --addr TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t
```

If the last step returns a meaningful balance/zeros, the network is
reachable and the binary works.
