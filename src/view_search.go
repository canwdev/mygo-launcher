package main

import (
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// searchHit is an item the search found, with the tab it is in.
type searchHit struct {
	guid    string
	name    string
	path    string
	tabGUID string
	tabName string
}

func (v *view) openSearch() {
	v.searchOpen = true
	v.searchQuery = ""
	v.searchChosen = 0
	v.searchList.ScrollTo(0, ui.Start)
	v.refresh()
}

// searchHits lists every item of every tab, ranked for the query: a name that
// matches exactly first, then a path that does, then those starting with it, and
// finally those merely containing it.
func (v *view) searchHits() []searchHit {
	q := strings.ToLower(strings.TrimSpace(v.searchQuery))
	if q == "" {
		return nil
	}
	var hits []searchHit
	for _, cat := range v.app.store.Categories {
		for _, slot := range cat.Slots {
			if slot == nil {
				continue
			}
			item := v.app.item(*slot)
			if item == nil {
				continue
			}
			name, path := strings.ToLower(item.Name), strings.ToLower(item.Path)
			if !strings.Contains(name, q) && !strings.Contains(path, q) {
				continue
			}
			hits = append(hits, searchHit{
				guid: item.GUID, name: item.Name, path: item.Path,
				tabGUID: cat.GUID, tabName: cat.Name,
			})
		}
	}
	rank := func(h searchHit) int {
		name, path := strings.ToLower(h.name), strings.ToLower(h.path)
		switch {
		case name == q:
			return 0
		case path == q:
			return 1
		case strings.HasPrefix(name, q):
			return 2
		case strings.HasPrefix(path, q):
			return 3
		default:
			return 4
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return rank(hits[a]) < rank(hits[b]) })
	return hits
}

// search is the search overlay: type to filter, Up and Down to choose, Enter to
// run the item, and the crosshair to jump to it in its tab.
func (v *view) search(c *ui.Context) {
	if !v.searchOpen {
		return
	}
	p := v.pal
	m := v.msgs()
	hits := v.searchHits()
	if v.searchChosen >= len(hits) {
		v.searchChosen = max(0, len(hits)-1)
	}

	ui.Modal(c, &v.searchOpen, func() {
		ui.Column(c).Width(560).Gap(10).Children(func() {
			field := ui.Row(c).Height(40).Gap(8).AlignItems(ui.Center)
			field.Children(func() {
				ui.Icon(c, iconSearch).Size(16, 16).TextColor(p.muted)
				ui.TextInput(c, &v.searchQuery).Bind(&v.searchInput).AutoFocus().
					Placeholder(m.SearchPrograms()).FontSize(14).Grow(1).
					OnTextKey(func(k ui.TextKey) bool {
						switch k.Key {
						case ui.KeyDown:
							if len(hits) > 0 {
								v.searchChosen = min(v.searchChosen+1, len(hits)-1)
								v.searchList.ScrollIntoView(v.searchChosen)
							}
							return true
						case ui.KeyUp:
							v.searchChosen = max(v.searchChosen-1, 0)
							if len(hits) > 0 {
								v.searchList.ScrollIntoView(v.searchChosen)
							}
							return true
						case ui.KeyEnter:
							v.launchHit(hits, v.searchChosen)
							return true
						}
						return false
					})
			})

			v.searchList.Selected = &v.searchChosen
			v.searchList.Key = func(i int) any { return hits[i].guid }
			v.searchList.Label = func(i int) string { return hits[i].name }
			// The list is as tall as its rows, up to a screenful: a List's
			// viewport must be told, or the rows below it overlap.
			height := float32(72)
			if len(hits) > 0 {
				height = float32(min(320, len(hits)*40))
			}
			ui.List(c, &v.searchList, len(hits), func(i int) {
				v.searchRow(c, hits[i], i)
			}).Height(height).Children(func() {
				msg := m.NoResults()
				if strings.TrimSpace(v.searchQuery) == "" {
					msg = m.TypeToSearch()
				}
				ui.Text(c, msg).FontSize(12).TextColor(p.muted).Padding(12)
			})

			ui.Text(c, m.SearchHelp()).
				FontSize(11).TextColor(p.faint)
		})
	})
}

// searchRow is one result: the item runs when its row is clicked, and the
// crosshair on the right brings it into view in its tab.
func (v *view) searchRow(c *ui.Context, hit searchHit, index int) {
	p := v.pal
	chosen := index == v.searchChosen
	row := ui.Row(c).Height(40).Padding(0, 10).Gap(10).Radius(8).AlignItems(ui.Center)
	if chosen {
		row.Background(p.accent)
	}
	textColor, muted := p.text, p.muted
	if chosen {
		textColor, muted = p.onAccent, p.onAccent.Alpha(0.75)
	}
	if row.Clicked() {
		v.launchHit(v.searchHits(), index)
	}
	row.Children(func() {
		if item := v.app.item(hit.guid); item != nil {
			v.itemIcon(c, item, 18)
		}
		ui.Text(c, hit.name).SingleLine().FontSize(14).TextColor(textColor).Grow(1)
		where := hit.path
		if where == "" {
			where = hit.tabName
		}
		ui.Text(c, where).SingleLine().FontSize(11).TextColor(muted).MaxWidthPercent(35)
		m := v.msgs()
		locate := iconButton(c, iconCross, 13, textColor, m.LocateInTab())
		locate.Tooltip(m.LocateIn(hit.tabName))
		locate.OnClick(func() { v.locateItem(hit.guid, hit.tabGUID) })
		locate.Hidden(!chosen && !row.Hovered())
	})
}

// launchHit runs a search result and closes the overlay.
func (v *view) launchHit(hits []searchHit, index int) {
	if index < 0 || index >= len(hits) {
		return
	}
	hit := hits[index]
	v.searchOpen = false
	v.runItem(hit.guid)
	v.refresh()
}

// locateItem shows an item in its tab, scrolled into view and highlighted for a
// moment.
func (v *view) locateItem(guid, tabGUID string) {
	v.searchOpen = false
	v.setActiveTab(tabGUID)
	v.locate = guid
	v.locateTill = time.Now().Add(1500 * time.Millisecond)
	v.later(func() { v.scrollTo(guid) })
	v.app.refresh()
	go func() {
		time.Sleep(1500 * time.Millisecond)
		v.app.dispatch(func() {
			if time.Now().After(v.locateTill) {
				v.locate = ""
			}
		})
	}()
}

// scrollTo brings an item's row, or cell, into the middle of the view.
func (v *view) scrollTo(guid string) {
	tab := v.tab()
	if tab == nil {
		return
	}
	if v.prefs.ViewMode == "grid" {
		for i, slot := range tab.Slots {
			if slot != nil && *slot == guid {
				v.grid.ScrollTo(i, ui.Center)
				return
			}
		}
		return
	}
	for i, row := range v.rows(tab) {
		if row.item.GUID == guid {
			v.list.ScrollTo(i, ui.Center)
			return
		}
	}
}
