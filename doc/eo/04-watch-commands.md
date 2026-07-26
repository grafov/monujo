# 04. Komandoj `watch`

La grupo `watch` estas **nur-legaj** operacioj: la programo parolas
nur al la publika HTTP-API de TronGrid kaj ne tuŝas privatajn
ŝlosilojn. Neniuj dosieroj estas skribataj. Taŭgas por:

- kontroli saldon de malvarma monujo de reta maŝino;
- vidi enirantajn/elirantajn USDT-transigojn;
- monitorajn skriptojn.

```text
monujo watch
├── balance      — TRX and USDT for an address
├── txs          — recent USDT (TRC-20) transfers
└── permissions  — owner/witness/active permissions (multisig structure)
```

Ĝeneralaj flagoj (`--network`, `--output`, `--http-url`, `--api-key`,
`--keystore-dir`, `--write-transport`) validas por ĉiuj subkomandoj
en la grupo. `--keystore-dir` bezonatas de `watch permissions` por
marki lokajn ŝlosilojn; `--grpc-url` / `--grpc-insecure` ne estas
uzataj de `watch` (nur kun `--write-transport grpc` por skribaj
komandoj).

---

## `watch balance`

Petas de TronGrid la saldon TRX en sun kaj la TRC‑20 USDT-saldon
en minimalaj unuoj (1 USDT = 1 000 000 minimalaj unuoj).

### Uzo

```bash
monujo watch balance --addr <TRON_BASE58_ADDRESS>
```

### Flagoj

| Flago    | Tipo   | Deviga | Defaŭlto | Priskribo                                                                                         |
|----------|--------|--------|----------|---------------------------------------------------------------------------------------------------|
| `--addr` | string | jes    | —        | Tron-adreso en Base58Check (komenciĝas per `T`). Validigata per `gotron-sdk` antaŭ iu ajn retpeto. |

Ĉi tiu komando ne havas aliajn lokajn flagojn.

### Kio okazas

1. La adreso estas validigata (`ValidateBase58Address`). Ĉe eraro —
   tuj eliro kun `invalid Tron address: ...`.
2. HTTP‑GET estas farata al `<HTTPBaseURL>/v1/accounts/<addr>` (kun
   la kapo `TRON-PRO-API-KEY` se agordita).
3. El la respondo:
   - `data[0].balance` — saldo TRX en sun;
   - `data[0].trc20[*]` — mapo `<contract>: <minimal_units>`.
     La programo serĉas en ĝi la USDT-kontrakton kongruan kun
     `--network` (vidu [07 — Retoj](07-networks.md)). Se tia
     ero ne ekzistas, la USDT-saldo estas traktata kiel `0`.
4. La rezulto estas formatita laŭ la elektita `--output`.

Se la adreso ankoraŭ ne estas aktivigita enĉene, TronGrid redonas
malplenan `data: []`. En tiu kazo la programo printas nulajn saldojn
(tio ne estas eraro, sed la normala stato de neaktivigita konto).

### Ekzemplo: formato `table`

```bash
$ monujo watch balance --addr TXYZ...
Address: TXYZ...
TRX:     12.345678 (12345678 sun)
USDT:    250 (minimal units 250000000)
```

- La sumo TRX estas printata kaj kiel decimalo (6 lokoj de
  precizeco, vidu `domain.FormatSunAsTRX`) kaj en sun.
- Por USDT — decimala reprezento (6 lokoj) kaj minimalaj
  unuoj.

### Ekzemplo: formato `json`

```bash
$ monujo --output json watch balance --addr TXYZ...
{
  "address": "TXYZ...",
  "trx_sun": 12345678,
  "trx": "12.345678",
  "usdt_minimal_units": "250000000",
  "usdt": "250"
}
```

JSON-kampoj estas stabilaj kaj povas esti konsumataj de skriptoj:

| Kampo                | Tipo    | Priskribo                                                              |
|----------------------|---------|------------------------------------------------------------------------|
| `address`            | string  | Adreso el la peto.                                                     |
| `trx_sun`            | integer | Saldo en sun.                                                          |
| `trx`                | string  | La sama saldo kiel decimala nombro (sen finaj nuloj).                  |
| `usdt_minimal_units` | string  | Saldo USDT en minimalaj unuoj (signoĉeno, por ne perdi precizecon).    |
| `usdt`               | string  | Decimala reprezento de la sama nombro.                                 |

### Eblaj eraroj

- `invalid Tron address: ...` — `--addr` ne validas.
- `trongrid accounts: HTTP 4xx: ...` — la nodo TronGrid redonis eraron.
  HTTP 401/403 — problemo de API-ŝlosilo; 429 — petlimo.
- `trongrid: <message>` — sukcesa HTTP-statuso, sed la respondo estas
  markita kiel malsukcesa (`success=false` kaj malplena `data`).
- `decode accounts: ...` — neatendita JSON en la respondo.

---

## `watch txs`

Redonas la lastajn konfirmitajn TRC‑20 USDT-transigojn por adreso
(ambaŭ direktoj).

### Uzo

```bash
monujo watch txs --addr <TRON_BASE58_ADDRESS> [--limit N]
```

### Flagoj

| Flago | Tipo | Deviga | Defaŭlto | Priskribo |
|------|------|----------|---------|-------------|
| `--addr` | string | jes | — | Tron-adreso en Base58Check. |
| `--limit` | int | ne | `20` | Maksimumo de transakcioj. Permesata intervalo **1..200**; valoroj ≤ 0 anstataŭiĝas per 20, > 200 per 200. |

### Kio okazas

1. Validigo de la adreso.
2. La USDT-kontrakto por `--network` estas elektita.
3. HTTP‑GET estas farata al `<HTTPBaseURL>/v1/accounts/<addr>/transactions/trc20`
   kun parametroj:
   - `only_confirmed=true` (nur konfirmitaj),
   - `limit=<N>`,
   - `contract_address=<USDT address>`.
4. Ĉiu registro estas konvertita al la interna
   modelo `TRC20TransferRow` kaj formatita kiel `table` aŭ `json`.

Konduto laŭ «direkto» de transigo: TronGrid redonas ĉiujn
transigojn kie la adreso estas la sendanto **aŭ** la ricevanto; la
direkto videblas el la kampoj `from` kaj `to`. La kampo `type` alvenas
kiel signoĉeno el TronGrid (ekz. `"Transfer"`); monujo ne interpretas ĝin.

### Ekzemplo: formato `table`

```text
$ monujo watch txs --addr TXYZ... --limit 3
2026-04-19T08:11:23Z  abc123...  TFROM... -> TXYZ...   1500000000 USDT  [Transfer]
2026-04-18T10:02:11Z  def456...  TXYZ... -> TANOTHER...  250000000 USDT  [Transfer]
2026-04-17T22:55:01Z  789aaa...  TFROM2... -> TXYZ...    50000000 USDT  [Transfer]
```

Kampoj laŭ ordo: tempo en RFC3339, transakcia id, sendanto,
ricevanto, sumo en **minimalaj unuoj** (ne dividita per 10^6),
ĵetona simbolo, kaj en kvadrataj krampoj — `type`.

> Noto: sumoj en eligo `table` ne estas dividitaj per 10^6. Por akiri
> «homan» kvanton de USDT, dividu per 1 000 000.

### Ekzemplo: formato `json`

```json
[
  {
    "tx_id": "abc123...",
    "time": "2026-04-19T08:11:23Z",
    "from": "TFROM...",
    "to": "TXYZ...",
    "value_minimal": "1500000000",
    "token": "USDT",
    "contract": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
    "type": "Transfer"
  }
]
```

Kampoj:

| Kampo           | Tipo   | Priskribo                                                     |
|-----------------|--------|---------------------------------------------------------------|
| `tx_id`         | string | Haŝo de la transakcio.                                        |
| `time`          | string | Bloka tempo en RFC3339 (`time.UnixMilli`).                    |
| `from`          | string | Adreso de la sendanto en Base58.                              |
| `to`            | string | Adreso de la ricevanto.                                       |
| `value_minimal` | string | Sumo en minimalaj unuoj de la ĵetono.                         |
| `token`         | string | Ĵetona simbolo (se malplena el TronGrid — anstataŭiĝas `USDT`). |
| `contract`      | string | Adreso de la ĵetona kontrakto.                                |
| `type`          | string | Eventokategorio el la API de TronGrid.                        |

### Eblaj eraroj

- `invalid Tron address: ...` — nevalida adreso.
- `trongrid trc20 txs: HTTP 4xx: ...` — eraro de API-voko.
- `decode trc20 txs: ...` — nevalida JSON.
- Malplena tabelo (`[]` por JSON, neniuj linioj por table) —
  ne estis transigoj, aŭ la adreso ankoraŭ ne estas aktivigita.

### Notoj

- `watch txs` montras **nur transigojn de la elektita USDT-kontrakto**,
  ne iun ajn TRC‑20. Por la Shasta-testĵetono
  uzatas `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (`Tron.constants.go`), kiu,
  memoru, **ne estas** reala Tether.
- La komando ne paĝigas: unu voko — unu fenestro de maksimume
  200 lastaj konfirmitaj registroj. Por pli profunda historio vi bezonas
  propran ilaron (ekz. rektajn vokojn al TronGrid kun
  `fingerprint` el la respondo).

---

## `watch permissions`

Montras la plursubskriban strukturon de la konto: `owner_permission` (id=0),
nedevigan `witness_permission` (id=1), kaj ĉiujn `active_permission`
(id=2..9). Por ĉiu permeso ĝi printas id, nomon, sojlon, la liston
de permesataj kontraktotipoj (operacia bitmapo malkodita per
`core.Transaction_Contract_ContractType_name`), kaj la liston de
subskribantoj `keys: {address, weight}`.

Se ŝlosila adreso kongruas kun registro en `--keystore-dir`, ĝi estas
markita **`[local: <name>]`** por ĉifritaj monujoj aŭ
**`[external: <name>]`** por registroj de `wallet import-external` (nur
adreso) — por ke vi vidu kiuj ŝlosiloj en la listo kongruas kun viaj
nomoj.

### Uzo

```bash
monujo watch permissions --addr <TRON_BASE58_ADDRESS>
```

### Flagoj

| Flago    | Tipo   | Deviga | Defaŭlto | Priskribo                                                    |
|----------|--------|--------|----------|--------------------------------------------------------------|
| `--addr` | string | jes    | —        | Tron-base58-adreso. Validigata loke antaŭ la HTTP-peto.      |

La ĝenerala `--keystore-dir` uzatas nur por solvi
lokajn monujojn; la ŝlosilujo mem ne estas malĉifrita.

### Ekzemplo: formato `table`

```text
$ monujo watch permissions --addr TXYZ...
permission owner  id=0  threshold=2
  keys:
    TAAA...  weight=1  [local: alice]
    TBBB...  weight=1
permission active0  id=2  threshold=2
  operations: TransferContract, TriggerSmartContract
  keys:
    TAAA...  weight=1  [local: alice]
    TCCC...  weight=1  [local: bob]
```

### Ekzemplo: formato `json`

```json
{
  "address": "TXYZ...",
  "owner": {
    "type": "owner",
    "id": 0,
    "permission_name": "owner",
    "threshold": 2,
    "keys": [
      {"address": "TAAA...", "weight": 1, "local_wallet": "alice"},
      {"address": "TBBB...", "weight": 1}
    ]
  },
  "active": [ ... ]
}
```

En objektoj `keys`, ĉe kongruo kun ŝlosilujo la kampo `local_wallet`
tenas la nomon de la registro; por eksteraj (`import-external`) kaŝnomoj
aldoniĝas `"external_wallet": true`.

### Eblaj eraroj

- `invalid Tron address: ...` — nevalida adreso.
- `trongrid permissions: HTTP 4xx: ...` — eraro de voko al TronGrid.
- Malplenaj `permissions` (ĉiuj kampoj `null`) — la adreso ankoraŭ ne
  estas aktivigita en la reto, aŭ ĝi ne havas proprajn permesojn
  (ekvivalenta al la defaŭlta skemo 1-el-1).

Vidu ankaŭ [13. Komandoj `multisig`](13-multisig.md).
