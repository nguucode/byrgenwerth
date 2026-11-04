//go:build windows

package main

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func blobOf(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func blobBytes(b windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)
}

// SaveAPIKey stores the key encrypted with DPAPI, so only this Windows account
// can read it back. An empty key deletes the file.
func SaveAPIKey(dir, key string) error {
	path := filepath.Join(dir, apiKeyFile)
	if key == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var out windows.DataBlob
	if err := windows.CryptProtectData(blobOf([]byte(key)), nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return err
	}
	return os.WriteFile(path, blobBytes(out), 0o600)
}

// LoadAPIKey returns "" when there is no key or it cannot be decrypted, such as
// a data folder copied from another account.
func LoadAPIKey(dir string) string {
	enc, err := os.ReadFile(filepath.Join(dir, apiKeyFile))
	if err != nil {
		return ""
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blobOf(enc), nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return ""
	}
	return string(blobBytes(out))
}
