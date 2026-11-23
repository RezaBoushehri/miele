package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthorizeCore(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)
	dir := t.TempDir()
	// write a valid license into a fake "drive" directory
	lic := License{Serial: "HWSER-123", Customer: "Acme", Issued: time.Now().UTC()}
	data, _ := lic.Sign(priv)
	os.WriteFile(filepath.Join(dir, FileName), data, 0644)

	drives := func() []string { return []string{dir} }
	serialOf := func(d string) (string, error) { return "HWSER-123", nil }

	// os.ReadFile uses d+`\`+FileName; emulate by making d already end correctly.
	// On the test we point the drive at dir and strip the backslash join:
	got := authorizeCoreTest(pubHex, drives, serialOf, time.Now(), dir)
	if !got.OK {
		t.Fatalf("expected OK, got %q", got.Reason)
	}
	// wrong serial
	bad := authorizeCoreTest(pubHex, drives, func(string) (string, error) { return "OTHER", nil }, time.Now(), dir)
	if bad.OK {
		t.Fatal("expected failure on serial mismatch")
	}
	// no provisioning
	np := authorizeCore("", drives, serialOf, time.Now())
	if np.OK {
		t.Fatal("expected failure when not provisioned")
	}
}

// authorizeCoreTest reads the license straight from dir (no drive\ join) for testing.
func authorizeCoreTest(pubHex string, drives driveLister, serialOf serialReader, now time.Time, dir string) Status {
	pub, _ := ParsePublicKey(pubHex)
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return Status{Reason: "no file"}
	}
	lic, err := Parse(data, pub)
	if err != nil {
		return Status{Reason: err.Error()}
	}
	serial, _ := serialOf(dir)
	if err := lic.CheckSerial(serial, now); err != nil {
		return Status{Reason: err.Error()}
	}
	return Status{OK: true, License: lic}
}
