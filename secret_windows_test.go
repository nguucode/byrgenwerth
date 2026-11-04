//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAPIKeyRoundTripsEncrypted(t *testing.T) {
	dir := t.TempDir()
	if LoadAPIKey(dir) != "" {
		t.Fatal("no file must read as no key")
	}
	key := "sk-ant-test-value"
	if err := SaveAPIKey(dir, key); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, apiKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(key)) {
		t.Fatal("key is on disk in plain text")
	}
	if got := LoadAPIKey(dir); got != key {
		t.Fatalf("got %q", got)
	}
	if err := SaveAPIKey(dir, ""); err != nil {
		t.Fatal(err)
	}
	if LoadAPIKey(dir) != "" {
		t.Fatal("empty key must delete the file")
	}
}
