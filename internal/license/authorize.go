package license

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Status is the result of an authorization check.
type Status struct {
	OK      bool
	License *License
	Drive   string // the drive letter that authorized the app
	Serial  string
	Reason  string // human-readable reason when OK is false
}

// driveLister and serialReader are injected so the core can be tested off-Windows.
type driveLister func() []string
type serialReader func(string) (string, error)

// authorizeCore scans the given drives for a valid license.
func authorizeCore(pubHex string, drives driveLister, serialOf serialReader, now time.Time) Status {
	pub, err := ParsePublicKey(pubHex)
	if err != nil || pub == nil {
		return Status{Reason: "این نسخه فاقد کلید مجوز است (برنامه provision نشده)."}
	}
	letters := drives()
	if len(letters) == 0 {
		return Status{Reason: "فلش مجوز پیدا نشد. لطفاً فلش را وصل کنید."}
	}
	lastReason := "فلش مجوز معتبر پیدا نشد."
	for _, d := range letters {
		data, err := os.ReadFile(filepath.Join(d+`\`, FileName))
		if err != nil {
			continue
		}
		lic, err := Parse(data, pub)
		if err != nil {
			lastReason = err.Error()
			continue
		}
		serial, err := serialOf(d)
		if err != nil {
			lastReason = err.Error()
			continue
		}
		if err := lic.CheckSerial(serial, now); err != nil {
			lastReason = err.Error()
			continue
		}
		return Status{OK: true, License: lic, Drive: d, Serial: serial}
	}
	return Status{Reason: lastReason}
}

// Authorize checks all removable drives for a valid license on this (Windows) machine.
func Authorize() Status {
	return authorizeCore(PublicKeyHex, RemovableDrives, DriveSerial, time.Now())
}

// ErrNoDrives is returned by the keygen when no removable drive is present.
var ErrNoDrives = errors.New("no removable drive found")
