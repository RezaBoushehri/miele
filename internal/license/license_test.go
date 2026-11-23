package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	l := License{Serial: "ab12-CD34", Customer: "Acme", Issued: time.Now().UTC().Truncate(24 * time.Hour)}
	data, err := l.Sign(priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data, pub)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.CheckSerial("AB12-cd34", time.Now()); err != nil {
		t.Fatalf("serial should match case-insensitively: %v", err)
	}
	if err := got.CheckSerial("other", time.Now()); err == nil {
		t.Fatal("wrong serial should fail")
	}
	// tamper
	data[len(data)-1] ^= 1
	if _, err := Parse(data, pub); err == nil {
		t.Fatal("tampered license should fail")
	}
	// wrong key
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	data2, _ := l.Sign(priv)
	if _, err := Parse(data2, pub2); err == nil {
		t.Fatal("wrong public key should fail")
	}
}

func TestExpiry(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	l := License{Serial: "s", Customer: "c", Issued: time.Now().UTC(),
		Expiry: time.Now().Add(-time.Hour)}
	data, _ := l.Sign(priv)
	got, _ := Parse(data, pub)
	if err := got.CheckSerial("s", time.Now()); err == nil {
		t.Fatal("expired license should fail")
	}
}
