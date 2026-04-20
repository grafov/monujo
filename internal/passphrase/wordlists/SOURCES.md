# Wordlist sources

All wordlists are embedded via `//go:embed` from [passphrase.go](../passphrase.go).
Each file contains one word per line, UTF-8, no BOM, LF line endings, sorted
lexicographically within its own collation. Validation is performed at
package load time; see [passphrase_test.go](../passphrase_test.go).

## english.txt (7776 words)

EFF Long Wordlist. Original format is `<dice>\t<word>`; the leading dice
column has been stripped to keep only the word.

- Source: https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt
- License: CC BY 3.0 US (Electronic Frontier Foundation, 2016).

## english_short.txt (1296 words)

EFF Short Wordlist #1. Same formatting caveat as above.

- Source: https://www.eff.org/files/2016/09/08/eff_short_wordlist_1.txt
- License: CC BY 3.0 US (Electronic Frontier Foundation, 2016).

## esperanto.txt (14851 words)

Merged frequency lemmas from:

1. **Wiktionary** — [Frequency lists/Esperanto/Wikipedia
   2023](https://en.wiktionary.org/wiki/Wiktionary:Frequency_lists/Esperanto/Wikipedia_2023)
   (15 000 surface lemmas from the 2023-03-01 Esperanto Wikipedia dump;
   stored verbatim in `upstream/esperanto_wikipedia_2023.txt`).
2. **Tekstaro** — the previous embedded list (originally the top 4096
   lemmas from
   [Vanege/esperanto-frequency-list-tekstaro](https://github.com/Vanege/esperanto-frequency-list-tekstaro),
   file `EO 15000 Tekstaro filtered with ESPDIC.txt`), merged so rare
   Fundamento/PMEG roots not present in the Wiki list are retained.

Processing (see `gen/esperanto/main.go`): `strings.ToLower` → Unicode NFC
→ keep only words matching `^[a-zĉĝĥĵŝŭ]{3,12}$` (runes) → NFKC-idempotent
per [keystore.NormalizePassphrase](../../keystore/passphrase.go) → union and
dedupe → drop any lemma whose x-system form exceeds 12 runes (so
`esperanto` and `esperanto_x` stay the same cardinality) → if two lemmas
share one x-system spelling, keep the lexicographically smallest lemma →
sort. Full Esperanto orthography `ĉ ĝ ĥ ĵ ŝ ŭ`. Effective size **14851**
words (~**13.858** bits per word, `log2(14851)`).

- License: CC BY-SA 4.0 (Wiktionary content; Tekstaro upstream is also
  CC BY-SA 4.0).

## esperanto_x.txt (14851 words)

Same lemmas as `esperanto.txt`, with diacritics rewritten in Zamenhof's
x-system (`ĉ→cx`, `ĝ→gx`, `ĥ→hx`, `ĵ→jx`, `ŝ→sx`, `ŭ→ux`). ASCII-only;
typeable on any QWERTY/AZERTY keyboard without an Esperanto input method.

- Produced by the same generator as `esperanto.txt`.
- License: CC BY-SA 4.0 (inherits from the sources above).

## russian.txt (4096 words)

Top 4096 distinct Russian words by frequency from the Leeds Russian
Internet Corpus (via `hingston/russian`), lengths 3–7 characters (runes),
lowercase only, Cyrillic letters only (U+0430–U+044F plus U+0451 `ё` and
U+0439 `й`). NFKC-idempotent. Note that the list includes some proper
names that appear frequently in the corpus (e.g. common first names);
these are acceptable for passphrase generation.

- Upstream: https://github.com/hingston/russian (file:
  `10000-russian-words.txt`)
- License: MIT.
- Size chosen as 4096 (= 2^12, exactly 12 bits of entropy per word).

## Regeneration

- **Esperanto** (diacritic + x-system): from the repository root run  
  `go run ./internal/passphrase/wordlists/gen/esperanto`  
  The tool reads `upstream/esperanto_wikipedia_2023.txt` and the current
  `esperanto.txt`, then overwrites `esperanto.txt` and `esperanto_x.txt`.
  After changing upstream inputs, always run `make test` to re-validate
  embed constraints.

- Other lists: follow the steps recorded in the repository history if you
  need to bump sizes. Upstream files may change — always re-run the
  embedded validation tests after regeneration.
