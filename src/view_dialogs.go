package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// dialogs builds the window's dialogs; each shows while its flag is true.
func (v *view) dialogs(c *ui.Context) {
	v.itemDialog(c)
	v.runtimeDialog(c)
	v.renameDialog(c)
	v.confirmDialog(c)
}

// labeled stacks a small label over a field.
func (v *view) labeled(c *ui.Context, label string, fn func()) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(v.pal.muted)
		fn()
	})
}

// browseButton asks for something on a goroutine, since a system dialog blocks,
// and puts the answer in set.
func (v *view) browseButton(c *ui.Context, label string, pick func() string, set func(string)) {
	ui.Button(c, label).OnClick(func() {
		go func() {
			if picked := pick(); picked != "" {
				v.app.dispatch(func() {
					set(picked)
					v.refresh()
				})
			}
		}()
	})
}

// itemDialog creates an item or edits one.
func (v *view) itemDialog(c *ui.Context) {
	if !v.editOpen {
		return
	}
	p := v.pal
	m := v.msgs()
	title := m.EditItem()
	if v.editNew {
		title = m.CreateItem()
	}
	ui.Modal(c, &v.editOpen, func() {
		ui.Column(c).Width(480).Gap(12).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(17).FontWeight(700)
				if !v.editNew {
					// The guid identifies the item in the data file; it is
					// not editable, so it is a caption rather than a field.
					ui.Text(c, m.ItemID(v.editGUID)).FontSize(10.5).TextColor(p.faint).SingleLine()
				}
			})

			// The fields scroll in a small window, so the buttons below stay
			// reachable.
			ui.Scroll(c).MaxHeight(dialogBodyMax(c)).Children(func() {
				ui.Column(c).Gap(6).Padding(0, 2, 6, 2).Children(func() {
					v.itemDialogFields(c)
				})
			})

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c, m.Cancel()).OnClick(func() { v.editOpen = false; v.refresh() })
				ui.PrimaryButton(c, m.Save()).OnClick(v.saveItemDialog)
			})
		})
	})
}

// dialogBodyMax is how tall a dialog's scrolling body may be: what the window
// leaves once the title, the buttons and the margins are in.
func dialogBodyMax(c *ui.Context) float32 {
	_, height := c.Size()
	return max(140, height-220)
}

// itemDialogFields builds the editable fields of the item dialog.
func (v *view) itemDialogFields(c *ui.Context) {
	p := v.pal
	m := v.msgs()

	v.labeled(c, m.Name(), func() {
		if ui.TextInput(c, &v.editName).Placeholder(m.NameHint()).Submitted() {
			v.saveItemDialog()
		}
	})

	v.labeled(c, m.Path(), func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &v.editPath).Grow(1)
			v.browseButton(c, m.Browse(), func() string {
				return v.app.pickFile(dirname(v.editPath))
			}, func(p string) { v.editPath = p })
		})
	})

	v.labeled(c, m.Args(), func() {
		ui.TextInput(c, &v.editArgs).Placeholder(m.Optional())
	})

	v.labeled(c, m.WorkingDirectory(), func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &v.editWorkDir).Grow(1)
			v.browseButton(c, m.Browse(), func() string {
				start := v.editWorkDir
				if start == "" {
					start = dirname(v.editPath)
				}
				return v.app.pickDirectory(start)
			}, func(p string) { v.editWorkDir = p })
		})
	})

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Checkbox(c, &v.editAdmin, m.RunAsAdministrator())
		ui.Icon(c, iconShield).FontSize(13).TextColor(c.Theme().Warning)
		ui.Text(c, m.UacHint()).FontSize(11).TextColor(p.muted)
	})

	v.labeled(c, m.Icon(), func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &v.editIcon).Grow(1).Placeholder(m.IconAuto())
			v.browseButton(c, m.Browse(), func() string {
				start := dirname(v.editIcon)
				if start == "" {
					start = dirname(v.editPath)
				}
				return v.app.pickImage(start)
			}, func(p string) { v.editIcon = p })
			ui.Button(c, m.Clear()).OnClick(func() { v.editIcon = ""; v.refresh() })
		})
	})
}

// dirname is the folder a picker starts in: the one of a path, if any.
func dirname(p string) string {
	if p == "" {
		return ""
	}
	p = strings.TrimRight(p, `\/`)
	if p == "" {
		return ""
	}
	if !strings.ContainsAny(p, `\/`) && !(len(p) >= 2 && p[1] == ':') {
		return ""
	}
	return filepath.Dir(p)
}

// runtimeDialog edits an item's accumulated runtime: exactly, in minutes, or an
// hour at a time with the buttons under the field.
func (v *view) runtimeDialog(c *ui.Context) {
	if !v.runtimeOpen {
		return
	}
	p := v.pal
	guid := v.runtimeGUID
	item := v.app.item(guid)
	if item == nil {
		v.runtimeOpen = false
		return
	}
	current := v.baseline(guid)
	m := v.msgs()

	minutes := 0
	if n, err := strconv.Atoi(strings.TrimSpace(v.runtimeMinutes)); err == nil {
		minutes = max(0, n)
	}
	preview := formatRuntime(v.lang, int64(minutes)*60000)

	ui.Modal(c, &v.runtimeOpen, func() {
		ui.Column(c).Width(460).Gap(10).Children(func() {
			ui.Text(c, m.EditRuntime()).FontSize(17).FontWeight(700)

			v.labeled(c, m.RuntimeMinutes(), func() {
				if ui.TextInput(c, &v.runtimeMinutes).Submitted() {
					v.saveRuntime()
				}
			})
			ui.Text(c, m.RuntimeCurrent(formatRuntime(v.lang, current), strconv.FormatInt(current, 10), preview)).
				FontSize(11).TextColor(p.muted)

			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Button(c, "-1h").Tooltip(m.SubtractHour()).
					OnClick(func() { v.adjustRuntime(-time.Hour) })
				ui.Button(c, "+1h").Tooltip(m.AddHour()).
					OnClick(func() { v.adjustRuntime(time.Hour) })
				ui.Text(c, m.RuntimeChangesSaved()).FontSize(11).TextColor(p.muted)
			})

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c, m.Cancel()).OnClick(func() { v.runtimeOpen = false; v.refresh() })
				ui.PrimaryButton(c, m.Save()).OnClick(v.saveRuntime)
			})
		})
	})
}

// adjustRuntime moves an item's stored runtime by d, never below zero. The
// dialog stays open, so the buttons can be pressed repeatedly, and the minutes
// field follows the saved value: pressing Save afterwards cannot undo them.
func (v *view) adjustRuntime(d time.Duration) {
	guid := v.runtimeGUID
	if v.app.item(guid) == nil {
		v.runtimeOpen = false
		return
	}
	ms := max(0, v.baseline(guid)+d.Milliseconds())
	if err := v.app.setRuntimeMs(guid, ms); err != nil {
		v.app.notify(err.Error())
		return
	}
	v.runtimeMinutes = strconv.FormatInt(ms/60000, 10)
	v.refresh()
}

func (v *view) saveRuntime() {
	minutes, err := strconv.Atoi(strings.TrimSpace(v.runtimeMinutes))
	if err != nil {
		v.app.notify(v.msgs().RuntimeInvalid())
		return
	}
	if err := v.app.setRuntimeMs(v.runtimeGUID, int64(max(0, minutes))*60000); err != nil {
		v.app.notify(err.Error())
		return
	}
	v.runtimeOpen = false
}

// renameDialog renames an item or a tab.
func (v *view) renameDialog(c *ui.Context) {
	if !v.renameOpen {
		return
	}
	ui.Modal(c, &v.renameOpen, func() {
		ui.Column(c).Width(380).Gap(12).Children(func() {
			ui.Text(c, v.renameTitle).FontSize(17).FontWeight(700)
			if ui.TextInput(c, &v.renameName).AutoFocus().Submitted() {
				v.applyRename()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c, v.msgs().Cancel()).OnClick(func() { v.renameOpen = false; v.refresh() })
				ui.PrimaryButton(c, v.msgs().Ok()).OnClick(v.applyRename)
			})
		})
	})
}

// confirmDialog asks before something that cannot be undone.
func (v *view) confirmDialog(c *ui.Context) {
	if !v.confirmOpen {
		return
	}
	m := v.msgs()
	switch ui.AlertDialog(c, &v.confirmOpen, m.Confirm(), v.confirmMsg, m.Cancel(), m.Ok()) {
	case 1:
		action := v.confirmAction
		v.confirmAction = nil
		if action != nil {
			v.later(action)
		}
	}
}
