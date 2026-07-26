# Uzantgvidilo de monujo

[English](../README.md) | [Русский](../ru/README.md)

*Noto: la tuta dokumentaro (kaj tradukoj) estas generita de LLM!*

`monujo` estas minimumisma Go CLI por labori kun la reto **TRON**:
legi saldojn kaj historion de **TRC-20 USDT**-transpagoj por malvarmaj
adresoj (nur publikaj datumoj, sen privataj ŝlosiloj), administri
**lokan ĉifritan ŝlosilujon** (krei/importi; **nomoj ne rajtas esti
validaj Tron-adresoj**, por eviti ambiguon kun `--addr`/`--from`/`--to`;
opcie **eksteraj** nomitaj adresoj), operacioj de **varma monujo**
(reta subskribo kaj sendo de TRX/USDT) kaj **aer-interspaca subskribo**
por malvarma stokado.

Ĉi tiu dokumentaro detale priskribas kiel la binaraĵo estas organizita,
kiujn komandojn, flagojn kaj mediajn variablojn ĝi elmetas, kiel ŝlosiloj
estas stokitaj, kaj kiuj tipaj uzoscenaroj estas subtenataj.

## Enhavo

| Sekcio                                              | Pri                                                                                                                                                      |
|-----------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| [01 — Superrigardo kaj arkitekturo](01-overview.md) | Kion faras la programo, el kiuj komponentoj ĝi konsistas, minacmodelo.                                                                                   |
| [02 — Konstruado kaj instalado](02-installation.md) | Mediaj postuloj, `make build`, instalado de la binaraĵo, versioj.                                                                                        |
| [03 — Ĝenerala agordo](03-configuration.md)         | Ĉiuj ĝeneralaj flagoj (`--network`, `--output`, `--http-url`, `--write-transport`, `--grpc-url`, `--grpc-insecure`, `--allow-insecure-http`, `--api-key`, `--keystore-dir`) kaj mediaj variabloj. |
| [04 — Komandoj `watch`](04-watch-commands.md)       | Detala referenco por `watch balance` kaj `watch txs`.                                                                                                    |
| [05 — Komandoj `hot`](05-hot-commands.md)           | Detala referenco por `hot balance`, `hot send-trx`, `hot send-usdt` (retaj operacioj).                                                                   |
| [06 — Ŝlosilujo](06-keystore.md)                    | Formato v2 (Argon2id) kaj legacy v1 (scrypt) kun aŭtomata migrado, KDF/ĉifrado, **ekstera** nur-adresa JSON, retaj metadatumoj, kongruo kun normoj, rezervkopio. |
| [07 — Retoj kaj kontraktoj](07-networks.md)         | Mainnet kaj Shasta, defaŭltaj finpunktoj, adresoj de USDT-kontraktoj, komisionlimoj.                                                                     |
| [08 — Tipaj scenaroj](08-scenarios.md)              | Paŝ-post-paŝaj receptoj: kontrolo de malvarma adreso, kreo de varma monujo, unua TRX al adreso, sendo de USDT, testoj sur Shasta, skripta uzo.          |
| [09 — Solvado de problemoj](09-troubleshooting.md)  | Klarigo de eraroj kaj manieroj ilin ĉirkaŭiri.                                                                                                           |
| [10 — Komandoj `offline`](10-offline-commands.md)   | Aer-interspaca scenaro: `offline build` (reta), `offline sign` (senreta), `offline broadcast` (reta); koverta formato, HMAC, izolado de ŝlosilujo.      |
| [11 — Komandoj `wallet`](11-wallet-commands.md)     | Vivociklo de ŝlosilujo: `wallet list` (local vs external), `wallet create`, `wallet import`, `wallet import-external`, `wallet check`. Sen reto; funkcias rete kaj en aer-interspaco. |
| [12 — Reta aktiveco kaj sekureco](12-network-security.md) | Kiuj subkomandoj vokas TronGrid HTTPS vs Tron gRPC, defaŭltaj gastigantoj kaj vojoj/RPC-nomoj, API-ŝlosilo, superskriboj kaj notoj pri kanalsekureco. |
| [13 — Komandoj `multisig`](13-multisig.md)          | Partopreno en Tron-multisig-transakcioj: `multisig cosign`, `info`, `broadcast`; dosierformatoj kaj ne-celoj.                                            |

## Rapida referenco

```bash
# Konstruo
make build                                    # binaraĵo ĉe build/monujo

# Nur-lega (sen ŝlosiloj)
./build/monujo watch balance --addr T...
./build/monujo watch txs     --addr T... --limit 50

# Administrado de monujoj (senreta)
./build/monujo wallet list
./build/monujo wallet create --name myhot
./build/monujo wallet import --name imported --key-hex <64_HEX>
./build/monujo wallet import-external --name custodian --addr T...
./build/monujo wallet check --addr myhot   # nur loka ĉifrita monujo (sen reto)

# Retaj operacioj (pasfrazo bezonata nur por subskribo; saldo bezonas neniun)
#
# --addr: hot balance, wallet check, offline sign (nomo en ŝlosilujo aŭ base58;
#   balance permesas ajnan adreson; check/sign bezonas lokan ĉifritan monujon,
#   ne import-external).
# --from / --to: hot send-trx, hot send-usdt, offline build (--from/--to povas
#   uzi monujnomojn inkluzive eksterajn alinomojn kie nur adreso necesas;
#   hot send --from devas esti loka ĉifrita monujo).
./build/monujo hot balance   --addr myhot
./build/monujo hot balance   --addr T...                 # arbitra adreso, ŝlosilujo ne bezonata
./build/monujo hot send-trx  --from funding --to T... --amount 1
./build/monujo hot send-trx  --from myhot --to T... --amount 12.5
./build/monujo hot send-trx  --from myhot --to mycold --amount 12.5   # ricevanto laŭ monujnomo
./build/monujo hot send-usdt --from myhot --to T... --amount 10.25

# Testreto
./build/monujo --network shasta watch balance --addr T...

# Maŝinlegebla eligo
./build/monujo --output json watch balance --addr T...

# Aer-interspaco (senreta subskribo)
./build/monujo offline build      --from T...cold --to T...rcpt --amount 12.5 --kind trx --out tx.unsigned.json
./build/monujo offline sign       --in tx.unsigned.json --addr cold --out tx.signed.json   # sur la senreta PC
./build/monujo offline broadcast  --in tx.signed.json

# Rezervkopio / transloko de la tuta ŝlosilujo kiel unu ĉifrita arkivo (age + tar).
./build/monujo tool vault pack   --out backup.age
./build/monujo tool vault unpack --in backup.age --out /tmp/restore
```

## Gravaj avertoj

- `monujo` **ne estas aparatara monujo**. Ĝia sekureco dependas de:
  fidinda konstruo, fidinda maŝino, permesoj `0600` sur ŝlosilujaj
  dosieroj, forta pasfrazo, kaj (opcie) API-ŝlosilo de TronGrid.
- **Rezervkopio de ŝlosilujo estas deviga.** Kopiu
  `~/.config/monujo/keystore/<name>.json` al apartan mediumon kaj
  stoku la pasfrazon aparte. Por movi la tutan dosierujon kiel unu
  ĉifritan arkivon uzu `monujo tool vault pack/unpack` (age scrypt-mode,
  sen exec en eksteran binaraĵon). Detala proceduro kaj rekomendoj estas
  en
  [06-keystore.md → «Rezerva kopio de keystore»](06-keystore.md#rezerva-kopio-de-keystore).
- **FIPS / kongruo.** monujo ne estas atestita kiel kriptomodulo kaj
  uzas Argon2id (ne en la FIPS-aprobita listo). Por reguligitaj medioj
  vidu
  [06-keystore.md → «Kongruo kun normoj»](06-keystore.md#kongruo-kun-normoj).
- Neniam enmetu ŝlosilujajn dosierojn, pasfrazojn aŭ hex-ŝlosilojn en
  la deponejon. La variablo `MONUJO_IMPORT_KEY` estas subtenata nur por
  skriptaj scenaroj en kontrolita medio.
- `wallet create --export-private-key --allow-private-key-export` unufoje
  printas la privatan ŝlosilon en klarteksto — uzu nur se vi konscie
  akceptas la riskon.
- La USDT-kontrakto sur Shasta (`TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs`)
  estas testa ĵetono, **ne vera Tether**. Vera USDT ekzistas nur sur
  mainnet.
