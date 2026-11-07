//go:build windows

package main

import "strings"

const recycleBinCLSID = `::{645FF040-5081-101B-9F08-00AA002F954E}`

func isRecycleBin(path string) bool {
	return strings.EqualFold(path, recycleBinCLSID) ||
		strings.EqualFold(path, "shell:RecycleBinFolder")
}

// specialItems are the shell objects with behavior of their own. The Recycle
// Bin redraws its icon as its contents change, and can be emptied from its
// menu.
var specialItems []specialItem

func init() {
	specialItems = []specialItem{
		{
			Match: isRecycleBin,
			Menu: func(v *view, item *AppItem) []menuEntry {
				count, ok := recycleBinItems()
				if !ok || count == 0 {
					return nil
				}
				msgs := v.msgs()
				return []menuEntry{{icon: iconTrash, label: msgs.EmptyRecycleBin(), action: func() {
					v.later(func() {
						v.ask(msgs.EmptyRecycleBinConfirm(), func() { v.app.emptyRecycleBin(item) })
					})
				}}}
			},
			RefreshIcon: recycleBinIcon,
		},
	}
}
