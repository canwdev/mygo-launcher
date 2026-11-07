package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDataDirMovesLegacyFolder(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "install", dataDir)
	target := filepath.Join(root, "local", dataDir)
	writeTestFile(t, filepath.Join(legacy, "go-launcher-data.json"), "store")
	writeTestFile(t, filepath.Join(legacy, "custom-icons", "a.png"), "icon")

	if !migrateDataDir(legacy, target) {
		t.Fatal("migrateDataDir reported failure")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy folder still exists: %v", err)
	}
	if got := readTestFile(t, filepath.Join(target, "go-launcher-data.json")); got != "store" {
		t.Errorf("store = %q, want %q", got, "store")
	}
	if got := readTestFile(t, filepath.Join(target, "custom-icons", "a.png")); got != "icon" {
		t.Errorf("icon = %q, want %q", got, "icon")
	}
}

func TestMigrateDataDirKeepsExistingTarget(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "install", dataDir)
	target := filepath.Join(root, "local", dataDir)
	writeTestFile(t, filepath.Join(legacy, "go-launcher-data.json"), "old")
	writeTestFile(t, filepath.Join(target, "go-launcher-data.json"), "new")

	if !migrateDataDir(legacy, target) {
		t.Fatal("migrateDataDir reported failure")
	}
	if got := readTestFile(t, filepath.Join(target, "go-launcher-data.json")); got != "new" {
		t.Errorf("store = %q, want the existing %q", got, "new")
	}
}

func TestMigrateDataDirFirstRun(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "local", dataDir)

	if !migrateDataDir(filepath.Join(root, "install", dataDir), target) {
		t.Fatal("migrateDataDir reported failure")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("first run created %s: %v", target, err)
	}
}

func TestRebasePath(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "install", dataDir)
	to := filepath.Join(root, "local", dataDir)

	got, ok := rebasePath(filepath.Join(from, "custom-icons", "a.png"), from, to)
	if !ok || got != filepath.Join(to, "custom-icons", "a.png") {
		t.Errorf("rebasePath = %q, %v", got, ok)
	}
	if _, ok := rebasePath(filepath.Join(from, "..", "other.png"), from, to); ok {
		t.Error("rebasePath moved a path outside the old folder")
	}
	if _, ok := rebasePath(filepath.Join("custom-icons", "a.png"), from, to); ok {
		t.Error("rebasePath moved a relative path")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
