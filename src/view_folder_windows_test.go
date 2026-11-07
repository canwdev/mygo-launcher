package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestReadFolderSkipsHiddenFiles 验证文件夹面板按 Windows 的隐藏属性跳过文件，
// 不只是按文件名。
func TestReadFolderSkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	shown := filepath.Join(dir, "shown.txt")
	secret := filepath.Join(dir, "secret.txt")
	for _, path := range []string{shown, secret} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := syscall.UTF16PtrFromString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatalf("make the file hidden: %v", err)
	}

	entries, err := readFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].name != "shown.txt" {
		t.Fatalf("readFolder = %+v, want only shown.txt", entries)
	}
}
