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

## esperanto.txt (4096 words)

Top 4096 distinct Esperanto lemmas, lengths 3–7 characters (runes), built
from a Tekstaro corpus frequency list filtered with ESPDIC. Keeps full
Esperanto orthography including diacritics `ĉ ĝ ĥ ĵ ŝ ŭ`. NFKC-idempotent.

- Upstream: https://github.com/Vanege/esperanto-frequency-list-tekstaro
  (file: `EO 15000 Tekstaro filtered with ESPDIC.txt`)
- License: CC BY-SA 4.0.
- Size chosen as 4096 (= 2^12, exactly 12 bits of entropy per word).

## esperanto_x.txt (4096 words)

Same set as `esperanto.txt`, with the diacritics rewritten in Zamenhof's
x-system (`ĉ→cx`, `ĝ→gx`, `ĥ→hx`, `ĵ→jx`, `ŝ→sx`, `ŭ→ux`). ASCII-only;
typeable on any QWERTY/AZERTY keyboard without Esperanto input method.

- Derived from `esperanto.txt` by the generation script documented above.
- License: CC BY-SA 4.0 (inherits from source).

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

If you need to regenerate these lists (e.g. to bump sizes), follow the
steps recorded in the repository history (see the commit that introduced
the `internal/passphrase` package). The generation process is purely
deterministic given upstream inputs, but upstream files may change —
always re-run the embedded validation tests after regeneration.
