package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// internetShortcutInfo is what a .url (Internet Shortcut) file names.
type internetShortcutInfo struct {
	URL string
}

// isURLFile reports whether path is a Windows Internet Shortcut, the text file
// Explorer saves for a page on the desktop.
func isURLFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".url")
}

// resolveInternetShortcut reads the URL an Internet Shortcut names. The file is
// an INI Explorer writes, usually just "[InternetShortcut]" and "URL=...".
func resolveInternetShortcut(path string) (internetShortcutInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return internetShortcutInfo{}, err
	}
	if len(data) > 1<<20 {
		return internetShortcutInfo{}, errors.New("internet shortcut is too large")
	}
	inSection := false
	for _, raw := range strings.Split(strings.ReplaceAll(decodeIniText(data), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			inSection = end > 0 && strings.EqualFold(strings.TrimSpace(line[1:end]), "InternetShortcut")
			continue
		}
		eq := strings.IndexByte(line, '=')
		if !inSection || eq <= 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(line[:eq]), "URL") {
			continue
		}
		url := strings.Trim(strings.TrimSpace(line[eq+1:]), `"`)
		if url == "" {
			break
		}
		return internetShortcutInfo{URL: url}, nil
	}
	return internetShortcutInfo{}, errors.New("no URL in the internet shortcut")
}
