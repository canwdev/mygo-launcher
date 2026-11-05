package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeLegacyStore 造一份旧版数据：数据在 .go-launcher-data 里，图标路径也指那儿。
func writeLegacyStore(t *testing.T, store AppStore) {
	t.Helper()
	legacy := filepath.Join(absBase, legacyDataDir)
	if err := os.MkdirAll(filepath.Join(legacy, "cached-icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacy, filepath.Base(saveFile))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "cached-icons", "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func legacyStore() AppStore {
	store := defaultStore()
	store.Apps["a"] = &AppItem{
		GUID: "a", Name: "Alpha", Path: "C:/tools/alpha.exe",
		Icon: legacyDataDir + "/cached-icons/a.png",
	}
	store.Categories[0].Slots = []*string{ptrString("a")}
	return store
}

// TestPrepareDataDirMovesTheOldFolder 验证 0.2.0 改数据文件夹名后，旧数据跟着搬过来，
// 缓存图标和它们记录的路径都仍然可用。
func TestPrepareDataDirMovesTheOldFolder(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	writeLegacyStore(t, legacyStore())

	prepareDataDir()

	legacy := filepath.Join(absBase, legacyDataDir)
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("the old folder must be gone after the move, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(absBase, iconsDir, "a.png")); err != nil {
		t.Fatalf("the cached icons must move with the store: %v", err)
	}

	store := loadStore()
	item := store.Apps["a"]
	if item == nil {
		t.Fatal("the moved store must keep its items")
	}
	want := dataDir + "/cached-icons/a.png"
	if item.Icon != want {
		t.Fatalf("icon path = %q, want %q", item.Icon, want)
	}
	if !isAutoIcon(item.Icon) {
		t.Fatalf("%q must still count as an icon the launcher extracted", item.Icon)
	}
}

// TestPrepareDataDirKeepsAnExistingFolder 验证新文件夹已经在时不动它，也不动旧文件夹：
// 迁移只做一次。
func TestPrepareDataDirKeepsAnExistingFolder(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	writeLegacyStore(t, legacyStore())
	if err := os.MkdirAll(filepath.Join(absBase, dataDir), 0o755); err != nil {
		t.Fatal(err)
	}

	prepareDataDir()

	if _, err := os.Stat(filepath.Join(absBase, legacyDataDir)); err != nil {
		t.Fatalf("the old folder must be left alone: %v", err)
	}
}

// TestPrepareDataDirCreatesTheFolder 验证首次运行：只为新文件夹做准备。
func TestPrepareDataDirCreatesTheFolder(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	prepareDataDir()

	if info, err := os.Stat(filepath.Join(absBase, dataDir)); err != nil || !info.IsDir() {
		t.Fatalf("the data folder must exist after startup: %v", err)
	}
}

// TestParseStoreMigratesIconPaths 验证手改的数据文件里写的老图标路径也会被改到新文件夹：
// 迁移后用户可能还留着旧路径。
func TestParseStoreMigratesIconPaths(t *testing.T) {
	store, ok := parseStore([]byte(`{
		"apps": {"a": {"guid": "a", "name": "A", "path": "C:/a.exe", "icon": ".go-launcher-data\\cached-icons\\a.png"}},
		"categories": [{"guid": "c", "name": "Main", "slots": ["a"]}],
		"settings": {}
	}`))
	if !ok {
		t.Fatal("parseStore rejected valid JSON")
	}
	want := dataDir + `\cached-icons\a.png`
	if got := store.Apps["a"].Icon; got != want {
		t.Fatalf("icon = %q, want %q", got, want)
	}
	if !isAutoIcon(store.Apps["a"].Icon) {
		t.Fatalf("%q must be recognized as a launcher icon", store.Apps["a"].Icon)
	}
}

// TestMigrateStoreFileRewritesTheOldFolder 验证数据文件本身也被改写到新文件夹：
// 只在内存里改的话，下次启动还得再改一遍，用户手动看文件也会困惑。
func TestMigrateStoreFileRewritesTheOldFolder(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	writeLegacyStore(t, legacyStore())
	prepareDataDir()

	data, err := os.ReadFile(filepath.Join(absBase, saveFile))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(legacyDataDir)) {
		t.Fatalf("the store file still names the old folder:/n%s", data)
	}
	if !bytes.Contains(data, []byte(dataDir+"/cached-icons/a.png")) {
		t.Fatalf("the store file must name the new folder:/n%s", data)
	}
}
