// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("runGenpass", func() {
	It("rejects non-TTY output without override", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:     7,
			separator: " ",
			dict:      "english",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("non-TTY"))
	})

	It("works with allow-non-tty", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       5,
			separator:   " ",
			dict:        "english",
			allowNonTTY: true,
		})
		Expect(err).NotTo(HaveOccurred())

		out := buf.String()
		Expect(out).To(ContainSubstring("Generated passphrase"))
		Expect(out).To(ContainSubstring("Estimated entropy"))
	})

	It("rejects words less than 4", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       3,
			separator:   " ",
			dict:        "english",
			allowNonTTY: true,
		})
		Expect(err).To(HaveOccurred())
	})

	It("rejects unknown dictionary", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       7,
			separator:   " ",
			dict:        "klingon",
			allowNonTTY: true,
		})
		Expect(err).To(HaveOccurred())
	})

	It("prints Esperanto warning", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       5,
			separator:   " ",
			dict:        "esperanto",
			allowNonTTY: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("Esperanto diacritics"))
	})

	It("prints Russian warning", func() {
		var buf bytes.Buffer
		err := runGenpass(&buf, genpassArgs{
			words:       5,
			separator:   " ",
			dict:        "russian",
			allowNonTTY: true,
		})
		Expect(err).NotTo(HaveOccurred())

		out := buf.String()
		Expect(out).To(ContainSubstring("Cyrillic"))
		Expect(out).To(ContainSubstring("ё (U+0451)"))
	})

	It("does not print warnings for ASCII dictionaries", func() {
		for _, dict := range []string{"english", "english-short", "esperanto-x"} {
			var buf bytes.Buffer
			err := runGenpass(&buf, genpassArgs{
				words:       5,
				separator:   " ",
				dict:        dict,
				allowNonTTY: true,
			})
			Expect(err).NotTo(HaveOccurred(), dict)
			Expect(buf.String()).NotTo(ContainSubstring("WARNING:"), dict)
		}
	})
})

var _ = Describe("obtainNewKeystorePassphrase", func() {
	It("returns generated passphrase when enabled in non-TTY mode with override", func() {
		// When Enabled=false, obtainNewKeystorePassphrase eventually calls
		// readPassTwice which requires an interactive terminal. Here we only
		// validate the generate path.
		var buf bytes.Buffer
		_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
			Enabled:     true,
			Words:       5,
			Dict:        "english",
			Separator:   " ",
			AllowNonTTY: true,
		}, "ignored")
		Expect(err).NotTo(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("Generated passphrase"))
	})

	It("rejects invalid word count", func() {
		var buf bytes.Buffer
		_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
			Enabled:     true,
			Words:       2,
			Dict:        "english",
			Separator:   " ",
			AllowNonTTY: true,
		}, "ignored")
		Expect(err).To(HaveOccurred())
	})

	It("requires allow-non-tty override for non-terminal output", func() {
		var buf bytes.Buffer
		_, err := obtainNewKeystorePassphrase(&buf, generateFlags{
			Enabled:   true,
			Words:     5,
			Dict:      "english",
			Separator: " ",
		}, "ignored")
		Expect(err).To(HaveOccurred())
	})
})
