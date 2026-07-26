# 08. Tipaj scenaroj

Ĉi tiu sekcio enhavas paŝ-post-paŝajn receptojn por oftaj taskoj. Ĉiuj
ekzemploj supozas, ke la binaraĵo estas ĉe `./build/monujo` (post `make build`);
se vi instalas ĝin en `PATH`, anstataŭigu tion per simpla `monujo`.

## Scenaro 1. Kontroli saldon de malvarma adreso

Taŭgas por aŭditado de malvarma stokado de normala reta maŝino.
Neniuj ŝlosiloj estas bezonataj.

```bash
./build/monujo watch balance --addr TXYZ...
```

Se vi volas maŝinlegeblajn datumojn:

```bash
./build/monujo --output json watch balance --addr TXYZ... | jq '.usdt'
```

Aldone — lastatempa historio de USDT-transigoj (ĝis 50 registroj):

```bash
./build/monujo watch txs --addr TXYZ... --limit 50
```

Kun API-ŝlosilo de TronGrid la limo de petoj estas esence pli alta:

```bash
export MONUJO_TRONGRID_API_KEY="..."
./build/monujo watch txs --addr TXYZ... --limit 200
```

## Scenaro 2. Krei novan varman monujon kaj sendi al ĝi la unuajn TRX

```bash
# 1. Create a wallet (mainnet by default). The `wallet` command is network-free.
./build/monujo wallet create --name myhot
# New keystore passphrase: ******
# Confirm passphrase: ******
# Created wallet "myhot" -> TLcVUPa...
```

Varianto, kiu printas la privatan ŝlosilon unufoje (necesa se vi volas
movi la monujon en alian ilon):

```bash
./build/monujo wallet create --name myhot --export-private-key --allow-private-key-export
# WARNING: printing private key is unsafe; ...
# Private key (hex): 7b8a...
```

Sur TRON konto devas ricevi sian unuan envenantan TRX por aperi en
la lega API. Sendu malgrandan kvanton de jam financita monujo:

```bash
./build/monujo hot send-trx \
  --from funding --to TLcVUPa... --amount 1
# (prompts for the funding passphrase)
# Broadcast OK ...
```

Post tio `watch balance` kaj `hot balance` por la nova adreso komencos
montri signifajn valorojn (antaŭe ili redonis nulojn).

## Scenaro 3. Importi ekzistantan ŝlosilon

```bash
./build/monujo wallet import \
  --name imported --key-hex 7b8a...c9d0
# Passphrase twice.
```

Estas pli sekure ne meti la ŝlosilon sur la komandlinion (ĝi restas en
la historio de la ŝelo). Uzu variablon:

```bash
read -rs MONUJO_IMPORT_KEY    # input without echo
export MONUJO_IMPORT_KEY
./build/monujo wallet import --name imported
unset MONUJO_IMPORT_KEY
history -d -1 -2              # delete recent history lines (zsh/bash)
```

## Scenaro 4. Kontroli saldon de nomita monujo

```bash
./build/monujo hot balance --addr myhot
# TRX: ...
# USDT: ...
```

Male al `watch balance`, vi ne bezonas memori la adreson — ĝi
estas legata el la publikaj metadatenoj de la ŝlosiluja dosiero. Pasfrazo
**ne estas bezonata** por ĉi tiu komando: `hot balance` estas nur-lega
operacio; deĉifrado de la ŝlosilujo okazas nur ĉe subskribado (`hot send-*`,
`offline sign`).

## Scenaro 5. Sendi TRX

```bash
./build/monujo hot send-trx \
  --from myhot --to TXYZ... --amount 12.5
# Passphrase: ...
# Broadcast OK
# Network: mainnet
# TxID:    abc123...
```

Se `myhot` ankoraŭ ne havas TRX, la elsendo malsukcesos. Kontrolu la
saldon antaŭ ol sendi:

```bash
./build/monujo hot balance --addr myhot
```

## Scenaro 6. Sendi USDT

```bash
./build/monujo hot send-usdt \
  --from myhot --to TXYZ... --amount 10.25
```

Postuloj:

- `myhot` havas **TRX** (ideale almenaŭ 5–10 TRX rezervitaj por
  energio/bendlarĝo);
- `myhot` havas sufiĉe da USDT;
- La USDT-kontrakto disponeblas sur la nuna `--network` (mainnet —
  Tether, Shasta — la testĵetono).

## Scenaro 7. Testado sur Shasta

Unue «financu» la novan adreson per TRX per la
[Shasta-fonto](https://www.trongrid.io/faucet) (neniu mana aktivigo):

```bash
./build/monujo --network shasta wallet create --name testhot
# Address TLcVUPa...
# Copy it and request TRX from the faucet in a browser.
```

Kontrolu post alveno de la monrimedoj:

```bash
./build/monujo --network shasta hot balance --addr testhot
```

Sendado de testaj TRX/USDT:

```bash
./build/monujo --network shasta hot send-trx \
  --from testhot --to TXYZ... --amount 1

./build/monujo --network shasta hot send-usdt \
  --from testhot --to TXYZ... --amount 1
```

Ĉe la unua `hot send-*` post `wallet create` la reto jam kongruas
(la dosiero jam havas `shasta` registrita), do ne estos
konfirma invito.

## Scenaro 8. La sama monujo sur mainnet kaj Shasta

Ĉar privata ŝlosilo donas la saman adreson sur ambaŭ retoj, unu
ŝlosiluja dosiero servas ambaŭ. La metadatenoj `networks` ekzistas nur por
averti vin pri ŝanĝo.

Ĉe la unua provo uzi novan reton:

```bash
./build/monujo --network shasta hot balance --addr myhot
# Warning: wallet "myhot" was previously used on network(s): mainnet
# The current operation uses network "shasta", which is not in that list.
# Continue? [y/N]: y
# ...
```

Post sukcesa `hot send-*`, `shasta` estas aldonita al `networks` kaj
la averto malaperas.

## Scenaro 9. Skriptita uzo sen TTY

En neinteragaj reĝimoj:

- La pasfrazo estas legata el STDIN (unu linio), sed sen kaŝita
  enigo (en `passphrase.go` tio estas la rezervovojo).
- Reta konfirmo **ne funkcios interage**. Se la dosiero jam
  estas registrita por unu reto kaj vi rulas sur alia sen
  `--force`, la programo eliras kun
  `network mismatch: use --force ...`.
- La solvo estas aldoni `--force` kiam vi certas, ke la reto estas
  ĝuste elektita.

Ekzempla skripto, kiu legas la pasvorton el medivariablo:

```bash
#!/usr/bin/env bash
set -euo pipefail

: "${MONUJO_PASS:?need MONUJO_PASS}"

printf '%s\n' "$MONUJO_PASS" | \
  ./build/monujo --output json --network shasta \
    hot send-trx --from testhot --to TXYZ... --amount 1 --force \
  | jq '.tx_id'
```

> **Atentu**: la programo legas la linion dufoje por `wallet create` kaj
> `wallet import` (nova pasfrazo + konfirmo). Liveru ĝin al STDIN
> dufoje sinsekve:
> ```bash
> printf '%s\n%s\n' "$MONUJO_PASS" "$MONUJO_PASS" | \
>   ./build/monujo wallet create --name myhot
> ```

## Scenaro 10. Movado de ŝlosilujo inter maŝinoj

1. Sur la fonta maŝino trovu la dosieron: `~/.config/monujo/keystore/<name>.json`
   (aŭ kien ajn `--keystore-dir` indikas).
2. Kopiu ĝin al la cela maŝino **per sekura kanalo** (scp/rsync
   tra ssh, USB-bastono, gpg-ĉifrita arkivo).
3. Metu ĝin en `~/.config/monujo/keystore/` (aŭ indiku vian propran dosierujon).
4. Instalu/konstruu monujo sur la cela maŝino.
5. Uzu la kutimajn komandojn: la pasfrazo estas la sama.

Se vi volas, vi povas unue renomi la dosieron — la grava parto estas, ke
la dosiernomo kongruu kun tio, kion vi pasigos al `--name` (kun
sanigado per `safeName`).

## Scenaro 11. Listigi ĉiujn monujojn sur la maŝino

```bash
./build/monujo wallet list
```

Maŝinlegeble:

```bash
./build/monujo --output json wallet list \
  | jq '.wallets[] | {name, address, networks}'
```

Utila por rapida memorigo, kiujn lokajn ŝlosilojn vi havas kaj kiujn
retojn ili jam vidis.

## Scenaro 12. Sanecekontrolo post instalado

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

Se la lasta paŝo redonas signifan saldon/nulojn, la reto estas
atingebla kaj la binaraĵo funkcias.
