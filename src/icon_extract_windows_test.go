//go:build windows

package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestExtractIconResource 验证能从 exe 的图标资源取出 PNG，并保持尺寸。
func TestExtractIconResource(t *testing.T) {
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no %s: %v", path, err)
	}
	data := extractIconResource(path, 64, 0)
	if len(data) == 0 {
		t.Fatal("the program's icon must be drawn")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the icon must be a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("the icon must have a picture, got %v", b)
	}
	if img.Bounds().Dx() != 64 {
		t.Fatalf("the icon must be drawn at the size asked, got %v", img.Bounds())
	}
}

// TestExtractIconResourcePicksIndex 验证序号选中 DLL 里的另一个图标，越界时没有图标。
func TestExtractIconResourcePicksIndex(t *testing.T) {
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", "shell32.dll")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no %s: %v", path, err)
	}
	if data := extractIconResource(path, 64, 1); len(data) == 0 {
		t.Fatal("the second icon of shell32.dll must be drawn")
	}
	if data := extractIconResource(path, 64, 9999); data != nil {
		t.Fatalf("an index past the icons must give none, got %d bytes", len(data))
	}
}

// TestExtractIconResourceWithoutIcons 验证没有图标的文件返回空，而不是崩溃。
func TestExtractIconResourceWithoutIcons(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(path, []byte("no icons"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data := extractIconResource(path, 64, 0); data != nil {
		t.Fatalf("a file without icons must give none, got %d bytes", len(data))
	}
}

// TestIconImageReportsImages 验证图标字段把图片当图片，其余文件去提取图标。
func TestIconImageReportsImages(t *testing.T) {
	for _, path := range []string{"a.png", "a.JPG", `C:\icons\a.svg`, "a.webp"} {
		if !isIconImage(path) {
			t.Errorf("%q must be an image", path)
		}
	}
	for _, path := range []string{"a.ico", "a.exe", "a.dll", ""} {
		if isIconImage(path) {
			t.Errorf("%q must be extracted from", path)
		}
	}
}
