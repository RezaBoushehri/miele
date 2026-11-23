//go:build !windows

package license

import "errors"

// RemovableDrives is a stub on non-Windows builds (the app ships for Windows only).
func RemovableDrives() []string { return nil }

// DriveSerial is a stub on non-Windows builds.
func DriveSerial(letter string) (string, error) {
	return "", errors.New("license: drive serial is only available on Windows")
}
