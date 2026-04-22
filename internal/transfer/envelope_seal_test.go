package transfer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sealed envelope operations", func() {
	It("round-trips payload without HMAC", func() {
		payload := []byte(`{"hello":"world"}`)
		blob, err := SealPayload(KindUnsignedTransfer, payload, nil)
		Expect(err).NotTo(HaveOccurred())

		got, fp, err := OpenEnvelope(blob, KindUnsignedTransfer, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(payload))
		Expect(fp.SHA256Hex).To(HaveLen(64))
	})

	It("round-trips payload with HMAC", func() {
		key, err := DeriveHMACKey([]byte("a strong passphrase"))
		Expect(err).NotTo(HaveOccurred())

		payload := []byte(`{"k":"v"}`)
		blob, err := SealPayload(KindSignedTransfer, payload, key)
		Expect(err).NotTo(HaveOccurred())

		got, _, err := OpenEnvelope(blob, KindSignedTransfer, key)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(payload))
	})

	It("rejects envelope kind mismatch", func() {
		blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), nil)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = OpenEnvelope(blob, KindSignedTransfer, nil)
		Expect(err).To(HaveOccurred())
	})

	It("round-trips multisig payload without HMAC", func() {
		payload := []byte(`{"hello":"multisig"}`)
		blob, err := SealPayload(KindMultisigTransfer, payload, nil)
		Expect(err).NotTo(HaveOccurred())

		got, _, err := OpenEnvelope(blob, KindMultisigTransfer, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(payload))
	})

	It("round-trips multisig payload with HMAC", func() {
		key, err := DeriveHMACKey([]byte("multisig-passphrase"))
		Expect(err).NotTo(HaveOccurred())

		payload := []byte(`{"k":"v"}`)
		blob, err := SealPayload(KindMultisigTransfer, payload, key)
		Expect(err).NotTo(HaveOccurred())

		got, _, err := OpenEnvelope(blob, KindMultisigTransfer, key)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(payload))
	})

	It("rejects tampered payload when HMAC is present", func() {
		key, err := DeriveHMACKey([]byte("p1"))
		Expect(err).NotTo(HaveOccurred())

		blob, err := SealPayload(KindUnsignedTransfer, []byte(`{"a":1}`), key)
		Expect(err).NotTo(HaveOccurred())

		var env Envelope
		err = json.Unmarshal(blob, &env)
		Expect(err).NotTo(HaveOccurred())

		tampered := []byte(strings.Replace(string(blob), env.PayloadB64,
			base64.StdEncoding.EncodeToString([]byte(`{"a":2}`)), 1))
		_, _, err = OpenEnvelope(tampered, KindUnsignedTransfer, key)
		Expect(errors.Is(err, ErrHMACMismatch)).To(BeTrue())
	})

	It("rejects envelopes opened with the wrong passphrase-derived key", func() {
		good, err := DeriveHMACKey([]byte("right"))
		Expect(err).NotTo(HaveOccurred())

		bad, err := DeriveHMACKey([]byte("wrong"))
		Expect(err).NotTo(HaveOccurred())

		blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), good)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = OpenEnvelope(blob, KindUnsignedTransfer, bad)
		Expect(errors.Is(err, ErrHMACMismatch)).To(BeTrue())
	})

	It("requires HMAC key when envelope carries HMAC", func() {
		key, err := DeriveHMACKey([]byte("k"))
		Expect(err).NotTo(HaveOccurred())

		blob, err := SealPayload(KindUnsignedTransfer, []byte("x"), key)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = OpenEnvelope(blob, KindUnsignedTransfer, nil)
		Expect(errors.Is(err, ErrHMACRequired)).To(BeTrue())
	})

	It("rejects malformed envelopes", func() {
		_, _, err := OpenEnvelope([]byte("not json"), KindUnsignedTransfer, nil)
		Expect(err).To(HaveOccurred())

		bad := []byte(`{"monujo":"other","kind":"unsigned_transfer","version":1,"payload_b64":"YQ=="}`)
		_, _, err = OpenEnvelope(bad, KindUnsignedTransfer, nil)
		Expect(err).To(HaveOccurred())
	})

	It("rejects empty passphrase when deriving HMAC key", func() {
		_, err := DeriveHMACKey(nil)
		Expect(err).To(HaveOccurred())
	})

	It("derives identical HMAC keys for normalization-equivalent passphrases", func() {
		nfd, err := DeriveHMACKey([]byte("cafe\u0301"))
		Expect(err).NotTo(HaveOccurred())

		nfc, err := DeriveHMACKey([]byte("caf\u00e9"))
		Expect(err).NotTo(HaveOccurred())
		Expect(nfd).To(Equal(nfc))

		lig, err := DeriveHMACKey([]byte("\uFB01ne"))
		Expect(err).NotTo(HaveOccurred())

		plain, err := DeriveHMACKey([]byte("fine"))
		Expect(err).NotTo(HaveOccurred())
		Expect(lig).To(Equal(plain))
	})

	It("opens envelope across normalization-equivalent passphrases", func() {
		sealKey, err := DeriveHMACKey([]byte("cafe\u0301-offline"))
		Expect(err).NotTo(HaveOccurred())

		openKey, err := DeriveHMACKey([]byte("caf\u00e9-offline"))
		Expect(err).NotTo(HaveOccurred())

		payload := []byte("hello")
		env, err := SealPayload(KindUnsignedTransfer, payload, sealKey)
		Expect(err).NotTo(HaveOccurred())

		got, _, err := OpenEnvelope(env, KindUnsignedTransfer, openKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.Equal(got, payload)).To(BeTrue())
	})
})
