package main

// specialItem is a shell object that does more than launch: it may add menu
// entries of its own and redraw its shell icon when its state changes, as the
// Recycle Bin's empty and full icons.
type specialItem struct {
	Match       func(path string) bool
	Menu        func(v *view, item *AppItem) []menuEntry
	RefreshIcon func(item *AppItem) ([]byte, bool)
}

// specialItemFor returns the special behavior for a path, if it has one.
func specialItemFor(path string) *specialItem {
	for i := range specialItems {
		if specialItems[i].Match != nil && specialItems[i].Match(path) {
			return &specialItems[i]
		}
	}
	return nil
}
