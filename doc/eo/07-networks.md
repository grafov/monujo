# 07. Retoj, finpunktoj kaj kontraktoj

## Subtenataj retoj

`monujo` funkcias kun du retoj TRON:

| `--network`              | Celo                    | HTTP TronGrid                    | gRPC                            | USDT-kontrakto                                                      |
|--------------------------|-------------------------|----------------------------------|---------------------------------|---------------------------------------------------------------------|
| `mainnet` (defaŭlte)     | Ĉefa publika reto       | `https://api.trongrid.io`        | `grpc.trongrid.io:50051`        | `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t` (Tether)                       |
| `shasta`                 | Testreto Shasta         | `https://api.shasta.trongrid.io` | `grpc.shasta.trongrid.io:50051` | `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs` (testa USDT-simila ĵetono)     |

Finpunktoj povas esti superskribitaj per tutmondaj flagoj:

- `--http-url` — anstataŭigas la HTTP-bazon de TronGrid;
- `--grpc-url` — anstataŭigas la adreson `host:port` por gRPC.

Tio estas utila se vi volas uzi propran plenan Tron-nodon aŭ
TronGrid-prokurilon sur propra domajno. La programo ne „konas“ aliajn
retojn (Nile, privata ĉeno) — sed per `--http-url`/`--grpc-url` ili
principe povas esti konektitaj se ili estas plene API-kongruaj.

## Adresoj de USDT

La USDT-kontrakto estas elektita de `USDTContractForNetwork` en
`internal/tron/constants.go`:

- **Mainnet** — `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`. Tio estas la
  kanona Tether USDT sur TRON, 6 frakciaj ciferoj. Uzata en
  `watch balance`, `watch txs` (filtrilo laŭ kontrakto), kaj
  `hot send-usdt`.
- **Shasta** — `TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`. Testa
  USDT-simila ĵetono, **ne vera Tether**. Oportuna por erarserĉado,
  sed ne uzu en produktado.

monujo ne montras aliajn kontraktojn: `watch balance` kaj
`watch txs` filtrilas laŭ ĉi tiu sola adreso. Se via monujo tenas
aliajn TRC-20-ĵetonojn, ili ne aperos en la eligo.

## Formato de adreso

monujo akceptas Tron-adresojn en norma **Base58Check** (komenciĝas
per `T`, longeco 34 karakteroj). Ekzemploj:

- adreso mainnet/shasta: `TLcVUPaYnQp1XzYSZ6h4kRxJZ6Ld9gP1aN`
- USDT-kontrakto: `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`

La sama privata ŝlosilo donas la saman adreson en **ambaŭ** retoj
(mainnet kaj Shasta). Tial la ŝlosilujo ne estas „ligita“ al reto —
ĝi nur „memoras“ la reton en metadatenoj `networks` por averti pri
ŝanĝo.

La adreso estas validigita per `gotron-sdk` (`Base58ToAddress`). Ĉe
eraro `invalid Tron address: <reason>` estas printita.

## Kotizoj kaj rimedoj

Sur TRON pago diferencas de Ethereum:

- **Bandwidth** — elspezata ĉe simplaj transigoj (TRX, aktivigo). Ĉiu
  konto ricevas senpagan ĉiutagan kvoton. Se ĝi estas elĉerpita, la
  manko estas pagita per „brulado“ de malgranda sumo de TRX.
- **Energy** — elspezata ĉe kontraktalvokoj, inkluzive de `transfer`
  sur TRC-20 (USDT). Se energy ne sufiĉas, TRX estas bruligita laŭ
  la nodo-kurzo.

`monujo` ne montras kosto-takson antaŭ sendo. Por alvokoj TRC-20
la kodo fikskodas `feeLimit = 100_000_000 sun` (100 TRX). Se la
efektiva kosto de la alvoko superas tion, la nodo redonas eraron
(ekz. `OUT_OF_ENERGY`).

Antaŭ sendo de USDT certigu:

- La sendanta adreso havas **TRX** (almenaŭ 5–10 TRX en rezervo por
  energy/bandwidth).
- La ricevanta adreso ekzistas (se ne — TronGrid tamen akceptas la
  transakcion, la konto estas kreita, sed en raraj kazoj povas esti
  ekstraj kostoj por kreo de konto).

## Komparo de retoj

| Eco                | Mainnet         | Shasta                                          |
|--------------------|-----------------|-------------------------------------------------|
| Vera mono          | Jes             | Ne (testo)                                      |
| De kie akiri TRX   | Borso, P2P      | [Shasta faucet](https://www.trongrid.io/faucet) |
| USDT-kontrakto     | Tether          | Testa ĵetono                                    |
| Stabileco de nodo  | Alta            | Foje degradas, sen SLA                          |
| Uzata por          | Produktaj operacioj | Erarserĉado, lernado, CI-testoj             |

## Superskribo de finpunktoj

### Propra HTTP-nodo

```bash
monujo --http-url https://my-tron-node.example.com \
       watch balance --addr T...
```

La nodo devas subteni almenaŭ la jenon por **legoj** (TronGrid
v1 API):

- `GET /v1/accounts/{addr}` — por `watch balance` / `hot balance`;
- `GET /v1/accounts/{addr}/transactions/trc20` — por `watch txs`.

Por **skriboj** defaŭlte (`--write-transport rest`) sur la sama
`--http-url` la Wallet-HTTP-metodoj estas bezonataj:
`/wallet/createtransaction`, `/wallet/triggersmartcontract`,
`/wallet/broadcasthex`, `/wallet/getsignweight`.

La API-ŝlosilo estas pasigita en la kapo `TRON-PRO-API-KEY`. Se via
nodo ne uzas ŝlosilojn, `--api-key` povas esti preterlasita.

### Propra gRPC-nodo (opt-in)

```bash
monujo --write-transport grpc --grpc-url my-tron-grpc.example.com:50051 \
       --grpc-insecure \
       hot send-trx --from myhot --to T... --amount 1
```

Kutime `--grpc-insecure` (plaintext gRPC) estas bezonata. mTLS ankoraŭ
ne estas subtenata per flagoj.

Kiam ĉeestas, la API-ŝlosilo estas pasigita en gRPC-metadatenoj per
gotron-sdk.

### API-ŝlosilo de TronGrid

Senpaga ŝlosilo de TronGrid konsiderinde altigas la limon de petoj
(precipe por `watch txs`). Registriĝu ĉe
[trongrid.io](https://www.trongrid.io). Post akiro:

```bash
export MONUJO_TRONGRID_API_KEY="<your key>"
monujo watch txs --addr T...
```

aŭ per flago:

```bash
monujo --api-key "<key>" watch balance --addr T...
```

## Kio estas skribita al metadatenoj de monujo

Ĉe `wallet create`, `wallet import`, kaj sukcesa `hot send-trx` aŭ
`hot send-usdt`, la nuna `--network` estas konservita en
`EncryptedWalletFile.networks`. Tio estas por ke ĉe posta rulo kun
malsama `--network` la uzanto vidu averton kaj ne akcidente agadu
sur la malĝusta reto (ekzemple, rulu testajn operaciojn sur mainnet,
aŭ sendu veran monon al Shasta).

Vidu [06 — Ŝlosilujo](06-keystore.md#metadatenoj-networks) por detaloj.
