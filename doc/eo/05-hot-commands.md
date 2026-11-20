# 05. Komandoj `hot`

La grupo `hot` kovras **retajn operaciojn** sur ekzistanta monujo el
la loka ŝlosilujo: kontrolo de saldo, sendo de TRX kaj USDT. Ĉiuj
komandoj en ĉi tiu grupo faras retajn alvokojn (HTTPS TronGrid: legoj
per `/v1/*`, skriboj per `/wallet/*` defaŭlte).

> La vivociklo de ŝlosilujaj dosieroj (krei, importi, listigi) loĝas en
> la aparta grupo [`monujo wallet`](11-wallet-commands.md) kaj ne faras
> retajn alvokojn; la samajn dosierojn poste uzas kaj `hot` kaj
> `offline sign`.

```text
monujo hot
├── balance       — saldo TRX kaj USDT laŭ monuja nomo aŭ adreso (sen pasvorto)
├── send-trx      — sendi TRX
└── send-usdt     — sendi USDT (TRC-20)
```

Tutmondaj flagoj (`--network`, `--output`, `--api-key`,
`--keystore-dir`, `--http-url`, `--write-transport`, `--grpc-url`,
`--grpc-insecure`, `--allow-insecure-http`) aplikiĝas al ĉiuj
subkomandoj. Vidu [03 — Agordo](03-configuration.md) por detaloj.

## Komunaj principoj

- **`--addr` kaj `--from` / `--to`**. Por `hot balance` la objekto
  estas agordita per **`--addr`**: nomo de ŝlosiluja dosiero (sen
  `.json`) aŭ Tron-base58-adreso. Por `hot send-trx` kaj `hot
  send-usdt` la sendinto estas **`--from`**, la ricevanto estas
  **`--to`**. La valoro estas aŭ ŝlosiluja nomo aŭ base58-adreso
  (T..., 34 karakteroj). Rekono estas aŭtomata: ĉio, kio trapasas
  base58-validigon, estas traktata kiel adreso, alie kiel nomo;
  solvo iras tra `keystore.ListWallets` sen deĉifrado.
    - **`--addr` en `hot balance`** akceptas arbitran adreson: la
      komando legas saldojn el la reto kaj ŝlosilujo ne estas
      bezonata.
    - **`--from` en `hot send-trx`/`hot send-usdt`** akceptas adreson
      nur se ĝi kongruas kun **loka** (ne ekstera) monujo en la
      ŝlosilujo. Nomo de ekstera ero (`import-external`) donas
      eraron: sendo postulas privatan ŝlosilon.
    - **`--to`** akceptas ajnan validan base58-adreson aŭ nomon de
      ŝlosiluja ero (loka aŭ **ekstera** — nur adreso).
- **Enigo de pasfrazo**. Komandoj, kiuj bezonas subskribon
  (`send-trx`, `send-usdt`), petas pasfrazon per TTY sen eĥo. `hot
  balance` **ne** petas pasvorton: ĝi legas la adreson el la malfermaj
  metadatenoj de la dosiero.
- **Viŝado de sekretoj**. La pasfrazo kaj materialo de la privata
  ŝlosilo estas nuligitaj (`ZeroPassphrase`, `Wipe`) tuj post uzo.
- **Konfirmo de reto**. Se la monuja dosiero jam havas ne-malplenan
  liston `networks` kaj la nuna `--network` ne estas en ĝi, averto
  estas printita kaj konfirmo `[y/N]` estas petata antaŭ la
  operacio. Por preterpasi ĉi tiun konfirmon en skriptoj — `--force`.
  Sen TTY kaj sen `--force`, la eraro `network mismatch: use
  --force to proceed without a terminal, or run interactively to
  confirm` estas redonita.
- **Skribo de metadatenoj**. Post sukcesa sendo de TRX aŭ USDT, la
  programo aldonas la nunan `--network` al la tabelo `networks` de
  la monuja dosiero (sen reĉifrado; nur metadatenoj ŝanĝiĝas, per
  atoma anstataŭigo kun `rename`).

---

## `hot balance`

Montras saldon TRX kaj USDT por ŝlosiluja **nomo** aŭ **kruda**
base58-adreso. Por nomo, la adreso estas legita el dosieraj
metadatenoj (inkluzive de eksteraj eroj `import-external`); **neniu
pasfrazo estas petata**.

### Uzo

```bash
monujo hot balance --addr <NAME>
monujo --network shasta hot balance --addr testhot --force
```

### Flagoj

| Flago     | Tipo   | Bezonata | Defaŭlto | Priskribo                                                                                        |
|-----------|--------|----------|----------|--------------------------------------------------------------------------------------------------|
| `--addr`  | string | jes      | —        | Nomo de ŝlosiluja monujo AŬ Tron-base58-adreso (arbitra).                                        |
| `--force` | bool   | ne       | `false`  | Preterpasi interagan konfirmon se la nuna `--network` ne estas registrita en la dosiera `networks`. |

### Kio okazas

1. La valoro `--addr` estas solvita per `keystore.ListWallets` (nomo
   aŭ base58-adreso).
2. **Se** ĝi estas nomo aŭ adreso trovita en la ŝlosilujo:
   - ĉe malkongruo inter la nuna `--network` kaj la tabelo `networks`
     de la dosiero, averto estas printita kaj (sen `--force`)
     konfirmo estas petata — same kiel por lokaj monujoj, **inkluzive
     de eksteraj** eroj kun ne-malplena `networks`;
   - poste saldoj estas petataj por la stokita adreso (por ekstera
     ero la sama nur-lega vojo estas uzata kiel por kruda adreso).
3. **Se** ĝi estas arbitra base58 ne en la ŝlosilujo — la paŝo de
   kontrolo de `networks` estas **preterpasita**, kaj HTTP-peto de
   saldo estas farita tuj.

> Deĉifrado de ŝlosilo por `hot balance` **ne** estas farita nek por
> lokaj nek por eksteraj eroj.

### Ekzemplo

```text
$ monujo hot balance --addr myhot
Address: TLcVUPa...
TRX:     50.123456 (50123456 sun)
USDT:    100 (minimal units 100000000)
```

La JSON-strukturo estas la sama kiel por `watch balance` (vidu
[04](04-watch-commands.md#example-json-format)).

---

## `hot send-trx`

Sendas denaskan TRX de loka monujo al iu ajn adreso. La unua enira
TRX al nova adreso en Tron igas la konton videbla al la lega API —
ordinara `send-trx` kun malgranda sumo al la cela adreso (aŭ faucet
en Shasta) sufiĉas por tio.

### Uzo

```bash
monujo hot send-trx --from <NAME> --to <ADDR> --amount <TRX>
```

### Flagoj

| Flago      | Tipo   | Bezonata | Defaŭlto | Priskribo                                                                                       |
|------------|--------|----------|----------|-------------------------------------------------------------------------------------------------|
| `--from`   | string | jes      | —        | Nomo de fonta monujo AŬ ĝia base58-adreso (devas kongrui kun loka ŝlosiluja dosiero).           |
| `--to`     | string | jes      | —        | Ricevanto: base58-adreso AŬ nomo de loka monujo.                                                |
| `--amount` | string | jes      | —        | Sumo TRX (dekuma nombro, ĝis 6 frakciaj ciferoj).                                               |
| `--force`  | bool   | ne       | `false`  | Preterpasi interagan konfirmon pri reto.                                                        |

### Kio okazas

1. Konfirmo de reto (vidu komunajn principojn supre).
2. Peto de pasvorto.
3. Analizo de sumo: `domain.ParseTRXDecimal`. Ĝis 6 ciferoj post la
   dekuma punkto estas permesitaj. Negativaj nombroj, malplena ĉeno,
   ekstraj karakteroj, kaj `int64`-trofluo kaŭzas eraron.
4. Validigo de ricevanta adreso.
5. Deĉifrado de ŝlosilujo.
6. Konstruo de transakcio per HTTPS REST (`/wallet/createtransaction`
   defaŭlte; opt-in gRPC — `gotron-sdk` `Transfer`). monujo malkodas
   la transakcion, kiun la nodo redonis, kaj rifuzas subskribi ĝin, se
   la posedanto, ricevanto aŭ sumo ne kongruas kun la peto, aŭ se la
   transakcio portas memorandon aŭ kroman kontrakton.
7. ECDSA-subskribo, elsendo (`/wallet/broadcasthex` aŭ gRPC `Broadcast`).
8. Se elsendo redonas ne-nulan kodon — eraro `broadcast: <message>`.
9. Aldono de `--network` al metadatenoj.

### Ekzemplo: formato `table`

```text
$ monujo hot send-trx --from myhot --to TXYZ... --amount 12.5
Passphrase: ********
Broadcast OK
Network: mainnet
TxID:    abc123...
From:    TLcVUPa...
To:      TXYZ...
TRX: 12.5
```

### Ekzemplo: formato `json`

```json
{
  "tx_id": "abc123...",
  "from": "TLcVUPa...",
  "to": "TXYZ...",
  "kind": "TRX",
  "amount": "12.5",
  "network": "mainnet"
}
```

### Notoj

- La transakcia ID estas kalkulita loke kiel
  `sha256(proto.Marshal(tx.GetRawData()))` — Tron uzas ĝuste ĉi tiun
  haketon kiel `txID`. Tial `tx_id` estas disponebla tuj post
  sukcesa elsendo, eĉ antaŭ inkludo en bloko.
- Tron pagas aŭ el **bandwidth** (senpage ene de la ĉiutaga limo) aŭ
  per „brulado“ de TRX se bandwidth estas elĉerpita. monujo ne petas
  eksplicitan takson de kotizo; por denaskaj transigoj tio kutime
  estas sekura. Se la adreso tute ne havas TRX, elsendo malsukcesos.
- Ĉe sukceso la nuna `--network` estas aldonita al la dosieraj
  metadatenoj (se ĝi ankoraŭ ne estas registrita). Se la dosiero ne
  povis esti ĝisdatigita pro iu kialo, la programo tamen raportas
  sukcesan elsendon kaj printas eraron pri metadatenoj:

  ```
  broadcast ok (tx <id>) but could not update wallet networks metadata: <reason>
  ```

---

## `hot send-usdt`

Sendas USDT (TRC-20) de loka monujo al iu ajn adreso. Uzadas la
kontrakton elektitan de `--network`:

- mainnet: `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` (Tether),
- Shasta: `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (testa ĵetono,
  **ne vera** USDT).

### Uzo

```bash
monujo hot send-usdt --from <NAME> --to <ADDR> --amount <USDT>
```

### Flagoj

| Flago      | Tipo   | Bezonata | Defaŭlto | Priskribo                                                                 |
|------------|--------|----------|----------|---------------------------------------------------------------------------|
| `--from`   | string | jes      | —        | Nomo de fonta monujo AŬ ĝia base58-adreso (devas kongrui kun loka ŝlosiluja dosiero). |
| `--to`     | string | jes      | —        | Ricevanto: base58-adreso AŬ nomo de loka monujo.                         |
| `--amount` | string | jes      | —        | Sumo USDT (dekuma nombro, ĝis 6 frakciaj ciferoj). Devas esti pozitiva.  |
| `--force`  | bool   | ne       | `false`  | Preterpasi interagan konfirmon pri reto.                                  |

### Kio okazas

1. Konfirmo de reto.
2. Peto de pasvorto.
3. Analizo de sumo: `domain.ParseUSDTDecimal` → `*big.Int` de
   minimumaj unuoj. Ĝis 6 ciferoj post la dekuma punkto. La sumo
   devas esti strikte pozitiva (`amount must be positive` alie).
4. Validigo de ricevanta adreso.
5. Deĉifrado de ŝlosilujo.
6. REST: `POST /wallet/triggersmartcontract` (USDT `transfer(address,uint256)`,
   `fee_limit=100_000_000` sun) aŭ opt-in gRPC `TRC20Send`.
7. monujo malkodas la respondon de la nodo kaj rifuzas subskribi, se
   tio ne estas simpla translokigo de la kanona USDT-kontrakto por
   `--network`: sama posedanto, ricevanto kaj sumo, `fee_limit` egala
   al la agordita limo, kaj `call_value`, `call_token_value`,
   `token_id` kaj la memorando de la transakcio malplenaj.
8. ECDSA-subskribo, elsendo.
9. Aldono de `--network` al metadatenoj.

### Ekzemplo: formato `table`

```text
$ monujo hot send-usdt --from myhot --to TXYZ... --amount 10.25
Passphrase: ********
Broadcast OK
Network: mainnet
TxID:    def456...
From:    TLcVUPa...
To:      TXYZ...
USDT: 10.25 USDT
```

### Ekzemplo: formato `json`

```json
{
  "tx_id": "def456...",
  "from": "TLcVUPa...",
  "to": "TXYZ...",
  "kind": "USDT",
  "amount": "10.25 USDT",
  "network": "mainnet"
}
```

### Notoj

- TRC-20-transigoj konsumas **energy**. Se la adreso mankas sufiĉan
  stakitan `energy`, la manko estas kovrita per brulado de TRX, sed
  ne pli ol `feeLimit = 100 TRX`. Se via saldo TRX estas sub la
  bezonata, elsendo malsukcesas kun `OUT_OF_ENERGY` aŭ `OUT_OF_TIME`
  de la nodo.
- La limo de kotizo (`100 TRX`) estas fikskodita
  (`config.defaultFeeLimitSun`). En la nuna versio ĝi ne povas esti
  ŝanĝita per flago.
- En Shasta la testa USDT-kontrakto povas konduti malsame ol vera
  Tether (decimaloj, eventoj, metodoj — la programo supozas la
  normon `transfer(address,uint256)` kaj 6 frakciajn ciferojn). Antaŭ
  uzo en Shasta, certigu, ke ĉi tio estas la kontrakto, por kiu tiu
  alvoko validas.

---

## Resumo de ĉiuj flagoj `hot`

| Subkomando  | Lokaj flagoj                                        | Petas pasvorton |
|-------------|-----------------------------------------------------|-----------------|
| `balance`   | `--addr`, `--force`                                 | ne              |
| `send-trx`  | `--from`, `--to`, `--amount`, `--force`             | jes             |
| `send-usdt` | `--from`, `--to`, `--amount`, `--force`             | jes             |
