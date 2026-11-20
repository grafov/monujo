# 12. Reta aktiveco kaj trafika sekureco

Ĉi tiu sekcio listigas **nur tiujn subkomandojn de `monujo`**, kiuj
iniciatas elirantajn retajn petojn, kun protokolo, defaŭltaj domajnoj,
kaj vojoj aŭ gRPC-metodoj. Pri flagoj kaj medivariabloj vidu
[03. Globala agordo](03-configuration.md); pri retoj kaj
USDT-kontraktadresoj — [07. Retoj kaj kontraktoj](07-networks.md).

## Komandoj sen ret-aliro

La jenaj subkomandoj **ne** malfermas konektojn al la Tron-API
kaj ne kontaktas aliajn forajn gastigantojn (nur loka disko kaj
uzant-enigo):

- `wallet list`, `wallet create`, `wallet import`, `wallet check`
- `offline sign`
- `tool genpass`
- `tool vault pack`, `tool vault unpack`

## Tabelo: komandoj kun reta trafiko

Bazaj URL-oj dependas de `--network` (`mainnet` aŭ `shasta`). Ili
povas esti anstataŭigitaj per `--http-url` indikanta kongruan
TronGrid aŭ vian propran plenan nodon.

Defaŭlte **skriboj** (konstruado kaj elsendo de transakcioj) iras
tra **HTTPS REST** (`--write-transport rest`). Nedevige:
`--write-transport grpc` kaj `--grpc-url` (vidu la gRPC-sekcion
sube).

| Komando                         | Protokolo (defaŭlte)      | Domajno (mainnet / shasta)            | Finpunktoj                                                                 |
|---------------------------------|-------------------------|--------------------------------------|---------------------------------------------------------------------------|
| `watch balance`                 | HTTPS (REST, TronGrid)  | `api.trongrid.io` / `api.shasta.trongrid.io` | `GET /v1/accounts/{address}`                                              |
| `watch txs`                     | HTTPS (REST, TronGrid)  | tio sama                     | `GET /v1/accounts/{address}/transactions/trc20`                           |
| `watch permissions`             | HTTPS (REST, TronGrid)  | tio sama                     | `GET /v1/accounts/{address}` (kampoj de permesoj owner/witness/active)      |
| `hot balance`                   | HTTPS (REST, TronGrid)  | tio sama                     | la samaj petoj kiel `watch balance`                                          |
| `hot send-trx`                  | HTTPS (REST)            | tio sama                     | `POST /wallet/createtransaction`, `POST /wallet/broadcasthex`             |
| `hot send-usdt`                 | HTTPS (REST)            | tio sama                     | `POST /wallet/triggersmartcontract`, `POST /wallet/broadcasthex`          |
| `offline build` (`--kind=trx`)  | HTTPS (REST)            | tio sama                     | `POST /wallet/createtransaction` (sen elsendo)                            |
| `offline build` (`--kind=usdt`) | HTTPS (REST)            | tio sama                     | `POST /wallet/triggersmartcontract` (sen elsendo)                         |
| `offline broadcast`             | HTTPS (REST)            | tio sama                     | `POST /wallet/broadcasthex`                                               |
| `multisig info`                 | HTTPS (REST)            | tio sama                     | `POST /wallet/getsignweight`                                              |
| `multisig broadcast`            | HTTPS (REST)            | tio sama                     | `POST /wallet/getsignweight`, `POST /wallet/broadcasthex`                 |

### Plenaj HTTPS-URL-oj

Kun defaŭltaj valoroj la HTTP-bazo estas:

- mainnet: `https://api.trongrid.io`
- shasta: `https://api.shasta.trongrid.io`

Vojoj el la tabelo estas aldonitaj, ekzemple:
`https://api.trongrid.io/v1/accounts/T...`.

### API-ŝlosilo de TronGrid

Se `--api-key` aŭ la medivariablo `MONUJO_TRONGRID_API_KEY` estas
agordita, la kapo `TRON-PRO-API-KEY` estas aldonita al **HTTPS**-petoj.

## Nedeviga gRPC-transporto (`--write-transport grpc`)

Por kongruo kun gRPC de `java-tron` aŭ por depurado vi povas
eksplicite elekti gRPC per gotron-sdk:

| Komando (sama kiel supre) | Protokolo (opt-in) | Defaŭlta gastiganto                               | RPC (Wallet API)                    |
|-------------------------|-------------------|--------------------------------------------|-------------------------------------|
| `hot send-*`, `offline build/broadcast`, `multisig info/broadcast` | gRPC (ofte **plaintext**) | `grpc.trongrid.io:50051` / `grpc.shasta.trongrid.io:50051` | `CreateTransaction2`, `TriggerContract`, `BroadcastTransaction`, `GetTransactionSignWeight` |

Flagoj:

- `--write-transport grpc` — ŝalti gRPC anstataŭ REST.
- `--grpc-url` — `host:port` de plena nodo.
- `--grpc-insecure` — plaintext gRPC (bezonata por publika TronGrid kaj plej multaj lokaj plenaj nodoj).

Kun `--write-transport grpc --grpc-insecure` kontraŭ konataj
TronGrid-gastigantoj la CLI printas **averton al stderr**: publika
gRPC de TronGrid ne uzas TLS; la defaŭlta `--write-transport rest`
estas preferinda.

## Anstataŭigo de nodoj

- `--http-url` — bazo por REST (legoj kaj skriboj kiam `rest`).
- `--grpc-url` — nur kun `--write-transport grpc`.

Ambaŭ parametroj povas indiki **alian domajnon**, se la nodo
realigas la saman HTTP- kaj (kiam bezonate) Wallet-gRPC-interfacon.

## Riskoj kaj streĉa agordo

1. **HTTPS REST defaŭlte.** Legoj kaj skriboj per TronGrid uzas
   **HTTPS** ĉe `api.*.trongrid.io`. gRPC estas nur eksplicita
   opt-in. `http://` por REST estas malpermesita sen
   `--allow-insecure-http`.

2. **Plaintext gRPC.** TronGrid kaj tipa `java-tron` ĉe `:50051`
   akceptas gRPC **sen TLS**. Ne ŝaltu `--write-transport grpc`
   sur nefidinda reto sen kompreni la MITM-riskon. Flagoj
   `--grpc-url` / `--grpc-insecure` sen `--write-transport grpc`
   estas eraro (neniu silenta refalo al REST).

3. **Fido al la nodo.** La nodo vidas adresojn, kvantojn kaj
   subskribitajn transakciojn. Ĝi ne povas ŝanĝi tion, kion subskribas
   `hot send` aŭ `offline build`: monujo komparas la redonitan
   transakcion kun la peto kaj rifuzas interŝanĝitan ricevanton,
   sumon, kontrakton aŭ kotizlimon, kaj kaŝitan aldonitan valoron.
   La nodo ankoraŭ povas rifuzi servon aŭ ekscii la metadatumojn de
   la peto.

4. **Neniu triparta telemetrio.** Eliranta trafiko limiĝas al la
   agorditaj HTTP/gRPC-finpunktoj (vidu la tabelojn supre).
