package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"go-launcher/locales"
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

// formatRuntime spells accumulated milliseconds shortly, as the rows show them
// in lang: "3d 2h 5m", "--" for nothing yet and "1m" for anything under a
// minute.
func formatRuntime(lang string, ms int64) string {
	msg := locales.In(lang)
	total := ms / 1000
	d := total / 86400
	h := (total % 86400) / 3600
	m := (total % 3600) / 60
	s := total % 60
	parts := make([]string, 0, 3)
	if d > 0 {
		parts = append(parts, msg.DurationDay(strconv.FormatInt(d, 10)))
	}
	if h > 0 {
		parts = append(parts, msg.DurationHour(strconv.FormatInt(h, 10)))
	}
	if m > 0 {
		parts = append(parts, msg.DurationMinute(strconv.FormatInt(m, 10)))
	}
	if len(parts) == 0 {
		if s > 0 {
			parts = append(parts, msg.DurationMinute("1"))
		} else {
			parts = append(parts, msg.DurationNone())
		}
	}
	return strings.Join(parts, " ")
}

// fileExists reports whether path names something on disk.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".bat" || ext == ".cmd" || ext == ".com"
	}
	return info.Mode()&0111 != 0
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
