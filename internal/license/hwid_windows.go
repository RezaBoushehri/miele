//go:build windows

package license

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	driveRemovable            = 2
	ioctlStorageQueryProperty = 0x002d1400
	storageDeviceProperty     = 0
	propertyStandardQuery     = 0
)

// RemovableDrives returns the letters (e.g. "E:") of plugged-in removable drives.
func RemovableDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := fmt.Sprintf("%c:\\", 'A'+i)
		p, _ := windows.UTF16PtrFromString(root)
		if windows.GetDriveType(p) == driveRemovable {
			out = append(out, fmt.Sprintf("%c:", 'A'+i))
		}
	}
	return out
}

// DriveSerial returns a stable hardware serial for the removable drive ("E:").
// It prefers the physical device serial and falls back to the volume serial.
func DriveSerial(letter string) (string, error) {
	letter = strings.TrimRight(letter, "\\")
	if s := deviceSerial(letter); s != "" {
		return s, nil
	}
	if s := volumeSerial(letter); s != "" {
		return s, nil
	}
	return "", fmt.Errorf("license: could not read serial of drive %s", letter)
}

func deviceSerial(letter string) string {
	path := `\\.\` + letter
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	// STORAGE_PROPERTY_QUERY{ PropertyId, QueryType, AdditionalParameters[1] }
	query := make([]byte, 12)
	putU32(query[0:], storageDeviceProperty)
	putU32(query[4:], propertyStandardQuery)

	buf := make([]byte, 1024)
	var ret uint32
	err = windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		&query[0], uint32(len(query)), &buf[0], uint32(len(buf)), &ret, nil)
	if err != nil || ret < 28 {
		return ""
	}
	serialOffset := getU32(buf[24:]) // STORAGE_DEVICE_DESCRIPTOR.SerialNumberOffset
	if serialOffset == 0 || int(serialOffset) >= len(buf) {
		return ""
	}
	return strings.TrimSpace(cstr(buf[serialOffset:]))
}

func volumeSerial(letter string) string {
	root := letter + `\`
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return ""
	}
	var serial, maxLen, flags uint32
	err = windows.GetVolumeInformation(p, nil, 0, &serial, &maxLen, &flags, nil, 0)
	if err != nil || serial == 0 {
		return ""
	}
	return fmt.Sprintf("VOL-%08X", serial)
}

func putU32(b []byte, v uint32) { *(*uint32)(unsafe.Pointer(&b[0])) = v }
func getU32(b []byte) uint32    { return *(*uint32)(unsafe.Pointer(&b[0])) }

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
