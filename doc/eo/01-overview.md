# 01. Superrigardo kaj arkitekturo

## Kio estas monujo

`monujo` estas Go-komanda linia ilo fokusita al la blokĉeno **TRON**
kaj la ĵetono **USDT (TRC‑20)**. Ĝi solvas kvar sendependajn taskojn:

1. **Nur-lega (watch‑only)** — akiri publikan informon (saldo TRX/USDT,
   historio de TRC‑20-transigoj) por arbitra adreso. Neniuj ŝlosiloj
   bezonatas; la adreso povas aparteni al iu ajn (inkluzive malvarman
   stokon). Uzatas nur la publika HTTP-API de TronGrid.
2. **Monujo (administrado de ŝlosilujo)** — krei novajn ŝlosilojn,
   importi ekzistantajn, kaj listigi lokajn monujojn. Tiuj komandoj
   laboras nur kun dosieroj sur disko kaj **ne tuŝas la reton**, do
   ili egale validas sur reta komputilo kaj sur aer-interspaca
   maŝino.
3. **Varma monujo (hot)** — retaj operacioj: `hot balance` (sen
   pasvorto; monuja nomo aŭ arbitra adreso per `--addr`),
   `hot send-trx`, `hot send-usdt`. Subskribado okazas en-proceze;
   elsendo iras tra TronGrid **HTTPS REST** (gRPC estas opt-in).
4. **Aer-interspaco (offline)** — tri komandoj por la klasika scenaro
   de fizike izolita ŝlosilo: `offline build` preparas nesubskribitan
   transakcion sur reta komputilo, `offline sign` sur senreta
   komputilo aldonas subskribon el la loka ŝlosilujo, `offline broadcast`
   denove sur la reta komputilo sendas la rezulton al la reto.
   Subtenatas TRX kaj USDT. Protekto de integreco dum movo de la
   dosiero — deviga vida kontrolo de analizitaj kampoj plus nedeviga
   HMAC-SHA256 super komuna pasfrazo.

Solana kaj BEP ne estas subtenataj en ĉi tiu versio.

## Subtenataj operacioj

- Legi saldojn TRX kaj USDT por iu ajn adreso.
- Legi historion de USDT (TRC‑20) transigoj — la lastajn N transakciojn.
- Krei novan monujon (generi hazardan secp256k1-ŝlosilon).
- Importi ekzistantan ŝlosilon (64 deksesumaj karakteroj).
- Listigi lokajn monujojn kun metadatenoj.
- Aktivigi novan adreson (sendi TRX por ke la konto aperu en la
  enĉena registro).
- Sendi denaskan TRX.
- Sendi USDT (TRC‑20) — mainnet uzas la kanonan kontrakton de Tether;
  Shasta uzas testĵetonon kun simila semantiko.
- Aer-interspaca sendo de TRX kaj USDT: prepari nesubskribitan
  transakcion rete (`offline build`), subskribi sur izolita maŝino
  (`offline sign`), elsendi denove rete
  (`offline broadcast`). Vidu [10 — komandoj `offline`](10-offline-commands.md).

## Arkitekturo

La kodbazo estas dividita en plurajn internajn pakaĵojn:

```
cmd/monujo/main.go            # entry point; injects version from ldflags
internal/cli/                 # command and flag parsing (cobra)
  root.go                     # global flags and Execute()
  watch.go                    # watch subcommands
  wallet.go                   # wallet subcommands (keystore lifecycle, no network)
  hot.go                      # hot subcommands (online operations)
  offline.go                  # offline subcommands (build/sign/broadcast)
  passphrase_input.go         # readPassTwice + --generate-passphrase for wallet
  network_confirm.go          # interactive confirmation for --network and --overwrite
internal/config/config.go     # Network, OutputFormat types and Config
internal/app/                 # ties CLI to the domain
  watch_service.go            # formats watch output (table/json)
  hot_service.go              # runs hot operations
  offline_service.go          # ties keystore + envelope + write_client
internal/keystore/            # encryption/decryption, metadata
  keystore.go                 # file format v2 (argon2id + AES‑GCM),
                              # legacy v1 (scrypt), auto-migration v1→v2
  passphrase.go               # password input without echo
internal/domain/              # immutable data types, amount parsing
  asset.go                    # ParseTRXDecimal, ParseUSDTDecimal, formatters
  transfer.go                 # TRC20TransferRow
internal/tron/                # network clients
  read_client.go              # HTTP to TronGrid (account, trc20 history)
  http_write_client.go        # HTTPS Wallet API (write by default)
  write_client.go             # opt-in gRPC (gotron-sdk) + signing
  address.go                  # ValidateBase58Address
  constants.go                # USDT contract addresses
  offline.go                  # build/sign/broadcast helpers outside keystore
internal/transfer/            # JSON envelopes for the air-gap scenario
  envelope.go                 # UnsignedTransferPayload / SignedTransferPayload types
  envelope_seal.go            # outer envelope + scrypt-HMAC for integrity
```

### Ekzekuta fluo

#### Komando `watch ...`

1. `cmd/monujo/main.go` vokas `cli.Execute()`.
2. `cli.newRootCmd()` registras ĝeneralajn flagojn kaj subkomandojn,
   analizas argumentojn.
3. `cli.loadConfig()` konstruas `config.Config` el flagoj kaj
   medivariabloj.
4. `app.NewWatchService(cfg)` kreas `tron.ReadClient`.
5. `WatchService.PrintBalance()` aŭ `PrintTransfers()` faras
   HTTP-petojn al TronGrid kaj printas la rezulton en la elektita
   formato (`table` aŭ `json`).

#### Komando `wallet ...`

1. Same `Execute()` → `loadConfig()`.
2. `wallet list` trairas la ŝlosilujan dosierujon per `keystore.ListWallets()`
   kaj printas nomojn/adresojn/registritajn retojn — sen retaj vokoj.
3. `wallet create` / `wallet import` petas pasfrazon (aŭ
   generas unu per `--generate-passphrase`), vokas
   `keystore.GenerateAndStore()` / `keystore.Store()`, kaj skribas la
   nunan `--network` en la dosierajn metadatenojn. Antaŭ skribo, se
   dosiero jam ekzistas, aplikatas `confirmWalletOverwrite()`.

#### Komando `hot ...`

1. Same `Execute()` → `loadConfig()`.
2. Por ĉiu komando `hot`,
   unue rulas `confirmWalletNetworkForOperation()`: metadatenoj
   `networks` de la monuja dosiero estas legataj kaj, ĉe malkongruo
   kun `--network`, konfirmo estas petata en interaga reĝimo (aŭ
   eraro se ne estas TTY kaj `--force` ne estis pasita).
3. `hot balance` solvas `--addr` (nomo aŭ base58) per
   `keystore.ListWallets()` kaj **ne petas pasvorton**; poste
   vokas `WatchService.PrintBalance()`.
4. `hot send-trx` / `hot send-usdt` petas
   pasfrazon (sen eĥo, per `golang.org/x/term`), poste:
   - `keystore.LoadPrivateKey()` malĉifras la ŝlosilujon
     (argon2id → AES‑GCM por v2, aŭ scrypt → AES‑GCM por malnova v1
     kun aŭtomata migrado al v2), kontrolas la adreson.
   - Transakcio estas konstruita per HTTPS REST TronGrid (defaŭlte)
     aŭ opt-in gRPC, subskribita loke, kaj elsenta sur la sama
     skriba transporto.
   - Ĉe sukcesa sendo,
     `keystore.EnsureNetworkRecorded()` estas vokata por ke la nuna
     reto aperu en la dosieraj metadatenoj se ĝi mankis.
5. La rezulto estas printata (`table` aŭ `json`).

### Kie loĝas la datumoj

- **Agordo**: nur en la medio kaj argumentoj. La
  programo neniam kreas yaml/json-agorddosierojn sur disko.
- **Ĉifritaj ŝlosiloj**: dosierujo `~/.config/monujo/keystore/`
  defaŭlte, anstataŭigebla per `--keystore-dir` aŭ
  `MONUJO_KEYSTORE_DIR`. Unu ŝlosilo — unu dosiero de la formo
  `<name>.json` kun reĝimo `0600`.
- **Protokoloj**: neniuj. La programo printas rezultojn al `stdout`,
  diagnozojn kaj avertojn al `stderr`. Nenio estas skribata al
  dosieroj krom la ŝlosilujo.

## Minaca modelo

`monujo` estas **programa** monujo, ne aparata. Ĝi protektas
la ŝlosilon kontraŭ likoj el la dosiersistemo (ĉifrado AES‑256‑GCM
kun ŝlosilo derivita per Argon2id el la pasfrazo; malnovaj
scrypt-v1-dosieroj estas legataj kaj aŭtomate migras al v2 ĉe la
unua malŝloso), sed **ne rezistas** kompromiton de la maŝino mem:
la procezo tenas la malĉifritan ŝlosilon en memoro dum la sendo;
la ekrano kaj tondujo estas regataj de la OS.

`monujo` **ne estas** atestita kriptografia modulo
(FIPS 140-3 / ISO/IEC 19790) kaj uzas Argon2id, kiu ne estas
FIPS-aprobita KDF. Por medioj kiuj postulas FIPS-kongruon
la ilo ne taŭgas. La pasfrazo estas normaligita al Unikodo NFKC
antaŭ la KDF (NIST SP 800-63B §5.1.1.2 / OWASP Password Storage
Cheat Sheet) por ke la sama pasvorto donu la saman ŝlosilon sur
macOS, Linux kaj Windows. Detaloj — en
[06-keystore.md → "Kongruo kun normoj"](06-keystore.md#kongruo-kun-normoj).

Supozoj sub kiuj la sistemo restas sekura:

- La binaraĵo estis konstruita el fidindaj fontoj (vi aŭ via CI).
- La maŝino ne estas kompromitita (neniuj klavregistriloj, fremda
  root-aliro, swap legebla de aliaj).
- Ŝlosilujaj dosieroj havas reĝimon `0600`; la hejma dosierujo
  estas protektita.
- La pasfrazo estas sufiĉe forta kaj ne estas konservita en la
  historio de la ŝelo.
- La API-ŝlosilo de TronGrid (se uzata) ne likas en publikajn
  deponejojn.

La programo aktive rezistas tipajn erarojn:

- Nuligas pasfrazojn kaj bufrojn de privataj ŝlosiloj tuj post
  uzo.
- Ne uzas `float64` por sumoj (TRX kaj USDT estas traktataj kiel
  `int64` aŭ `*big.Int`) por eviti rondoerarojn.
- Ŝlosilujaj dosieroj estas legataj tra `os.Root` por malebligi
  simbolajn ligojn ekster la ŝlosilujan dosierujon.
- Dosiernomoj estas sanitigitaj: nur `[a-zA-Z0-9_-]` estas permesataj;
  aliaj karakteroj estas anstataŭigitaj per `_`. Malplena nomo
  fariĝas `default`.
- Atomaj skriboj de metadatenoj (per portempa dosiero + `rename`)
  kiam reto estas aldonata al la ŝlosilujo.

## Kion monujo NE faras

- Ne administras aparatajn subskribilojn.
- Ne laboras kun TRC‑10, NFT-oj, aŭ kontraktoj krom la
  eksplicite enkonstruita USDT (por TRC‑20 nur `transfer(address,uint256)`
  sur la kanona USDT-kontrakto de la nuna reto estas subtenata,
  inkluzive en senreta reĝimo).
- Ne faras plursubskribon.
- Ne montras saldojn de aliaj TRC‑20-ĵetonoj.
- Ne faras interŝanĝon/konverton.
- Ne sendas sciigojn, ne observas la reton en reala tempo, kaj ne
  rulas demonon.

Ĉi tio estas intence minimuma CLI de «unufoja ago»: voku —
ricevu rezulton — eliru.
