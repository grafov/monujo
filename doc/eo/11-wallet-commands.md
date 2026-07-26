# 11. Komandoj `wallet`

La grupo `wallet` administras la lokan dosierujon de la **ŝlosilujo**:
ĉifritajn monujojn kun privata ŝlosilo, kaj **eksterajn**
adres-nurajn enskribojn (sen ŝlosilo) por oportuna nomado de fremdaj
adresoj. Ĝi listigas enskribojn kaj kontrolas la paron «adreso en
dosiero — pasfrazo» por lokaj monujoj. **Ĉi tiuj komandoj ne faras
retajn alvokojn** — tial ili funkcias same sur reta komputilo kaj sur
aer-interspaca maŝino. La samajn dosierojn poste uzas `monujo hot`
(retaj operacioj) kaj `monujo offline sign` (subskribado sur izolita
maŝino).

```text
monujo wallet
├── list               — montri ŝlosilujajn enskribojn (lokajn kaj eksterajn)
├── create             — krei novan monujon (hazarda ŝlosilo)
├── import             — importi ekzistantan ŝlosilon (hex)
├── import-external    — stoki Tron-adreson sub nomo (sen privata ŝlosilo)
└── check              — kontroli pasfrazon de loka monujo (sen reto)
```

Globalaj flagoj (`--network`, `--output`, `--keystore-dir`, kaj tiel
plu) aplikiĝas al ĉiuj subkomandoj. Detalojn vidu en
[03 — Globala agordo](03-configuration.md).

## Oftaj principoj

- **Pasfrazo**. `wallet create` kaj `wallet import` petas pasfrazon
  per TTY sen eĥo — dufoje sinsekve por konfirmo. Alternative,
  `--generate-passphrase` generas hazardan frazon kaj printas ĝin
  unufoje al stderr (vidu
  [06 — Ŝlosilujo → «Generilo de pasfrazoj»](06-keystore.md#generilo-de-pasfrazoj)).
  **`wallet import-external` ne petas pasvorton** — la dosiero
  enhavas neniun ŝlosilan materialon.
- **Viŝado de sekretoj**. La pasfrazo kaj la materialo de la privata
  ŝlosilo estas nuliĝitaj (`ZeroPassphrase`, `Wipe`) tuj post uzo.
- **Skribo de metadatenoj**. Ĉe kreaĵo/importo la nuna `--network`
  estas skribita en la metadatenan tabelon `networks` de la dosiero.
  Tiu enskribo poste estas uzata de komandoj `hot` por kontroli
  retan kongruecon.
- **Nomo de monujo** (`--name`) estas sanigita: nur `a-z`, `A-Z`,
  `0-9`, `-` kaj `_` estas permesataj. Aliaj signoj estas anstataŭigitaj
  per `_`. Malplena nomo fariĝas `default` (do `--name " "` kreas
  `default.json`).
- **La nomo ne rajtas aspekti kiel Tron-base58-adreso**. Se la
  tondita ĉeno pasas la saman heŭristikon kiel por `--from` /
  `--to` / `--addr`, `wallet create`, `wallet import` kaj
  `wallet import-external` finiĝas kun eraro (antaŭ la pasfraza
  peto, `--overwrite` kaj iu ajn skribo de dosiero), por ke dosiernomo
  ne estu konfuzita kun adreso en aliaj komandoj.
- **`--addr` en `wallet check`**. La valoro estas interpretata same
  kiel en `hot balance` kaj `offline sign`: ĉeno kiu aspektas kiel
  Tron-base58-adreso estas serĉata inter ŝlosilujaj enskriboj laŭ la
  kampo `address` en JSON; alie la ĉeno estas traktata kiel **nomo**
  de monujo. Kontrolo postulas **lokan ĉifritan** monujon:
  enskriboj **`import-external` (eksteraj) estas malakceptitaj** kun
  klara eraro, ĉar la dosiero ne enhavas privatan ŝlosilon.

---

## `wallet list`

Montras ĉiujn enskribojn en la ŝlosiluja dosierujo: nomo, **tipo**
(`local` — ĉifrita ŝlosilo, `external` — nur adreso), Tron-adreso, kaj
la listo de retoj el metadatenoj. **Malĉifrado ne okazas**; pasfrazo
ne estas bezonata.

### Uzo

```bash
monujo wallet list
monujo --output json wallet list
```

### Flagoj

Ne estas lokaj flagoj. La dosierujo estas legata el
`--keystore-dir`/`MONUJO_KEYSTORE_DIR`/`~/.config/monujo/keystore`.

### Kio okazas

1. La ŝlosiluja dosierujo estas malfermita. Se ĝi ne ekzistas, malplena
   listo estas printita (sen eraro).
2. Ĉiuj dosieroj `*.json` estas iteraciitaj (subdosierujoj estas
   preterlasitaj).
3. Ĉiu dosiero estas analizata kiel monuja JSON: aŭ ĉifrita loka
   dosiero (v1/v2 kun KDF), aŭ minimuma ekstera dosiero kun
   `"external": true` (vidu [06 — Ŝlosilujo](06-keystore.md)).
   Nevalidaj dosieroj estas preterlasitaj sen eraro.
4. Enskriboj estas ordigitaj laŭ nomo.

### Ekzemplo: formato `table`

```text
$ monujo wallet list
Keystore: /home/user/.config/monujo/keystore
NAME      TYPE      ADDRESS               NETWORKS
funding   local     TFG7NgT...            mainnet,shasta
custodian external  TXYZ...               mainnet
imported  local     TXYZ...               -
myhot     local     TLcVUPa...            shasta
```

Kolumnoj estas tab-apartigitaj. Se `networks` estas malplena, `-`
estas printita. Se la dosierujo estas malplena — `No wallets.`.

### Ekzemplo: formato `json`

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

La kampo **`external`** ĉiam ĉeestas en JSON (`true` / `false`).
`networks` estas preterlasita el JSON kiam malplena (etikedo
`omitempty`).

---

## `wallet create`

Kreas **novan** monujon: generas hazardan secp256k1-ŝlosilon, derivas
Argon2id-ŝlosilon el la pasfrazo, ĉifras la privatan ŝlosilon per
AES‑256‑GCM, kaj skribas la dosieron kun reĝimo `0600`. La nuna
`--network` estas aŭtomate stokita en la metadatenojn `networks` de
la nova dosiero.

### Uzo

```bash
monujo wallet create --name <NAME> [--export-private-key --allow-private-key-export]
monujo --network shasta wallet create --name testhot
monujo --output json wallet create --name myhot --export-private-key --allow-private-key-export
```

### Flagoj

| Flago | Tipo | Bezonata | Defaŭlto | Priskribo |
|------|------|----------|---------|-------------|
| `--name` | string | jes | — | Nomo de monujo. Nur ASCII-literoj, ciferoj, `-` kaj `_` estas permesataj; la nomo ne rajtas esti valida Tron-base58-adreso. |
| `--export-private-key` | bool | ne | `false` | Printi la generitan privatan ŝlosilon unufoje. **Danĝere**: la ŝlosilo aperos en terminala eligo aŭ JSON-fluo. |
| `--allow-private-key-export` | bool | ne | `false` | Bezonata riska agnosko por `--export-private-key`. |
| `--overwrite` | bool | ne | `false` | **DANĜERE**: permesi anstataŭigon de ekzistanta monuja dosiero. La malnova privata ŝlosilo estas porĉiam perdita. Sur TTY ekstra konfirmo `[y/N]` estas petata; sen TTY la anstataŭigo procedas sen tiu ekstra peto. |
| `--generate-passphrase` | bool | ne | `false` | Generi hazardan pasfrazon kaj printi ĝin unufoje anstataŭ interaga enigo. |
| `--generate-words` | int | ne | `7` | Nombro de vortoj en la generita frazo (≥ 4). |
| `--generate-dict` | string | ne | `english` | Vortaro: `english`, `english-short`, `esperanto-x`, `esperanto`, `russian`. |
| `--generate-separator` | string | ne | ` ` | Apartigilo inter vortoj. |
| `--generate-capitalize` | bool | ne | `false` | Majuskligi la unuan literon de ĉiu vorto. |
| `--generate-digits` | int | ne | `0` | Aldoni N hazardajn dekumajn ciferojn ĉe la fino de la frazo. |
| `--generate-allow-non-tty` | bool | ne | `false` | Permesi printadon de la generita frazo al ne-TTY stderr (**nesekure**). |

### Kio okazas

1. Nova pasfrazo estas petata dufoje (aŭ generita per
   `--generate-passphrase`). Ĉe malkongruo — eraro
   `passphrases do not match`.
2. Generado de ŝlosilo: `crypto.GenerateKey()` el go‑ethereum.
3. La Tron-adreso estas derivita el la publika ŝlosilo.
4. Ĉifrado: `argon2id(passphrase, random_salt)` →
   `AES-256-GCM(nonce=random_12, plaintext=raw_priv_32)`.
5. Dosiero `<keystore_dir>/<safeName>.json` estas kreita kun reĝimo
   `0600`. Se la dosierujo ankoraŭ ne ekzistas, ĝi estas kreita kun
   reĝimo `0700`.

### Ekzemplo: formato `table`

```text
$ monujo wallet create --name myhot
New keystore passphrase: ********
Confirm passphrase: ********
Created wallet "myhot" -> TLcVUPa...
Keystore: /home/user/.config/monujo/keystore
```

Kun `--export-private-key --allow-private-key-export`:

```text
$ monujo wallet create --name myhot --export-private-key --allow-private-key-export
...
WARNING: printing private key is unsafe; anyone with this hex can spend funds.
Private key (hex): 7b8a...c9d0
```

### Ekzemplo: formato `json`

```json
{
  "name": "myhot",
  "address": "TLcVUPa...",
  "path": "/home/user/.config/monujo/keystore"
}
```

Kun `--export-private-key --allow-private-key-export`:

```json
{
  "name": "myhot",
  "address": "TLcVUPa...",
  "path": "/home/user/.config/monujo/keystore",
  "private_key_hex": "7b8a...c9d0"
}
```

### Gravaj notoj

- La adreso derivita el la ŝlosilo validas en **ambaŭ** retoj
  (mainnet kaj Shasta). La etikedo `--network` nur kontrolas, kiu
  reto estas skribita en la metadatenojn `networks` kaj kiu estas
  uzata defaŭlte por postaj komandoj.
- La dosiero estas kreita per rekta `os.WriteFile` (ne atome). Se
  la procezo estas interrompita inter generado de la ŝlosilo kaj
  skribo de la dosiero, la ŝlosilo estas perdita, ĉar la privata
  ŝlosilo ĵus estis generita nur en memoro kaj ne estas stokita
  aliloke.
- Se dosiero kun tiu nomo **jam ekzistas**, la komando defaŭlte
  finiĝas kun eraro `wallet "<name>" already exists at <path>;
  use --overwrite to replace (DANGEROUS, irreversible)` antaŭ la
  pasfraza peto. Anstataŭigo estas permesata nur kun eksplicita
  `--overwrite`: sur TTY ekstra `[y/N]` estas petata; en
  neinteraga reĝimo (skripto, CI, redirektita stdin) la anstataŭigo
  procedas sen tiu ekstra konfirmo — la flago mem estas traktata
  kiel konfirmo. Privata ŝlosilo perdita tiel estas **ne
  restaŭrebla**.

---

## `wallet import`

Importas ekzistantan privatan ŝlosilon (64 deksesumaj signoj) en novan
ŝlosilujan dosieron. La nuna `--network` estas skribita en
metadatenojn.

### Uzo

```bash
monujo wallet import --name <NAME> --key-hex <64_HEX>
# aŭ
MONUJO_IMPORT_KEY=<64_HEX> monujo wallet import --name <NAME>
monujo --network shasta wallet import --name testhot --key-hex 0x...
```

### Flagoj

| Flago          | Tipo   | Bezonata | Defaŭlto | Priskribo                                                                                                                                                           |
|---------------|--------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--name`      | string | jes      | —       | Nomo de ŝlosiluja dosiero. Ne rajtas esti ĉeno rekonata kiel valida Tron-base58-adreso (vidu oftajn principojn).                                                           |
| `--key-hex`   | string | ne       | —       | 64 deksesumaj signoj de secp256k1-privata ŝlosilo. Nedeviga prefikso `0x`.                                                                                                   |
| `--overwrite` | bool   | ne       | `false` | **DANĜERE**: permesi anstataŭigon de ekzistanta monuja dosiero. La malnova privata ŝlosilo estas porĉiam perdita. Sur TTY — kun konfirmo `[y/N]`; sen TTY — sen.       |
| `--generate-passphrase` kaj rilataj `--generate-*` | — | ne | — | La samaj flagoj de generado de pasfrazo kiel ĉe `wallet create`. |

Se `--key-hex` ne estas agordita, la programo prenas la ŝlosilon el la
medivariablo `MONUJO_IMPORT_KEY`. Se nek unu nek la alia ĉeestas —
eraro:

```
provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key
```

### Kio okazas

1. La deksesuma ŝlosilo estas analizata: longo (ĝuste 64 signoj) kaj
   valideco (valida ECDSA secp256k1, ne ĉiuj nuloj) estas kontrolataj.
2. Se dosiero kun tiu nomo jam ekzistas en la ŝlosilujo, la
   komando rifuzas daŭrigi sen `--overwrite` (same kiel
   `wallet create`; vidu la respondan sekcion supre).
3. La pasvorto estas petata dufoje (aŭ generita per
   `--generate-passphrase`).
4. Ĉifrado kaj stokado funkcias same kiel en `wallet create`.
5. La adreso en la dosiero estas derivita determinisme el la ŝlosilo.

### Ekzemplo: formato `table`

```text
$ monujo wallet import --name imported --key-hex 7b8a...c9d0
New keystore passphrase: ********
Confirm passphrase: ********
Imported wallet "imported" -> TXYZ...
Keystore: /home/user/.config/monujo/keystore
```

### Ekzemplo: formato `json`

```json
{
  "name": "imported",
  "address": "TXYZ...",
  "path": "/home/user/.config/monujo/keystore"
}
```

### Sekureco

- Ŝlosilo el medivariablo povas finiĝi en protokoloj kaj ŝela
  historio. Uzu `MONUJO_IMPORT_KEY` nur en kontrolita medio
  (unufoja asigno, sen skribo al `.bash_history`).
- En reĝimo `table` la ŝlosilo mem neniam estas printita; en reĝimo
  `json` ankaŭ ne — `import` ne havas ekvivalenton de
  `--export-private-key`.

---

## `wallet import-external`

Stokas **nur** Tron-base58-adreson sub la elektita dosiernomo en
la ŝlosilujo. En la dosiero ne estas privata ŝlosilo; la enskribo estas
markita `"external": true` kaj aperas kiel **external** en
`wallet list`.

Tia kaŝnomo uzeblas ĉie, kie la CLI akceptas monujan nomon aŭ adreson
por anstataŭigo de adreso (ekzemple `hot balance --addr`,
`hot send-* --to`, `offline build --from` / `--to`). Komandoj kiuj
bezonas privatan ŝlosilon (`hot send-* --from`, `offline sign`,
`multisig cosign`, `wallet check`) finiĝas kun eraro por ekstera
enskribo **antaŭ** la pasfraza peto.

La nuna `--network` estas skribita en la metadatenan tabelon
`networks` (kiel ĉe `wallet create` / `wallet import`), por ke `hot`
povu apliki la saman kontrolon de ret-malkongruo kiel por lokaj
monujoj.

### Uzo

```bash
monujo wallet import-external --name <NAME> --addr <TRON_BASE58>
monujo --network shasta wallet import-external --name payer --addr T...
```

### Flagoj

| Flago          | Tipo   | Bezonata | Defaŭlto | Priskribo |
|---------------|--------|----------|---------|-------------|
| `--name`      | string | jes      | —       | Dosiernomo en la ŝlosilujo (`<safeName>.json`). Ne rajtas esti ĉeno rekonata kiel valida Tron-base58-adreso (vidu oftajn principojn). |
| `--addr`      | string | jes      | —       | Tron-base58-adreso (validigita loke antaŭ skribo). |
| `--overwrite` | bool   | ne       | `false` | **DANĜERE**: anstataŭigi ekzistantan dosieron. Se ĝi estis loka monujo, la privata ŝlosilo estas porĉiam perdita. |

### Ekzemplo: formato `json`

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

Kontrolas **senrete** (sen RPC kaj sen elsendo de transakcio), ke la
enigita pasfrazo malĉifras lokan monujan dosieron kaj ke la
malĉifrita ŝlosilo donas la saman Tron-adreson kiel stokita en la
kampo `address` de tiu dosiero. Utila por kontroli rezervkopion: ke
dosiernomo, registrita adreso kaj pasfrazo ne estas miksitaj.

**Enskriboj `import-external` ne estas subtenataj**: ili ne havas
ŝlosilon por malĉifri — la komando finiĝas kun eraro antaŭ la
pasfraza peto.

Male al `wallet list`, la komando **faras plenan malĉifradon**
(la ŝlosilo ekzistas momente en memoro, poste estas viŝita). Male al
`hot balance`, ĝi ne kontaktas la reton.

Se la dosiero estas ankoraŭ en formato v1 (scrypt), sukcesa kontrolo
povas **ekigi aŭtomatan migradon** al v2 (argon2id), kiel iu ajn alia
operacio kiu uzas `LoadPrivateKey` — vidu
[06 — Ŝlosilujo → Migrado v1 → v2](06-keystore.md#migrado-v1--v2).

### Uzo

```bash
monujo wallet check --addr <WALLET_NAME>
monujo wallet check --addr T...   # se la adreso aperas en wallet list
monujo --output json wallet check --addr myhot
```

### Flagoj

| Flago     | Tipo   | Bezonata | Defaŭlto | Priskribo |
|----------|--------|----------|---------|-------------|
| `--addr` | string | jes      | —       | Nomo de **loka** monujo en la ŝlosilujo **aŭ** ĝia Tron-base58-adreso. Eksteraj (`import-external`) enskriboj estas malakceptitaj. |

### Kio okazas

1. La ŝlosiluja enskribo por `--addr` estas trovita (nur legado de
   listaj metadatenoj). Se la enskribo estas **ekstera** — tuja eraro.
2. La pasfrazo estas petata unufoje (TTY sen eĥo; en ne-TTY
   la frazo povas esti pasita per stdin; la fina novlinio estas
   tondita).
3. `LoadPrivateKey` ruliĝas: KDF, AES-GCM, kaj kontrolo ke la derivita
   adreso kongruas kun la kampo `address` en JSON.
4. Ĉe sukceso — mallonga mesaĝo al stdout (aŭ JSON kun kampoj
   `ok`, `wallet`, `address` kiam `--output json`).

Ĉe malĝusta frazo aŭ difektita dosiero la komando finiĝas kun eraro
(ekzemple, `decryption failed (wrong passphrase or corrupt file)`).

---

## Resuma tabelo de ĉiuj flagoj `wallet`

| Subkomando | Lokaj flagoj                                                | Petas pasvorton                                 |
|------------|------------------------------------------------------------|---------------------------------------------------|
| `list`     | —                                                          | ne                                                |
| `create`   | `--name`, `--export-private-key`, `--allow-private-key-export`, `--overwrite`, `--generate-*` | jes (× 2 — nova frazo, aŭ generado)          |
| `import`   | `--name`, `--key-hex`, `--overwrite`, `--generate-*`       | jes (× 2 — nova frazo, aŭ generado)             |
| `import-external` | `--name`, `--addr`, `--overwrite` | ne |
| `check`    | `--addr`                                                   | jes (unufoje — ekzistanta frazo)                      |
