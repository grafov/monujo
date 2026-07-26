# 03. Ĝenerala agordo

Ĝeneralaj flagoj validas por iu ajn komando kaj devas esti specifitaj
**antaŭ** la nomo de la subkomando (aŭ — danke al `cobra` — inter la
grupo kaj la subkomando). Plej multaj flagoj havas alternativon
per medivariablo; la komanda-linia flago havas prioritaton super la
medio, kaj la medio super enkonstruitaj defaŭltoj.

## Resuma tabelo

| Flago | Medivariablo | Defaŭlto | Celo |
|------|---------------------|---------|---------|
| `--network <id>` | — | `mainnet` | Cela TRON-reto: `mainnet` aŭ `shasta`. |
| `--output <fmt>` | — | `table` | Eliga formato: `table` (homlegebla) aŭ `json`. |
| `--http-url <url>` | — | laŭ reto | Baza HTTPS-URL (lego kaj skribo kiam `rest`). |
| `--write-transport <t>` | — | `rest` | Skriba transporto: `rest` (HTTPS) aŭ `grpc` (opt-in). |
| `--grpc-url <host:port>` | — | laŭ reto | gRPC-adreso; nur kun `--write-transport grpc`. |
| `--grpc-insecure` | — | `false` | Klarteksta gRPC (bezonata por TronGrid kaj plej multaj plenaj nodoj). |
| `--allow-insecure-http` | — | `false` | Permesi `http://` por REST (nesekure; averto al stderr). Sen la flago REST postulas `https://`. |
| `--api-key <key>` | `MONUJO_TRONGRID_API_KEY` | malplena | API-ŝlosilo de TronGrid por pli altaj petlimoj. |
| `--keystore-dir <path>` | `MONUJO_KEYSTORE_DIR` | `~/.config/monujo/keystore` | Dosierujo de ĉifritaj monujaj dosieroj. |
| `--version` | — | — | Montri version kaj git-komiton enmetitajn ĉe konstruado. |
| `--help` / `-h` | — | — | Helpo de `cobra` por komando aŭ subkomando. |

Pliaj medivariabloj sen kongrua flago:

| Variablo | Kie uzata | Celo |
|----------|------------|---------|
| `MONUJO_IMPORT_KEY` | `wallet import` | Alternativo al `--key-hex`: 64 deksesumaj karakteroj de privata ŝlosilo. |
| `MONUJO_TRON_E2E` | Nur en `make test-integration` | Ŝaltas realajn integrajn testojn kontraŭ vivaj Tron-nodoj. Ne influas normalan CLI-uzon. |

## Priskribo de ĉiu flago

### `--network`

Tipo: signoĉeno. Permesataj valoroj:

- `mainnet` (defaŭlte) — la ĉefa TRON-reto.
- `shasta` — la publika testreto Shasta.

Majuskloj/minuskloj ne gravas (`Mainnet`, `SHASTA` ankaŭ validas),
sed en la ŝlosiluja dosiero la reto ĉiam estas stokita minuskle.

Eraro: `unknown network "..." (use mainnet or shasta)`.

Ĉi tiu elekto determinas:

- la HTTP-finpunkton de TronGrid (vidu [07 — Retoj](07-networks.md));
- la gRPC-adreson de la nodo;
- la adreson de la USDT-kontrakto;
- la skribon en ŝlosilujajn metadatenojn `networks` ĉe `wallet create`/`wallet import`
  kaj ĉe sukcesa sendo per `hot send-*`;
- ĉu la interaga averto ekas kiam la retoj de jam uzata monujo
  ne kongruas.

### `--output`

Tipo: signoĉeno. Permesataj valoroj:

- `table` (defaŭlte) — homlegebla teksto.
- `json` — maŝinlegebla JSON taŭga por `jq`/skriptoj.

Majuskloj/minuskloj ne gravas. Eraro: `unknown output format "..." (use table or json)`.

Pli pri formatoj — en ĉiu el la komandaj sekcioj.

### `--http-url`

Tipo: signoĉeno (URL). Defaŭlte anstataŭiĝas unu el la jenaj:

- `https://api.trongrid.io` (mainnet)
- `https://api.shasta.trongrid.io` (shasta)

Uzatas por `watch balance`, `watch txs`, `hot balance`, kaj por
skribaj operacioj kiam `--write-transport rest` (`/wallet/*`). Se
vi pasas malplenan signoĉenon — aplikatas la defaŭlto.

Komencaj kaj finaj spacetoj estas tonditaj; ankaŭ fina `/` estas
tondita. Ne estas aŭtomata prefikso de `https://` — specifu la URL-on
plene.

Kun `--write-transport rest` la skemo devas esti `https://`. Por
loka nodo kun `http://`, pasu `--allow-insecure-http`
(averto al stderr).

### `--write-transport`

Tipo: signoĉeno. Permesataj valoroj:

- `rest` (defaŭlte) — HTTPS Wallet HTTP API de TronGrid (`/wallet/createtransaction`, `/wallet/broadcasthex`, …).
- `grpc` — gotron-sdk al `--grpc-url` (malnova, propra nodo, senerarigado).

Eraro: `unknown write transport "..." (use rest or grpc)`.

### `--grpc-url` kaj `--grpc-insecure`

Havas sencon nur kun `--write-transport grpc`. Se sub la defaŭlta
`rest` vi eksplicite pasas `--grpc-url` aŭ `--grpc-insecure`, la CLI
eliras kun eraro (vi bezonas aldoni `--write-transport grpc`).

Defaŭltoj:

- `grpc.trongrid.io:50051` (mainnet)
- `grpc.shasta.trongrid.io:50051` (shasta)

Publika gRPC de TronGrid rulas **sen TLS**. Kun
`--write-transport grpc --grpc-insecure` sur tiuj gastigantoj la CLI
printas averton al stderr. Por normala uzo de TronGrid, restu ĉe la
defaŭlta `rest`.

### `--api-key`

Tipo: signoĉeno. Pasata:

- en HTTP-petoj al TronGrid en la kapo `TRON-PRO-API-KEY`;
- en la gRPC-kliento per `gc.SetAPIKey(...)` el `gotron-sdk`.

Se la ŝlosilo estas agordita nek per flago nek per `MONUJO_TRONGRID_API_KEY`,
petoj iras sen ŝlosilo — tio funkcias, sed la publika limo de
TronGrid estas tre malalta (pli bone akiri senpagan ŝlosilon ĉe
[trongrid.io](https://www.trongrid.io)).

### `--keystore-dir`

Tipo: signoĉeno (vojo). Algoritmo de solvado:

1. Se `--keystore-dir` estas agordita, uzi ĝin (kiel estas).
2. Alie se `MONUJO_KEYSTORE_DIR` estas agordita, uzi `filepath.Abs`
   de ĝi.
3. Alie uzi `~/.config/monujo/keystore` (`os.UserHomeDir()`).

La dosierujo estas kreata aŭtomate ĉe `wallet create`/`wallet import`
kun reĝimo `0700`. Se la dosierujo mankas, `wallet list` silente
redonas malplenan liston.

La dosierujo tenas monujajn JSON-dosierojn: ĉifritajn lokajn (v1/v2)
kaj nedevige **eksterajn** nur-adresajn registrojn (`wallet import-external`).
Subdosierujojn kaj aliajn dosierojn `wallet list` preterlasas sen eraroj.

### `--version`

Printas la version kiel enmetitan ĉe konstruado per Makefile. Eliga
formato: `<version> (<commit>)`. Sen Makefile-konstruado tio estos
`dev (unknown)` (vidu `cmd/monujo/main.go`).

```bash
monujo --version
# v0.1.0 (a1b2c3d)
```

### `--help` / `-h`

Norma `cobra`. Sur iu ajn nivelo — radiko, grupo, kaj specifa
subkomando. Vidu [02 — Instalado](02-installation.md#kie-trovi-la-propran-helpon-de-la-programo).

## Medivariabloj pli detale

### `MONUJO_TRONGRID_API_KEY`

Kaŝnomo por `--api-key`. Komencaj/finaj spacetoj estas tonditaj.
Uzatas nur kiam la flago ne estas agordita.

### `MONUJO_KEYSTORE_DIR`

Kaŝnomo por `--keystore-dir`. Antaŭ uzo la vojo estas farita
absoluta (`filepath.Abs`).

### `MONUJO_IMPORT_KEY`

Uzatas nur en `wallet import`. Se `--key-hex` ne estas agordita aŭ
estas malplena, la programo prenas la valoron el ĉi tiu variablo
(kun `TrimSpace`). Se ankaŭ la variablo mankas — printatas eraro:

```
provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key
```

Enhavo — 64 deksesumaj karakteroj (nedevige kun prefikso `0x`,
kiu estos forigita). Iuj ajn kromaj karakteroj aŭ malĝusta longo
kaŭzas validigan eraron; tute nula ŝlosilo estas rifuzata kiel
evidente malforta.

## Kio NE estas agordebla

- **Parametroj de scrypt** (`N=32768, r=8, p=1`) — fiksitaj en kodo.
- **Fee limit** por TRC‑20-transakcioj (`DefaultFeeLimitSun =
  100_000_000` sun, t.e. **100 TRX**) — fiksita en kodo, ne
  eksponita kiel ekstera flago.
- **Tempolimoj HTTP/gRPC** — `30s` por HTTP TronGrid kaj `60s` por gRPC,
  sen flagoj.
- **Versio de ĉifrita ŝlosilujo** — skribata kiel `2` (argon2id);
  malnovaj dosieroj `1` (scrypt) ankaŭ estas legataj, kun aŭtomata
  migrado ĉe malŝloso.

## Kongrueco de flagoj inter komandoj

La samaj ĝeneralaj flagoj validas por iu ajn komando. Dume,
lokaj flagoj de specifa subkomando (ekz. `--addr`,
`--from`, `--name`, `--amount`, `--force`) estas priskribitaj nur tie
kaj devas esti specifitaj post la nomo de la subkomando.

Ekzemplo de plena linio:

```bash
monujo --network shasta --output json --api-key "$KEY" \
       hot send-usdt --from myhot --to T... --amount 10.25 --force
```
