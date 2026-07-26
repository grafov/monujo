# 09. Problemsolvado

Sube estas la plej verŝajnaj eraroj kaj avertoj, kiujn vi povas vidi. Ĉiuj
mesaĝoj estas en la angla (kiel en la fontoj); apud ĉiu estas
klarigo kaj solvo.

## Agordaj eraroj

### `unknown network "..." (use mainnet or shasta)`

Valoro de `--network` alia ol `mainnet`/`shasta` estis pasigita (kaj ne
malplena signoĉeno). Kontrolu usklecon kaj tajperarojn.

### `unknown output format "..." (use table or json)`

Nevalida valoro de `--output`. Nur `table` kaj `json` estas permesataj.

### `provide --key-hex or set MONUJO_IMPORT_KEY with the hex private key`

`wallet import` estis rulita sen ŝlosilo. Pasigu `--key-hex` aŭ eksportu la
variablon `MONUJO_IMPORT_KEY`.

### `private key must be 64 hex characters`

Enigo tro mallonga aŭ tro longa. Prefikso `0x` estas permesata;
karakteroj devas esti strikte `0-9a-fA-F`. Komencaj/finaj spacetoj estas
forigitaj.

### `decode private key hex: ...`

La signoĉeno enhavas ne-deksesumajn karakterojn. Duoble kontrolu, ke vi
kopiis la deksesuman reprezenton.

### `invalid private key length`

Post malkodado vi ne ricevis 32 bajtojn. Tio povas okazi se la
deksesuma signoĉeno havas «ekstran» karakteron, kiu ne rompis la
longkontrolon (ekzemple nedisigita prefikso `0x` lasis neparan longon).

### `refusing weak private key`

Tute nula ŝlosilo estas malpermesata. Ĝi ne povas esti importita — tio
estas eksplicita protekto kontraŭ eraro/injekto.

## Pasfrazaj eraroj

### `passphrases do not match`

Por `wallet create`/`wallet import` la du enigitaj pasfrazoj malsamas.
Rulu la komandon denove.

### `decryption failed (wrong passphrase or corrupt file)`

Kiam oni provas uzi ekzistantan ŝlosilujon (`hot send-*`, `offline sign`,
`wallet check`; `hot balance` ne petas pasvorton kaj neniam
montras ĉi tiun eraron):

- Plej verŝajne la pasvorto estis enigita malĝuste. Uskleco gravas.
- La dosiero povas esti difektita (kontrolu `cat ~/.config/monujo/keystore/<name>.json`
  — ĝi estu valida JSON v1 aŭ v2).
- Se la pasvorto enhavas **ne-ASCII**-karakterojn (cirilajn, umlaŭtojn,
  ligaturojn, plenlarĝajn, emoĝiojn) kaj la monujo estis kreita per
  antaŭa konstruaĵo de monujo sen NFKC-normaligo.
  ASCII-pasvortoj ne havas ĉi tiun problemon.

### `address mismatch in keystore`

Tre rara kazo: deĉifrado sukcesis, sed la privata ŝlosilo produktis
adreson malsaman ol tiu stokita en `address`. Tio indikas
dosieran difekton (aŭ manan redaktadon). Ne uzu tian ŝlosilujon —
rekreu ĝin.

### `invalid decrypted key length`

La deĉifrita bufro ne estas 32 bajtoj. La dosiero estas difektita.

## Kvantaj eraroj

### `empty amount`

`--amount ""` estis pasigita. Specifu nombron.

### `negative amount not allowed`

Kvanto ne povas komenciĝi per `-`. La programo rifuzas negativajn nombrojn
ĉe analiziĝo.

### `invalid character in amount` / `invalid character in fraction`

Nur ciferoj kaj unu punkto estas permesataj. `1,5` (komo) aŭ `1e3`
(scienca notacio) ne estas subtenataj.

### `multiple decimal points`

Pli ol unu punkto en la nombro.

### `too many fractional digits (max 6)`

Kaj TRX kaj USDT havas 6 frakciajn ciferojn. Ekzemple `1.1234567`
estas rifuzata; uzu `1.123456`.

### `trx amount overflows int64`

TRX en sun ne rajtas superi `int64`. Tio estas astronomie granda
kvanto (pli ol ĉiuj ekzistantaj TRX), sed la kontrolo ekzistas.

### `amount must be positive` (nur USDT)

Por `hot send-usdt`, nulaj kaj negativaj kvantoj estas malpermesataj per
aparta kontrolo post analiziĝo.

## Adresaj eraroj

### `invalid Tron address: ...`

La adreso malsukcesas validigon Base58Check. Oftaj kaŭzoj:

- ekstra spaceto aŭ `\n`;
- adreso de alia reto estis kopiita (ekz. Ethereum `0x...`);
- la signoĉeno estis stumpigita (adreso estas ĝuste 34 karakteroj).

## Retaj eraroj

### `trongrid accounts: HTTP 4xx: ...` / `trongrid trc20 txs: HTTP 4xx: ...`

La nodo TronGrid redonis eraron:

- `401`/`403` — kontrolu la API-ŝlosilon.
- `429` — limo de petoj superita. Agordu API-ŝlosilon aŭ atendu.
- `5xx` — portempa problemo de la nodo. Reprovu la peton.

### `trongrid: <error>`

La nodo redonis HTTP 200, sed la JSON havas `success=false` kaj
nemalplenan kampon `error`. La mesaĝo estas citita laŭvorte.

### `decode accounts: ...` / `decode trc20 txs: ...`

JSON ne analiziĝas. La nodo eble respondas en formato ne kongrua kun
TronGrid (ekzemple vi anstataŭigis `--http-url` kaj trafis
malĝustan finpunkton).

### `broadcast: <message>`

La skriba transporto (HTTPS REST aŭ gRPC) akceptis la peton, sed la
nodo redonis neneulan kodon. Tipaj mesaĝoj:

- `OUT_OF_ENERGY` / `OUT_OF_TIME` — por USDT-transigo ne estis
  sufiĉe da energio/bendlarĝo, kaj la TRX-kotizo superis `feeLimit=100 TRX`.
  Plenigu la adreson per TRX (aŭ staku energion).
- `BANDWITH_ERROR` — nesufiĉa bendlarĝo.
- `CONTRACT_VALIDATE_ERROR` — kutime: la ricevanto estas nevalida, la
  sendanto ne estas aktivigita, aŭ la USDT-kontrakto ne troviĝis sur la
  elektita reto.
- `DUP_TRANSACTION_ERROR` — duobla transakcio kun la sama
  `ref_block_hash`. Atendu 1–2 blokojn.

### `empty transaction`

(Specifa por la nedeviga gRPC-transporto) `gotron-sdk` redonis nulan
transakcion — kutime tio signifas, ke la gRPC-nodo estas neatingebla aŭ
redonis eraron, kiun la SDK envolvis kiel `nil`. Kontrolu `--grpc-url` /
haveblecon de la nodo.

### `connection refused`, `context deadline exceeded` / `timeout`

Retaj problemoj. La tempolimo de la skriba HTTP/gRPC-kliento estas 60 sekundoj;
por `ReadClient` ĝi estas 30 sekundoj (`HTTP`). Ĉi tiuj limoj ne povas esti
malstreĉitaj en la nuna versio.

## Eraroj de reta konfirmo

### `network mismatch: use --force to proceed without a terminal, or run interactively to confirm`

En la monuja dosiero `networks` estas nemalplena kaj ne enhavas la
nunan `--network`, kaj STDIN ne estas TTY (ekz. en CI). Solvoj:

- Rulu interage kaj respondu `y`;
- Pasigu `--force`, konsciante ke **alia** reto estis elektita ol
  tiu, sur kiu la monujo estis antaŭe uzata;
- Forigu la malnovan eron el `networks` (redaktu la JSON).

### `aborted`

La uzanto respondis `n` (aŭ premis Enter) al la
invito `Continue? [y/N]:`. Nenio rompiĝis; la operacio estis nuligita.

## Ŝlosilujaj eraroj

### `unsupported keystore version <N>`

La dosiero estis skribita en formato ekster `{1, 2}`. Plej verŝajne ĝi
venas de alia programo aŭ de estonta versio. La nuna eldono de monujo
povas legi v1 (malnova scrypt) kaj v2 (argon2id); v1 estas migrata al v2 ĉe
la unua sukcesa malŝloso.

### `keystore v1 requires kdf "scrypt", got "..."` / `keystore v2 requires kdf "argon2id", got "..."`

Nekongrua kombinaĵo de `version` kaj `kdf` estis trovita en la dosiero
(ekz. `version: 2, kdf: "scrypt"`). Tio estas aŭ mane redaktita
dosiero aŭ interrompita migrado. Se vi ankoraŭ havas rezervkopion
`<name>.scrypt.bak` — renomu ĝin al `<name>.json` kaj rulu iun
varman komandon: ĝi migros denove.

### `note: migrated wallet "..." from scrypt to argon2id; legacy backup saved as <...>.scrypt.bak`

Informa mesaĝo al stderr (ne eraro): via malnova v1-dosiero
estis sukcese migrata al v2. Rezervkopio `<name>.scrypt.bak` aperis
apude. Vi povas forigi ĝin post kiam vi unufoje konfirmis, ke la
v2-dosiero funkcias per subskriba operacio
(`hot send-trx --from <name>` kun minimuma kvanto al testa adreso,
aŭ `offline sign`). `wallet list` kaj `hot balance` montras nur publikajn
metadatenojn kaj ne pruvas validecon de la ĉifroteksto.

### `warning: failed to migrate wallet "..." to argon2id: ...`

Migrado malsukcesis (ekz. neniu skribrajto en la dosierujo). La
origina operacio tamen **sukcese finiĝis** — la ŝlosilo estis
deĉifrita kaj uzata. Ĉe la sekva rulo monujo provos migri denove.

### Mana reveno v2 → v1

Se post migrado vi bezonas reveni al v1 (ekz. malpli nova binaraĵo
de monujo sen subteno de argon2id):

1. Forigu `<name>.json` (tio estas la nova v2).
2. Renomu `<name>.scrypt.bak` al `<name>.json`.
3. Rulu la malnovan konstruaĵon. La nova konstruaĵo migros la dosieron denove ĉe la
   unua malŝloso, do ne uzu ambaŭ versiojn kontraŭ la sama dosierujo
   samtempe.

### `empty network` (interna)

Levita en `EnsureNetworkRecorded` kiam `--network` normaliĝas al
malplena signoĉeno. Ne devus okazi en normala uzo.

### `broadcast ok (tx <id>) but could not update wallet networks metadata: ...`

La transakcio eliris, sed ĝisdatigo de la ŝlosiluja dosiero malsukcesis (ekz.
rajtoj ŝanĝiĝis). La operacio mem sukcesis — la monrimedoj estis
senditaj. La dosiero provos ĝisdatiĝi denove ĉe la sekva sukcesa sendo.

## Simptomoj kaj diagnozo

### «La programo longe ne respondas»

- Ĉu `--api-key` estas malplena kaj vi trafis la limon de petoj de TronGrid? Provu kun
  ŝlosilo.
- Ĉu HTTPS REST (la ĉefa transporto) estas neatingebla? Kontrolu
  `curl https://api.trongrid.io/v1/accounts/T...` aŭ ŝanĝu `--http-url`.
- Se nedeviga `--write-transport grpc` estas uzata: ĉu gRPC estas neatingebla
  (fajroŝirmilo/prokurilo/regiona bloko)? Provu `--grpc-url` kun alia
  publika nodo.

### «Saldo montras 0, sed la esplorilo montras monrimedojn»

- Malĝusta reto. La adreso estas la sama sur mainnet kaj Shasta, sed
  la saldoj malsamas. Kontrolu `--network`.
- La konto ne estas aktivigita — `data: []` en la respondo de TronGrid.
  La programo printas nulojn. Sendu la unuajn TRX per `hot send-trx` (aŭ
  atendu envenantan transigon).
- En `watch txs` vi eble ne vidas transakciojn se ili ne estas por la
  kanone enkonstruita USDT-kontrakto. La komando ne montras aliajn
  TRC-20-ĵetonojn.

### «Aliaj TRC-20-ĵetonoj videblas, sed ne en la eligo de monujo»

Tio estas laŭdezajne: `watch balance` kaj `watch txs` filtradas nur USDT
por la elektita reto. Ne ekzistas flago `--contract-address` en la
nuna versio.

### «`wallet list` estas malplena kvankam dosieroj ekzistas»

Kontrolu:

- `--keystore-dir` indikas la ĝustan dosierujon;
- dosieroj havas la finaĵon `.json` (rezervkopioj `.scrypt.bak` ne
  estas listigitaj — tio estas atendata);
- `version` ene de la JSON estas `1` aŭ `2`;
- la kampo `address` estas nemalplena;
- rajtoj ne malhelpas vian uzanton legi ilin.

Dosieroj, kiuj malsukcesas ĉi tiujn kontrolojn, estas silente preterpasitaj.

### «`--version` montras `dev (unknown)`»

La binaraĵo estis konstruita sen `-ldflags` (ekz. per `go build` sen
la Makefile). Konstruado per `make build` enmetas realajn valorojn el
`git describe`/`git rev-parse`.

## Kiam raporti cimon

Se vi observas:

- panikon el la norma biblioteko aŭ triapartiaj pakaĵoj;
- detruon/superskribon de alia monujo (en `safeName` aŭ
  `EnsureNetworkRecorded`);
- malsukcesan deĉifradon de dosiero, kiu certe estis uzata antaŭe kun
  la sama pasvorto;
- malĝustan averton «network mismatch» kiam la reto kongruas,

— aldonu al la cimraporto:

1. Version: `monujo --version`;
2. La plenan komandon sen sentemaj datumoj;
3. La erarmesaĝon;
4. Anonimizitan ŝlosilujan JSON (sen `data_hex`, `nonce_hex`,
   `salt_hex` — lasu `version`, `kdf`, `n/r/p` aŭ
   `argon2_*`, `address`, `networks`).

Kaj nepre indiku la OS-on kaj la Go-version uzitan por konstrui
(`go version`).
