package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func sanitizeBase(name string) string {
	base := strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		return r
	}, name)
	if len(base) > 40 {
		base = base[:40]
	}
	return base
}

// expandPath fills in %VAR% names, the form Windows paths and the shell's icon
// picker use ("%SystemRoot%\System32\shell32.dll"); a name without a value is
// left as it was.
func expandPath(path string) string {
	if !strings.Contains(path, "%") {
		return path
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(path, '%')
		if i < 0 {
			b.WriteString(path)
			break
		}
		j := strings.IndexByte(path[i+1:], '%')
		if j < 0 {
			b.WriteString(path)
			break
		}
		j += i + 1
		b.WriteString(path[:i])
		if value, ok := os.LookupEnv(path[i+1 : j]); ok {
			b.WriteString(value)
		} else {
			b.WriteString(path[i : j+1])
		}
		path = path[j+1:]
	}
	return b.String()
}

// isShellTarget reports whether path names something the shell opens instead of
// a file: a URL ("https://…"), a settings or shell URI ("ms-settings:colors",
// "shell:AppsFolder"), or a shell namespace path ("::{CLSID}"). A drive letter
// ("C:\…") is a file path, not a scheme. Items migrated from CLaunch carry
// these, and they must not be joined to the data directory or rewritten.
func isShellTarget(path string) bool {
	if path == "" {
		return false
	}
	if strings.HasPrefix(path, "::") {
		return true
	}
	colon := strings.IndexByte(path, ':')
	if colon <= 0 {
		return false
	}
	if colon == 1 {
		// A one-letter scheme is a drive: "C:\tools\x.exe" and "C:x" are paths.
		return false
	}
	for _, r := range path[:colon] {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '+' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// isShellNamespace reports whether path names a shell folder, as "::{CLSID}"
// or "shell:...", rather than a file or a URL. The shell draws its icon.
func isShellNamespace(path string) bool {
	return strings.HasPrefix(path, "::") || strings.HasPrefix(strings.ToLower(path), "shell:")
}

// isPackagedApp reports whether path names a packaged (UWP) app, which has no
// file system properties or shell context menu of the kinds the launcher
// offers.
func isPackagedApp(path string) bool {
	return strings.HasPrefix(strings.ToLower(path), `shell:appsfolder\`)
}

func normalizePath(path string) string {
	if runtime.GOOS != "windows" || isShellTarget(path) {
		return path
	}
	p := strings.ReplaceAll(path, "/", `\`)
	if len(p) >= 3 && p[0] == '\\' && p[2] == ':' &&
		((p[1] >= 'A' && p[1] <= 'Z') || (p[1] >= 'a' && p[1] <= 'z')) {
		p = p[1:]
	}
	return p
}

// shortcutInfo holds the parsed contents of a Windows .lnk shortcut.
type shortcutInfo struct {
	Target  string // resolved program/file path
	Args    string // launch arguments as stored in the shortcut
	WorkDir string // startup ("start in") folder
}

func isShortcutFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".lnk")
}

func defaultTitle(path string) string {
	base := filepath.Base(path)
	// filepath.Ext 会把以点开头的隐藏文件/目录的整个 basename 当作扩展名返回
	// （如 ".pnpm-store" → Ext 返回 ".pnpm-store"），直接 TrimSuffix 会把名字剥成空串。
	// 只在扩展名的点位于 basename 内部（index > 0）时才剥除，保留隐藏名本身。
	if idx := strings.LastIndex(base, "."); idx > 0 {
		return base[:idx]
	}
	return base
}

// formatRuntime spells accumulated milliseconds shortly, as the rows show them:
// "3d2h5m". The units stay untranslated on purpose: the localized ones are too
// long for a cell. Nothing yet is "", and anything under a minute is "1m".
func formatRuntime(ms int64) string {
	total := ms / 1000
	d := total / 86400
	h := (total % 86400) / 3600
	m := (total % 3600) / 60
	s := total % 60
	var b strings.Builder
	if d > 0 {
		b.WriteString(strconv.FormatInt(d, 10))
		b.WriteByte('d')
	}
	if h > 0 {
		b.WriteString(strconv.FormatInt(h, 10))
		b.WriteByte('h')
	}
	if m > 0 {
		b.WriteString(strconv.FormatInt(m, 10))
		b.WriteByte('m')
	}
	if b.Len() == 0 {
		if s > 0 {
			return "1m"
		}
		return ""
	}
	return b.String()
}

// fileExists reports whether path names something on disk.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// executableExts are the extensions Windows starts as a program: launching one
// is a tracked process, so its item has a runtime.
var executableExts = map[string]bool{".exe": true, ".bat": true, ".cmd": true, ".com": true}

// executableExt reports whether path has the extension of a program. It reads
// no disk, so a program that is missing for now still counts as one.
func executableExt(path string) bool {
	return executableExts[strings.ToLower(filepath.Ext(path))]
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return executableExt(path)
	}
	return info.Mode()&0111 != 0
}

// timeableItem reports whether an item has a runtime: only a program started as
// a process is tracked, so only its cell shows a timer. A folder, a file, a URL
// or a shell target is opened by the shell, and an item run elevated starts
// untracked, so none of them counts time. It reads the path, not the disk, so an
// item whose file is missing for now keeps its timer.
func timeableItem(item *AppItem) bool {
	return item != nil && item.Path != "" && !isShellTarget(item.Path) && executableExt(item.Path)
}

func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}
