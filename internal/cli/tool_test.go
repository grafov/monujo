package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunGenpassNonTTYRefusedWithoutFlag(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:     7,
		separator: " ",
		dict:      "english",
	})
	if err == nil {
		t.Fatal("expected error when writer is not a TTY and --allow-non-tty is false")
	}
	if !strings.Contains(err.Error(), "non-TTY") {
		t.Errorf("error message does not mention TTY: %v", err)
	}
}

func TestRunGenpassAllowNonTTY(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:       5,
		separator:   " ",
		dict:        "english",
		allowNonTTY: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Generated passphrase") {
		t.Errorf("expected generated-passphrase header, got: %s", out)
	}
	if !strings.Contains(out, "Estimated entropy") {
		t.Errorf("expected entropy line, got: %s", out)
	}
}

func TestRunGenpassWordsTooFew(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:       3,
		separator:   " ",
		dict:        "english",
		allowNonTTY: true,
	})
	if err == nil {
		t.Fatal("expected error for words < 4")
	}
}

func TestRunGenpassUnknownDict(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:       7,
		separator:   " ",
		dict:        "klingon",
		allowNonTTY: true,
	})
	if err == nil {
		t.Fatal("expected error for unknown dict")
	}
}

func TestRunGenpassInputMethodWarningEsperanto(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:       5,
		separator:   " ",
		dict:        "esperanto",
		allowNonTTY: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Esperanto diacritics") {
		t.Errorf("expected esperanto input-method warning, got: %s", buf.String())
	}
}

func TestRunGenpassInputMethodWarningRussian(t *testing.T) {
	var buf bytes.Buffer
	err := runGenpass(&buf, genpassArgs{
		words:       5,
		separator:   " ",
		dict:        "russian",
		allowNonTTY: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cyrillic") || !strings.Contains(out, "ё (U+0451)") {
		t.Errorf("expected russian input-method warning, got: %s", out)
	}
}

func TestRunGenpassNoWarningForAsciiDicts(t *testing.T) {
	for _, dict := range []string{"english", "english-short", "esperanto-x"} {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       5,
			separator:   " ",
			dict:        dict,
			allowNonTTY: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.Contains(out, "WARNING:") {
			t.Errorf("%s: unexpected warning in output: %s", dict, out)
		}
	}
}

func TestObtainNewKeystorePassphrase_disabledFallsThrough(t *testing.T) {
	// When Enabled=false, obtainNewKeystorePassphrase must call
	// readPassTwice which needs a terminal. We cannot exercise the
	// happy path from a test; we only verify that the code path does
	// not panic when we flip Enabled on with allow-non-tty+bogus dict.
	var buf bytes.Buffer
	_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
		Enabled:     true,
		Words:       5,
		Dict:        "english",
		Separator:   " ",
		AllowNonTTY: true,
	}, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Generated passphrase") {
		t.Errorf("expected generated-passphrase header, got: %s", buf.String())
	}
}

func TestObtainNewKeystorePassphrase_invalidWordsRejected(t *testing.T) {
	var buf bytes.Buffer
	_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
		Enabled:     true,
		Words:       2,
		Dict:        "english",
		Separator:   " ",
		AllowNonTTY: true,
	}, "ignored")
	if err == nil {
		t.Fatal("expected error for --generate-words < 4")
	}
}

func TestObtainNewKeystorePassphrase_nonTTYRequiresOverride(t *testing.T) {
	var buf bytes.Buffer
	_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
		Enabled:   true,
		Words:     5,
		Dict:      "english",
		Separator: " ",
	}, "ignored")
	if err == nil {
		t.Fatal("expected error when writer is not a TTY and --generate-allow-non-tty is false")
	}
}
