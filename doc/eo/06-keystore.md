# 06. Ŝlosilujo (keystore)

`monujo` stokas privatajn ŝlosilojn en loka **ĉifrita ŝlosilujo**. Ĉiu
monujo estas aparta JSON-dosiero en la dosierujo agordita per
`--keystore-dir` / `MONUJO_KEYSTORE_DIR` / defaŭlte
`~/.config/monujo/keystore`.

## Lokigo kaj permesoj

- La ŝlosiluja dosierujo estas kreita aŭtomate
  (`os.MkdirAll(... 0o700)`) ĉe la unua skribo (`wallet create`/`wallet
  import`).
- Dosieroj estas kreitaj kun reĝimo `0600` (`os.WriteFile(... 0o600)`).
- La dosiernomo estas konstruita el `--name` per la funkcio `safeName`:
  - permesitaj karakteroj: `a-z A-Z 0-9 _ -`,
  - ĉiuj aliaj estas anstataŭigitaj per `_`,
  - malplena nomo fariĝas `default`.
  Ekzemple, `--name "My Cold #1"` kreas `My_Cold__1.json`.
- Ĉe skribo de metadatenoj (vidu malsupre), atoma anstataŭigo estas
  uzata per portempa dosiero `.monujo-keystore-*.tmp` kaj `os.Rename`.
- Legado de ŝlosilo iras tra `os.OpenRoot(keystoreDir)`: dosiero povas
  esti malfermita **nur** ene de la donita dosierujo; provo eskapi per
  simbolligo aŭ `..` estas garantiite malsukcesa ĉe la nivelo de
  sistemvoko, eĉ se truo aperas en `safeName`.

## Dosierformato v2 (nuna)

La nuna versio de la formato estas `2` kun KDF `argon2id`. La programo
ankaŭ legas dosierojn de versio `1` (legacy, scrypt) kaj ĉe la unua
sukcesa malŝloso **aŭtomate reverkas ilin al v2** (vidu „Migrado v1 →
v2“ malsupre). Versioj ekster `{1, 2}` donas
`unsupported keystore version <N>`.

Minimuma ekzemplo de v2-dosiero:

```json
{
  "version": 2,
  "address": "TLcVUPaYnQp...",
  "networks": ["mainnet"],
  "kdf": "argon2id",
  "salt_hex": "8a7b...e1f2",
  "argon2_time": 3,
  "argon2_memory": 65536,
  "argon2_threads": 4,
  "argon2_key_len": 32,
  "cipher": "aes-256-gcm",
  "nonce_hex": "5c1d...90ab",
  "data_hex": "fa12...34cd"
}
```

### Ekstera ero (nur adreso, sen ŝlosilo)

Dosiero kun `"external": true` stokas **nomitan** Tron-adreson sen
KDF kaj sen ĉifroteksto de ŝlosilo. Ĝi estas kreita per `monujo wallet
import-external`. Minimuma formo:

```json
{
  "version": 2,
  "external": true,
  "address": "TXmVpin5vq5gdZsciyyjdZgKRUju4st1wM",
  "networks": ["mainnet"]
}
```

Tiaj dosieroj partoprenas en `wallet list` kaj en nomsolvado en
komandoj, kiuj bezonas nur adreson; `LoadPrivateKey`, `wallet check`,
sendo kun `--from`, `offline sign`, kaj `multisig cosign` estas
**nedisponeblaj** por ili. La kampo `networks` por ekstera dosiero
estas agordita ĉe importo kaj **ne** estas aŭtomate etendita post
transakcioj (`EnsureNetworkRecorded` estas malpermesita por eksteraj
eroj).

### Kampoj de v2

| Kampo             | Tipo            | Priskribo                                                                                                                                             |
|-------------------|-----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| `version`         | int             | Ĉiam `2`.                                                                                                                                             |
| `address`         | string          | Tron-adreso en Base58Check. Kontrolita ĉe deĉifrado (se la deĉifrita ŝlosilo donas malsaman adreson — eraro `address mismatch in keystore`).         |
| `networks`        | array of string | Nedeviga. Retoj, en kiuj la monujo jam estis uzata. Unikaj, ordigitaj. `omitempty`.                                                                  |
| `kdf`             | string          | Ĉiam `"argon2id"` por v2.                                                                                                                             |
| `salt_hex`        | string          | Hazarda salo, 16 bajtoj (128 bitoj) en hex. Postulo de NIST SP 800-132 §5.1.                                                                          |
| `argon2_time`     | uint32          | Parametro `t` de Argon2id (pasoj). Defaŭlte `3`.                                                                                                     |
| `argon2_memory`   | uint32          | Parametro `m` de Argon2id (memorkosto) en KiB. Defaŭlte `65536` = 64 MiB.                                                                            |
| `argon2_threads`  | uint8           | Parametro `p` (paraleleco). Defaŭlte `4`.                                                                                                             |
| `argon2_key_len`  | uint32          | Longeco de la derivita ŝlosilo en bajtoj. Ĉiam `32` (por AES-256).                                                                                    |
| `cipher`          | string          | Nomo de ĉifro: `"aes-256-gcm"`.                                                                                                                       |
| `nonce_hex`       | string          | Hazarda GCM-nonce, 12 bajtoj en hex.                                                                                                                  |
| `data_hex`        | string          | Ĉifroteksto de privata ŝlosilo (32 bajtoj klarteksto + 16 bajtoj GCM-etikedo), en hex.                                                                |

### Dosierformato v1 (legacy, aŭtomate migrata)

Malnova formato antaŭ la migrado al argon2id. Kampoj `n, r, p` tenas
parametrojn de scrypt (`N=32768, r=8, p=1`), `kdf="scrypt"`, 32-bajta
salo. Ĉe la unua sukcesa `LoadPrivateKey` la dosiero estas reverktita
al v2; rezervokopio `<name>.scrypt.bak` restas apud ĝi kaj povas esti
forigita post la unua sukcesa kontrolo de la nova versio.

## Kriptografio (v2)

```
DK         = argon2id(passphrase, salt=16 rand, t=3, m=65536 KiB, p=4, dkLen=32)
ciphertext = AES-256-GCM(key=DK, nonce=12 rand, plaintext=raw_priv_key_32)
```

- **argon2id**: `golang.org/x/crypto/argon2`, RFC 9106. Parametroj
  `t=3, m=64 MiB, p=4` kongruas kun la profilo „server“ el la OWASP
  Password Storage Cheat Sheet. Valoroj estas stokitaj en la dosiero,
  do kiam defaŭltoj altiĝos en estontaj versioj, ekzistantaj dosieroj
  restos legeblaj.
- **AES-256-GCM**: `crypto/aes` + `crypto/cipher.NewGCM`. La
  aŭtentiga etikedo (16 bajtoj) estas aldonita al `ciphertext`.
- **Fonto de hazardo**: `crypto/rand.Reader` por salo (16 bajtoj) kaj
  nonce (12 bajtoj). Forto kongruas kun la OS.
- **Viŝado**: post deĉifrado kaj subskribo la programo nuligas:
  - `passphrase` (`ZeroPassphrase`);
  - la derivitan ŝlosilon argon2id/scrypt (`zeroBytes`);
  - la klartekstan bufron de privata ŝlosilo (`zeroBytes`);
  - materialon de ECDSA-ŝlosilo (`zeroPriv` nuligas `D.Bits()`).

  Protekto kontraŭ elfluoj per swap/core-dumpoj **ne estas garantita**
  kaj dependas de la OS.

## Migrado v1 → v2

Aŭtomata unufoja proceduro:

1. Vi rulas iun komandon, kiu deĉifras la ŝlosilon: `hot send-trx`, `hot
   send-usdt`, `offline sign`, aŭ `wallet check`. `hot balance` ne
   deĉifras la ŝlosilon (ĝi legas la adreson el malfermaj metadatenoj),
   do ĝi **ne** ekigas migradon.
2. monujo sukcese deĉifras la v1-dosieron per la scrypt-ŝlosilo kaj
   kontrolas la adreson.
3. **Nur post sukcesa kontrolo** la originala `<name>.json` estas
   alinomita al `<name>.scrypt.bak`, kaj freŝa v2/argon2id-dosiero
   okupas ĝian lokon (atoma skribo per tmpfile + `rename`).
4. Linio estas printita al stderr: `note: migrated wallet "..." from
   scrypt to argon2id; legacy backup saved as <name>.scrypt.bak`.

Invariantoj:

- Se migrado malsukcesas post paŝo 2, la programo tamen redonas la
  ŝlosilon al la alvokanta operacio (ĝi jam sukcese deĉifris). La
  sekva rulo reprovos migri.
- Ĉe malĝusta pasvorto — nek migrado nek rezervokopio.
- Se `<name>.scrypt.bak` jam ekzistas (ekzemple vi nuligis operacion
  kaj rerulis ĝin post redaktado de la dosiero), la **dua
  rezervokopio ne superskribas la unuan** — ĝi ricevas sufikson
  `<name>.scrypt.bak.<unix-ms>`.
- `networks` el v1 estas portitaj en v2 senŝanĝe.

Kiam forigi `<name>.scrypt.bak`: post kiam vi konfirmis, ke iu
subskriba operacio (`hot send-trx`, `hot send-usdt`, aŭ `offline sign`)
sur la nova v2-dosiero funkcias kiel atendite — tio sola pruvas, ke la
ĉifroteksto estas ĝusta. **Sen reto** la sama estas pruvita per
`wallet check --addr ...`: ĝi deĉifras la ŝlosilon kaj kontrolas la
adreson sen sendi transakciojn. `wallet list` montras nur malfermajn
metadatenojn kaj ne garantias validecon de la ĉifrita enhavo.
Ĝis tiam `.bak` estas via asekuro se eksteraj kondiĉoj (disko, FS)
koruptis la v2-dosieron dum skribo.

Mana restaŭro: se pro iu kialo vi bezonas la malnovan version, forigu
`<name>.json`, alinomi `<name>.scrypt.bak` reen al `<name>.json`, kaj
rulu malnovan konstruon de monujo. La nova konstruo ne malfermos
miksajn kombinojn (`v2+scrypt` / `v1+argon2id`) — tio protektas kontraŭ
akcidente koruptita dosiero.

## Kongrueco kun ne-ASCII-pasvortoj antaŭ NFKC-normaligo

Komencante de la versio, kiu aldonis NFKC-normaligon (vidu
„Kongruo kun normoj“ malsupre), ĉiuj pasfrazoj estas alkondukitaj al
unuforma formo antaŭ la KDF. Por ASCII-pasvortoj ĉi tiu ŝanĝo estas
**travidebla**: NFKC estas idento sur ASCII, bajtoj ne ŝanĝiĝas.

Se vi kreis monujon sur antaŭa konstruo de monujo kun **ne-ASCII**-
pasvorto (cirila, umlaŭtoj, ligaturoj, plenlarĝaj, emoji), unu el du
aferoj povas okazi:

- La originala bajta formo jam kongruas kun NFKC — ĉio funkcias
  senŝanĝe (tipa en Linux/Windows, kie la terminalo jam liveras
  komponitan formon).
- La originala bajta formo diferencas de NFKC — ekzemple macOS
  liveras NFD (`"café"` kiel `e`+`\u0301`), kaj malnova monujo
  pasigis tiujn bajtojn en argon2id „kiel estas“. Post la ĝisdatigo
  la samaj klavpremoj sur la sama maŝino fariĝas NFC-formo, kaj la
  malnova monujo **ĉesas malŝlosiĝi**.

Du vojoj por ripari tion:

1. **Reiru al la malnova konstruo**, malfermu la monujon, akiru la
   privatan ŝlosilon (ekzemple per skripto uzanta la malnovan
   `keystore.LoadPrivateKey`), kaj rekrei per `wallet import` sur la
   nova konstruo kun iu ajn pasvorto. Ŝlosilo kaj adreso restas la
   samaj.
2. Se vi ne havas malnovan konstruon — mane „kunmetu“ la pasvorton
   el la atendataj bajtoj kaj pipu ĝin en monujo sen terminala
   normaligo. Tio estas unufoja hako; la ĝusta vojo estas opcio #1.

**ASCII-pasvortoj ne havas ĉi tiun problemon.** Se vi ne certas, ĉu
vi antaŭe uzis ne-ASCII-pasvorton — post ĝisdatigo kontrolu malŝloson
de monujo: **`wallet check --addr ...`** (sen reto) aŭ subskriba
operacio (`hot send-trx --from ...` kun minimuma sumo al testa
adreso, aŭ `offline sign`). `wallet list` kaj `hot balance` ne
petas pasvorton kaj ne diagnozaz ĉi tiun problemon.

## Kongruo kun normoj

- **NIST SP 800-132 — Password-Based Key Derivation.** 128-bita salo
  el `crypto/rand`, unika por ĉiu dosiero; parametroj de KDF estas
  stokitaj en la dosiero. Postuloj pri salo estas plenumitaj. SP
  800-132 listigas PBKDF2 kiel aprobatan KDF; monujo uzas Argon2id
  kiel modernan alternativon, kiu **ne** ekvivalentas al la
  FIPS-aprobata listo (vidu malsupre).
- **OWASP Password Storage Cheat Sheet.** Parametroj de v2 (`t=3, m=64
  MiB, p=4, keyLen=32`) kongruas kun la profilo „server“. La pli
  malalta akceptebla sojlo en la ĉeftabelo estas `t=2, m=19 MiB, p=1`;
  se sub viaj kondiĉoj la OWASP-defaŭlto estas troa por rendimento,
  parametroj povas esti mallevitaj per rekta redaktado de la dosiero
  — ili estas stokitaj en la kampoj `argon2_*`.
- **NIST SP 800-63B §5.1.1.2 — Memorized Secrets.** monujo akceptas
  arbitran UTF-8-pasfrazon, ne devigas rotacion, kaj ne limigas
  longecon de supre. **Unicode-NFKC-normaligo** estas aplikata antaŭ
  ĉiu alvoko de KDF (Argon2id v2, scrypt v1, scrypt por senreta
  HMAC). Tio signifas:
  - `"café"` (NFC, unuopa kodpunkto `U+00E9`) kaj `"cafe\u0301"`
    (NFD, `e`+`\u0301`) donas la saman ŝlosilon;
  - ligaturoj kiel `"ﬁle"` (`U+FB01`+`le`) estas ekvivalentaj al
    `"file"` (NFKC-kongrueca mapado);
  - plenlarĝaj variantoj `"ｐａｓｓ"` estas ekvivalentaj al ASCII `"pass"`.

  Limo: NFKC **ne** egaligas vide simile sed kodpunkt-malsamajn
  karakterojn (cirila `а` ≠ latina `a`, greka `ο` ≠ latina
  `o`). Uzu unu klavaran aranĝon ĉe enigo de la pasvorto.
- **NIST SP 800-108 — KDF from PRF.** Regas derivadon de ŝlosiloj el
  aliaj ŝlosiloj (ekz. HKDF); ĝi ne aplikiĝas al pasvortaj KDF-oj kaj
  ne kovras monujo.
- **FIPS 140-3 / ISO/IEC 19790 — Requirements for Cryptographic
  Modules.** monujo **ne** estas atestita kriptografia modulo.
  Argon2id **ne** estas en la FIPS-aprobata listo de KDF (FIPS 140-3
  per SP 800-140C / SP 800-132 permesas nur PBKDF2-HMAC-SHA2 por
  pasvortoj). Por medioj, kiuj postulas kongruon kun FIPS 140-3 aŭ
  ISO/IEC 19790, monujo en sia nuna formo **ne taŭgas** — uzu
  atestitan monujon aŭ atendu apartan reĝimon PBKDF2 (forestanta en
  ĉi tiu versio).

Fina poziciigo: podetal- / disvolva utilaĵo, kiu sekvas OWASP-
parametrojn kaj postulojn de SP 800-132 pri salo, sed **ne** estas
FIPS-kongrua kaj **ne** atestita.

## Rezerva kopio de keystore

La dosiero `<name>.json` estas memsufiĉa: ĝi tenas la adreson, salon,
parametrojn de KDF, nonce, kaj ĉifrotekston. La pasfrazo **ne** estas
stokita en la dosiero. Rezerva kopio = dosiero + pasvorto, kaj ili
devas esti tenataj aparte.

### Minimuma proceduro

```bash
mkdir -p /path/to/backup
cp ~/.config/monujo/keystore/<name>.json \
   /path/to/backup/<name>.$(date +%F).json
chmod 600 /path/to/backup/<name>.*.json
```

Kontrolu restaŭron sur alia medio:

```bash
monujo --keystore-dir /path/to/backup wallet list
```

La adreso en la eligo devas kongrui kun la originalo. Por pli forta
kontrolo — rulu iun subskriban operacion (`hot send-trx --from <name>`
kun minimuma sumo al testa adreso, aŭ `offline sign`) kun tiu
`--keystore-dir`: nur tio pruvas, ke la ĉifrita materialo deĉifriĝas
kun la sama pasvorto (nulaj sumoj por send-trx estas en ordo; kio
gravas estas la kapablo malŝlosi).

### Utilaĵo `tool vault` (rekomendata vojo)

`monujo tool vault pack` pakas la **tutan ŝlosilujan dosierujon** en
unu ĉifritan age-arkivon (scrypt-pasfraza ricevanto, stilo RFC
9106); `monujo tool vault unpack` faras la malon. Tio estas maldika
envolvaĵo super la samaj primitivaj kiel la age-CLI — sen exec de
ekstera binaraĵo, pura Go-dependeco `filippo.io/age`.

```bash
# Paki la tutan ŝlosilujon en unu dosieron (pasvorto petata dufoje).
monujo tool vault pack --out backup-$(date +%F).age

# Malpaki en la celan dosierujon (defaŭlte --keystore-dir).
monujo tool vault unpack --in backup-2026-04-21.age --out /restore/keystore

# Kontroli, ke adresoj kongruas.
monujo --keystore-dir /restore/keystore wallet list
```

Kio eniras la arkivon: nur `*.json` kaj
`*.scrypt.bak[.<unix-ms>]`. Ĉio alia (subdosierujoj, punktdosieroj,
`.monujo-keystore-*.tmp`) estas silente ignorata. Nomoj ene de la tar
estas „plataj“, sen dosieruja prefikso, do malpakado estas portebla.

Flagoj:

- `--armor` (pack) — ASCII-kiraso (PEM-simila teksto), oportuna por
  kopiado per retpoŝto/git/printaĵo.
- `--overwrite` (pack) — permesi superskribon de ekzistanta dosiero
  `--out`. Anstataŭigo estas atoma: `monujo` skribas al frata
  `.monujo-vault-*.tmp` kaj post sukcesa pako alinomas ĝin super la
  malnova arkivo; malsukcesa pako ne damaĝas la antaŭan rezervon. Sur
  TTY la flago tamen petas `[y/N]` ankoraŭ unufoje.
- `--overwrite` (unpack) — permesi superskribon de ekzistantaj
  dosieroj en `--out`. Defaŭlte unpack rifuzas se almenaŭ unu cela
  dosiero jam ekzistas (kontrolo antaŭ ajnaj skriboj; nenia parta
  malpako). Sur TTY la flago tamen petas `[y/N]` ankoraŭ unufoje.
- `--dry-run` (unpack) — plena validigo (deĉifrado, tar-analizo,
  permeslisto, konfliktokontrolo) sen unu sola skribo al disko.
  Taŭga kiel „kontrolo de vault“.

Limigoj:

- Defaŭlte `--out` por `pack` ne devas ekzisti (protekto kontraŭ
  superskribo de preta rezervokopio); anstataŭigo nur kun eksplicita
  `--overwrite`. Stdout estas permesata per `--out -`, kie
  `--overwrite` ne estas bezonata.
- La vault-pasvorto estas **aparta ento** de la pasvortoj de
  ŝlosilujaj dosieroj interne. Perdu la vault-pasvorton — perdu la
  arkivon; perdu unu ŝlosilujan pasvorton — perdu tiun monujon eĉ kun
  sendifekta arkivo. Stoku la vault-pasvorton je la sama sekureca
  nivelo kiel ŝlosilujajn pasvortojn (administrilo / papera koverto),
  **ne** sur la sama medio kiel la arkivo.
- Kongrueco kun age: la dosiero teorie povas esti malfermita per
  simpla `age -d` (vi ricevas tar interne), sed tio estas
  implementada detalo, ne promeso pri stabileco de formato; uzu
  `monujo tool vault unpack` por fidinda analizo kun
  permeslistaj kontroloj.

### Tenu la pasvorton aparte

- Pasvorta administrilo (1Password, KeePass, Bitwarden) — ĉefa
  rekomendata opcio.
- Papera koverto en sekurŝranko — por longdaŭraj malvarmaj
  rezervokopioj.
- **Neniam** metu la pasvorton sur la saman medion kiel la `.json`,
  kaj ne konservu ĝin en nuba sinkronigo sen ekstra ĉifra tavolo.

### Ekstra ĉifra tavolo (rekomendata)

Super la ĉifro de monujo vi povas envolvi la dosieron per alia
pasvorto. Ekzemple kun `age`:

```bash
age -p -o wallet.json.age ~/.config/monujo/keystore/<name>.json
age -d -o /path/to/restored/<name>.json wallet.json.age
```

Aŭ GPG:

```bash
gpg -c --cipher-algo AES256 ~/.config/monujo/keystore/<name>.json
gpg -d <name>.json.gpg > /path/to/restored/<name>.json
```

Tia rezervokopio estas pli sekura por meti en la nubon / retpoŝton /
komunajn retajn diskojn: eĉ se la dosiero kaj metadatenoj de monujo
elfluas, ekstera pasvorto restas. Se vi bezonas envolvi la **tutan
dosierujon**, ne ĉiun dosieron aparte — uzu `monujo tool vault pack`
(vidu supre): la sama primitivo (age scrypt-reĝimo), sed kiel unu
arkivo kun permeslistaj kontroloj ĉe malpako.

### Geografia disigo

Almenaŭ **du** kopioj:

- ĉefa labora medio (ekstera SSD/USB, apartigita de la maŝino
  rulanta monujo);
- malvarma stokado (USB en sekurŝranko / fora fizika loko / nubo kun
  ekstera age/GPG-pasvorto).

Por aer-interspaca scenaro (vidu `10-offline-commands.md`)
rezervokopio de ŝlosilujo estas **deviga** kaj ne devas koincidi kun
retaj maŝinoj (`build`/`broadcast`).

### Kio NE estas rezervokopio

- `privHex` el `wallet create --export-private-key
  --allow-private-key-export` sen pasvorto — tio estas „nuda“
  privata ŝlosilo, ne rezervokopio: sen envolvaĵo, ĝia elfluo egalas
  ŝtelon de monoj. Se vi ĝin skribis, stoku ĝin kiel pasvorton
  (administrilo / papero), forigu ĝin el `~/.bash_history` /
  tondujo.
- Ekrankopio kun pasvorto aŭ hex-ŝlosilo.
- Kopio de nur la pasvorto sen la `.json`.
- Kopio de nur la `.json` sen la pasvorto (sed tio estas ĉifro, ne
  „neniu rezervokopio“: la dosiero ekzistas, aliro al la ŝlosilo
  ne).

### Rotacio kaj migrado

Movo al nova maŝino: kopiu la `.json`, enigu la saman pasvorton. Se
la dosiero estas ankoraŭ v1 — aŭtomata migrado ruliĝas ĉe la unua
malŝloso, post kio sinkronigu rezervokopiojn de la nova v2-versio. La
v1-dosiero ne estas bezonata post sukcesa migrado, sed
`<name>.scrypt.bak` povas esti tenata kiel kriza kopio ĝis la unua
sukcesa operacio sur v2.

### Reakiro de aliro se la pasvorto estas perdita

**Neeble.** Argon2id estas desegnita tiel, ke brutforta atako kontraŭ
pasvorto estas kompute nepraktika ĉe normala longeco de pasfrazo. Tio
estas deklarita operacia risko (FIPS 140-3 / ISO/IEC 19790 kaj SP
800-63B eksplicite antaŭvidas ĝin): la sola rimedo estas fidinda
rezervokopio de la **pasvorto** mem. monujo ne solvas ĉi tiun
problemon kaj ne pretendas tion.

## Metadatenoj `networks`

La kampo `networks` estas „taglibro“ de uzoj de monujo, helpanta
eviti konfuzon de mainnet kun testreto.

### Kiam ĝi estas plenigita

- Ĉe `wallet create --network <X>` kaj `wallet import --network <X>` —
  registrita kiel `[X]`.
- Post sukcesa `hot send-trx`/`hot send-usdt` kun
  `--network <X>` — se `X` mankas, ĝi estas aldonita
  (`EnsureNetworkRecorded`). La ĉifro ne estas rekonstruita; nur
  metadatenoj ŝanĝiĝas; la dosiero estas atome reverktita.

### Kiam ĝi estas kontrolita

En tri komandoj: `hot balance`, `hot send-trx`, `hot send-usdt`.
Logiko: se la tabelo `networks` estas **ne-malplena** kaj la nuna
`--network` estas **forestanta** el ĝi, la programo printas averton
al `stderr`:

```
Warning: wallet "myhot" was previously used on network(s): mainnet
The current operation uses network "shasta", which is not in that list.
This wallet has already been used in operations on the listed network(s).
```

Poste:

- se `--force` estas agordita — daŭrigas sen demandoj;
- se STDIN estas TTY — demandas `Continue? [y/N]:`. Respondo `y`/`yes`
  (sen distingi majusklon) — daŭrigi; io ajn alia — eraro `aborted`;
- se STDIN ne estas TTY — tuj eraro `network mismatch: use
  --force to proceed without a terminal, or run interactively to
  confirm`.

### Normaligo

- Ĉiu enigo estas minuskligita kaj mapita al unu el la kanonaj valoroj
  `mainnet`/`shasta`.
- Duplikatoj estas kunfanditaj.
- La listo estas ordigita (`slices.Sort`).
- Malplenaj/nekonataj valoroj estas forĵetitaj.

Ne ekzistas eksplicita komando por forigi eron el `networks` — la
tabelo nur kreskas. Se vi bezonas „forgesi“ antaŭan reton, redaktu
la JSON mane (la kampo `networks`).

## Kiel la programo trovas dosieron laŭ nomo aŭ adreso

`hot balance`, `hot send-*`, `offline build`/`sign`, kaj `wallet check`
serĉas la ŝlosilujon per **`--addr`** (objekto sen `--to` en la
CLI) aŭ **`--from`/`--to`** (transigoj kaj nesubskribita konstruo).
Komandoj `wallet create`, `wallet import`, kaj `wallet import-external`
ne permesas `--name`, kiu post normaligo aspektas kiel valida
Tron-base58-adreso — alie tiu nomo poste kolizius kun la heŭristiko
„adreso aŭ nomo“ en la samaj flagoj.

Por eroj **`import-external`** (`"external": true`) anstataŭigo de
adreso per nomo funkcias kie ajn ordinara monujo bezonas nur la
kampon `address` en JSON; operacioj kun **privata ŝlosilo**
(`LoadPrivateKey`, `EnsureNetworkRecorded` por aldoni reton post
elsendo) estas malpermesitaj por tiaj dosieroj.

1. Se la valoro trapasas validigon de Tron base58 — ĝi estas traktata
   kiel adreso.
   - Por **`--from`** en `hot send-*`, por **`--addr`** en
     `offline sign`/`wallet check` la programo serĉas kongruon per
     `keystore.ListWallets` (laŭ la kampo `address` en klartekstaj
     metadatenoj, sen deĉifrado); la adreso devas respondi al loka
     dosiero, alie la komando malsukcesas (privata ŝlosilo estas
     bezonata).
   - Por **`--addr`** en `hot balance`, por **`--from`** en
     `offline build`, kaj por **`--to`** ĉie, malkongruo kun la
     loka listo estas permesata: `balance` povas peti arbitran
     adreson, `offline build` sur reta maŝino ne deĉifras la
     ŝlosilon, kaj `--to` estas arbitra ricevanta adreso.
2. Se la valoro NE aspektas kiel adreso, ĝi estas traktata kiel
   nomo:
   - `safeName` estas aplikata (konverto al sekura dosiernomo).
   - Dosiero `<safeName>.json` estas malfermita per
     `os.OpenRoot(keystoreDir)`.
   - JSON estas analizita. Se `version` ne estas en `{1, 2}` — eraro
     `unsupported keystore version`.
   - Por `--to` nur la klarteksta kampo `address` estas prenita el
     ĉi tiu dosiero; nenia deĉifrado de privata ŝlosilo okazas.

La sama aplikiĝas al `wallet *` (per `--name`). Tio signifas: se vi
origine kreis monujon nomitan `My Cold`, la dosiero estos
`My_Cold.json`, kaj vi povas alparoli ĝin kaj kiel `--from "My Cold"`
(en `send-trx` / `offline build`) kaj kiel `--addr "My Cold"` (en
`hot balance` / `wallet check` / `offline sign`): la ĉeno estas
normaligita al la sama dosiernomo.

## Generilo de pasfrazoj

monujo inkluzivas enkonstruitan generilon de pasfrazoj sur
`crypto/rand` kun uniforma specimenado sen modulo-biaso. Komando:

```bash
./build/monujo tool genpass [--words N] [--dict DICT] [--separator S]
                            [--capitalize] [--digits K]
                            [--allow-non-tty]
```

La samaj parametroj estas disponeblaj kiel opcioj de `wallet create`
kaj `wallet import` per la prefikso `--generate-*`; kun
`--generate-passphrase` nova frazo estas generita, printita unufoje
al stderr, kaj tuj uzata kiel pasvorto de la nova ŝlosiluja dosiero.
La flago `--generate-allow-non-tty` estas la analogo de
`--allow-non-tty` por la enkonstruita kazo.

### Invariantoj

- Fonto de hazardo — **nur** `crypto/rand.Reader`. Uniforma
  specimenado per `rand.Int(rand.Reader, big.NewInt(N))`.
- La frazo estas NFKC-normaligita ĉe eligo: la bajtoj, kiujn vi vidas
  sur ekrano, kongruas bit-por-bite kun tio, kio eniras Argon2id.
- Eligo estas **nur** al stderr kaj **nur** unufoje. La frazo neniam
  estas skribita al stdout (oportune en pipoj), nek konservita al
  dosiero / tondujo / medivariablo.
- Se stderr ne estas terminalo, la CLI rifuzas printi la frazon sen
  `--allow-non-tty` (aŭ `--generate-allow-non-tty`) — protekto kontraŭ
  akcidenta faligo de pasvorto en CI-protokolojn.
- Post printado la bufro estas viŝita per `ZeroPassphrase`.

### Vortaroj

| Flago `--dict`   | Grandeco | Karakteroj     | Bitoj/vorto | Kiam elekti                                               |
|------------------|----------|----------------|-------------|-----------------------------------------------------------|
| `english`        | 7776     | ASCII          | 12.92       | Defaŭlte, EFF Long Wordlist, kanona.                      |
| `english-short`  | 1296     | ASCII          | 10.34       | Oportuna por diktado laŭte; mallongaj frazoj.             |
| `esperanto-x`    | 14851    | ASCII          | 13.86       | Esperanto en x-sistemo; sekura sur iu ajn klavaro.        |
| `esperanto`      | 14851    | UTF-8 (ĉĝĥĵŝŭ) | 13.86       | Nur se enigo estas agordita sur ĉiuj malŝlosaj maŝinoj.   |
| `russian`        | 15213    | UTF-8 (а-я,ёй) | 13.89       | Nur kun garantita cirila IME ĉie.                         |

La vortaroj `esperanto` kaj `esperanto-x` estas kunfando de la
frekvenca listo de Wiktionary/Wikipedia 2023 (15 000 lemoj) kaj la
pli frua listo Tekstaro (vidu
[internal/passphrase/wordlists/SOURCES.md](../../internal/passphrase/wordlists/SOURCES.md)):
post `strings.ToLower` restas nur minusklaj literoj `a–z` kaj `ĉ ĝ ĥ ĵ ŝ ŭ`,
longeco 3–12 runoj; ambaŭ flagoj dividas la saman enhavon (x-sistemo
estas mekanika anstataŭigo de diakritaj signoj).

La vortaro `russian` estas la frekvenca listo de Ljaŝevskaja–Ŝarov
(Rusa Nacia Korpuso, 2009), 20 004 lemoj en la spegulo de
Wiktionary; post `strings.ToLower` restas nur minusklaj literoj
`а-я ё й` de longeco 3–10 runoj — entute 15 213 vortoj. Ortografioj
sen `ё` (`бобер`, `еще`) estas konservitaj kiel en la fonto; NFKC
distingas `ё/е` kaj `й/и`, vidu malsupre.

Minimumo — 4 vortoj, defaŭlte — 7 vortoj. Kiam atendata entropio
estas sub 80 bitoj, la CLI printas averton sed ne abortas.

### Avertoj por ne-ASCII-vortaroj

Ĉe elekto de `--dict esperanto` aŭ `--dict russian`, antaŭ la
frazo mem la CLI printas apartan WARNING pri enpostuloj:

- `esperanto` — averto pri bezono tajpi `ĉ ĝ ĥ ĵ ŝ ŭ` sur ĉiu
  malŝlosa maŝino; `esperanto-x` estas rekomendata en scenaroj sen
  agordita Esperanta aranĝo.
- `russian` — averto pri cirila IME, diferencoj `ё/е` kaj `й/и`
  post NFKC, kaj la danĝero de latinaj vidaj klonoj
  (`a/а`, `o/о`, `e/е`).

### Kion NFKC solvas kaj ne solvas

NFKC-normaligo estas aplikata al la frazo antaŭ ol ĝi eniras
Argon2id, do malsamaj manieroj enigi **la saman kodpunkton** donas
la saman ŝlosilon:

- `ĉ` (U+0109) ≡ `c`+U+0302 (malkompono).
- `й` (U+0439) ≡ `и`+U+0306.
- `ё` (U+0451) ≡ `е`+U+0308.
- Ligaturoj `ﬁ` (U+FB01) → `fi`, plenlarĝaj `ｐａｓｓ` → `pass`, ktp.

NFKC **ne** egaligas:

- `cx` kaj `ĉ` — tio estas malsamaj Esperantaj ortografiaj sistemoj
  (x-sistemo kontraŭ supersignoj); elektu unu vortaron unufoje kaj
  ne miksu.
- `ё` kaj `е` — malsamaj kodpunktoj; frazoj `ещё` kaj `еще` donas
  **malsamajn** ŝlosilojn. La sama validas por `й` kaj `и`.
- Vidaj klonoj: cirila `а` (U+0430) kaj latina `a` (U+0061) estas
  malsamaj literoj. Tenu unu aranĝon ĉe enigo de la pasvorto.

Se la frazo estas perdita — ĝi estas perdita por ĉiam (vidu
§„Reakiro de aliro se la pasvorto estas perdita“).

### Ekzemploj

```bash
# 7-vorta EFF Long, spaca apartigilo (defaŭlte, ~90 bitoj).
./build/monujo tool genpass

# 8 vortoj + 2 finaj ciferoj, unuaj literoj majuskligitaj.
./build/monujo tool genpass --words 8 --capitalize --digits 2

# Rusa vortaro, 7 vortoj, streketo kiel apartigilo.
./build/monujo tool genpass --dict russian --separator -

# Krei novan monujon tuj kun generita pasvorto.
./build/monujo wallet create --name cold --generate-passphrase \
    --generate-words 8 --generate-dict english
```

## Ŝanĝo de la pasfrazo

Ankoraŭ ne ekzistas rekta CLI-mekanismo por ŝanĝi la pasvorton.
Portempa solvo: deĉifri la ŝlosilon (per ekvivalento de
`LoadPrivateKey` — neeble el la ŝelo, tio estas API), aŭ ĉe kreo de
nova monujo konservi `privHex` unufoje, rekrei per `wallet import` kun
nova frazo, kaj sekure forigi la malnovan dosieron. Aparta komando
`wallet rekey` estas planita sekvaĵo.

## Kio **ne** estas en la ŝlosilujo

- La pasfrazo. La programo neniam konservas ĝin al disko.
- Fremdaj adresoj (ricevantoj). La programo ne tenas
  „adreslibron“.
- Protokoloj de operacioj. monujo skribas neniujn ĵurnalojn krom
  ĝisdatigo de `networks`.
- Iuj ne-Tron-ŝlosiloj (Solana/BEP — ekster amplekso de la nuna
  versio).

## Kongrueco kun aliaj ŝlosilujoj

La formato de monujo **ne** estas rekte kongrua kun la ŝlosilujo de
go-ethereum aŭ kun TronLink (propra formato). Por migrado vi povas:

- el go-ethereum: deĉifri tie, akiri la hex-ŝlosilon, importi per
  `wallet import --key-hex`;
- el TronLink/mnemonikoj: eksporti la privatan ŝlosilon de la
  bezonata konto (per etendaĵo / wallet CLI), importi.

`monujo` intence funkcias nur kun „nudaj“ secp256k1-privataj ŝlosiloj
— sen BIP39, sen HD-derivado.
