// Package license implements the flash-bound licensing used to lock the app.
//
// A license binds the app to one USB flash drive: it carries that drive's hardware
// serial, a customer name and an optional expiry, all signed with the owner's Ed25519
// private key (held only by the keygen tool). The app embeds the matching public key
// and, at start-up, refuses to run unless a plugged-in drive carries a license whose
// signature is valid and whose serial matches that drive's real hardware serial.
//
// This raises the bar against casual copying, but — like any client-side scheme — it is
// not unbreakable. See docs/SECURITY.md.
package license

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// FileName is the license file expected on the flash drive.
const FileName = "miele.lic"

const magic = "MLC1"

// License is the signed grant stored on the flash drive.
type License struct {
	Serial   string    // hardware serial of the flash drive
	Customer string    // who the licence is issued to
	Issued   time.Time // issue date (UTC, day precision)
	Expiry   time.Time // zero = never expires
}

// signedBody is the canonical byte form that gets signed (everything but the signature).
func (l License) signedBody() []byte {
	var b []byte
	put := func(s string) {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
		b = append(b, s...)
	}
	b = append(b, magic...)
	put(strings.ToUpper(strings.TrimSpace(l.Serial)))
	put(l.Customer)
	b = binary.LittleEndian.AppendUint64(b, uint64(l.Issued.Unix()))
	exp := int64(0)
	if !l.Expiry.IsZero() {
		exp = l.Expiry.Unix()
	}
	b = binary.LittleEndian.AppendUint64(b, uint64(exp))
	return b
}

// Sign produces the license file bytes using the owner's private key.
func (l License) Sign(priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("license: invalid private key")
	}
	body := l.signedBody()
	sig := ed25519.Sign(priv, body)
	out := append([]byte(nil), body...)
	out = append(out, sig...)
	return out, nil
}

// Parse decodes and verifies a license file against the public key.
func Parse(data []byte, pub ed25519.PublicKey) (*License, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("license: app is not provisioned with a public key")
	}
	if len(data) < len(magic)+ed25519.SignatureSize {
		return nil, errors.New("license: file too short")
	}
	body := data[:len(data)-ed25519.SignatureSize]
	sig := data[len(data)-ed25519.SignatureSize:]
	r := &cur{b: body}
	if string(r.take(len(magic))) != magic {
		return nil, errors.New("license: bad magic")
	}
	l := &License{}
	l.Serial = string(r.str())
	l.Customer = string(r.str())
	l.Issued = time.Unix(int64(r.u64()), 0).UTC()
	if exp := int64(r.u64()); exp != 0 {
		l.Expiry = time.Unix(exp, 0).UTC()
	}
	if r.err != nil {
		return nil, fmt.Errorf("license: %w", r.err)
	}
	if !ed25519.Verify(pub, body, sig) {
		return nil, errors.New("license: signature does not match (file tampered or wrong key)")
	}
	return l, nil
}

// CheckSerial reports whether the license is valid right now for the given drive serial.
func (l *License) CheckSerial(drive string, now time.Time) error {
	if !strings.EqualFold(strings.TrimSpace(drive), strings.TrimSpace(l.Serial)) {
		return errors.New("license: this license is bound to a different flash drive")
	}
	if !l.Expiry.IsZero() && now.After(l.Expiry) {
		return fmt.Errorf("license: expired on %s", l.Expiry.Format("2006-01-02"))
	}
	return nil
}

// ParsePublicKey decodes a hex-encoded Ed25519 public key.
func ParsePublicKey(h string) (ed25519.PublicKey, error) {
	h = strings.TrimSpace(h)
	if h == "" {
		return nil, nil
	}
	raw, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("license: public key must be 32 bytes")
	}
	return ed25519.PublicKey(raw), nil
}

type cur struct {
	b   []byte
	err error
}

func (c *cur) take(n int) []byte {
	if c.err != nil || len(c.b) < n {
		c.err = errors.New("unexpected end of license")
		return nil
	}
	v := c.b[:n]
	c.b = c.b[n:]
	return v
}

func (c *cur) u32() uint32 {
	b := c.take(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func (c *cur) u64() uint64 {
	b := c.take(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func (c *cur) str() []byte { return c.take(int(c.u32())) }
