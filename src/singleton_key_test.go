package main

import (
	"encoding/hex"
	"testing"
)

func TestInstallKeyStableAndHex(t *testing.T) {
	key1 := installKey()
	key2 := installKey()
	if key1 != key2 {
		t.Fatalf("installKey() must be stable within a process: %q vs %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Fatalf("installKey() must return a 64-char SHA-256 hex, got %d chars", len(key1))
	}
	if _, err := hex.DecodeString(key1); err != nil {
		t.Fatalf("installKey() must be valid hex: %v", err)
	}
}
