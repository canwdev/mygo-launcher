package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeURLFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveInternetShortcut 验证 .url 文件里的 URL 被读出来，UTF-16LE 带 BOM
// 也能读；键和分节名不区分大小写，别的分节里的 URL 不算。
func TestResolveInternetShortcut(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		data []byte
	}{
		{"plain.url", []byte("[InternetShortcut]\r\nURL=https://192.168.0.100:4790/\r\n")},
		{"utf16.url", utf16LEBytes("[InternetShortcut]\r\nURL=https://192.168.0.100:4790/\r\n")},
		{"mixed.url", []byte("[internetshortcut]\nurl = \"https://192.168.0.100:4790/\"\nIconFile=C:\\x.ico\n")},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.name)
		writeURLFile(t, path, tc.data)
		us, err := resolveInternetShortcut(path)
		if err != nil {
			t.Fatalf("%s: resolve: %v", tc.name, err)
		}
		if us.URL != "https://192.168.0.100:4790/" {
			t.Fatalf("%s: URL = %q", tc.name, us.URL)
		}
	}

	other := filepath.Join(dir, "other.url")
	writeURLFile(t, other, []byte("[Other]\nURL=https://example.com/\n"))
	if _, err := resolveInternetShortcut(other); err == nil {
		t.Fatal("a URL outside [InternetShortcut] must not count")
	}
	missing := filepath.Join(dir, "missing.url")
	writeURLFile(t, missing, []byte("[InternetShortcut]\nIconFile=x.ico\n"))
	if _, err := resolveInternetShortcut(missing); err == nil {
		t.Fatal("a file without URL must fail")
	}
}

// TestBuildItemParsesURLFile 验证添加 .url 条目时用文件名作名称、用文件里的 URL
// 作路径，没有工作目录，并记住 .url 文件本身以便更新图标。
func TestBuildItemParsesURLFile(t *testing.T) {
	newTestApp(t)
	path := filepath.Join(absBase, "Sunshine.url")
	writeURLFile(t, path, []byte("[InternetShortcut]\r\nURL=https://192.168.0.100:4790/\r\n"))

	item := buildItem(path, false, false)
	if item == nil {
		t.Fatal("the internet shortcut must become an item")
	}
	if item.Name != "Sunshine" {
		t.Fatalf("Name = %q, want Sunshine", item.Name)
	}
	if item.Path != "https://192.168.0.100:4790/" {
		t.Fatalf("Path = %q, want the URL", item.Path)
	}
	if item.WorkingDir != "" || item.Args != "" {
		t.Fatalf("a URL item has no working directory or arguments, got %q %q", item.WorkingDir, item.Args)
	}
	if want := toStoredPath(path); item.Link != want {
		t.Fatalf("Link = %q, want %q", item.Link, want)
	}
	if timeableItem(item) {
		t.Fatal("a URL item is not timed")
	}
}

// TestAddPathsParsesURLFile 验证通过「选择文件」或拖入添加 .url 时，页面里出现
// 的是打开该网址的条目，而不是 .url 文件本身。
func TestAddPathsParsesURLFile(t *testing.T) {
	app := newTestApp(t)
	path := filepath.Join(absBase, "Sunshine.url")
	writeURLFile(t, path, []byte("[InternetShortcut]\r\nURL=https://192.168.0.100:4790/\r\n"))

	tab := &app.store.Categories[0]
	app.addPaths([]string{path}, tab.GUID)
	if len(tab.Slots) != 1 {
		t.Fatalf("the page must hold one item, got %d", len(tab.Slots))
	}
	item := app.store.Apps[*tab.Slots[0]]
	if item.Path != "https://192.168.0.100:4790/" {
		t.Fatalf("the item must open the URL, got %q", item.Path)
	}
}
