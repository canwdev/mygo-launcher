package main

// minWindowOpacity is how see-through the window may get: the whole window
// fades, texts included, so less than this leaves them unreadable.
const minWindowOpacity = 0.3

// windowTransparency is the slider's view of the window opacity: how much of
// what is behind the window shows through it, from 0 (opaque) to 1 - the
// floor.
func windowTransparency(opacity float64) float64 {
	return 1 - min(1, max(minWindowOpacity, opacity))
}

// windowOpacityFrom is the preference the transparency slider stores.
func windowOpacityFrom(transparency float64) float64 {
	return 1 - min(1-minWindowOpacity, max(0, transparency))
}

// applyWindowOpacity paints the window at the opacity the preference gives.
// Windows are layered for it on the fly, so the settings show the change as
// it is made and no restart is needed.
func (v *view) applyWindowOpacity() {
	w := v.app.win
	if w == nil {
		// Without a window, as in tests, there is nothing to paint.
		return
	}
	w.SetOpacity(min(1, max(minWindowOpacity, v.prefs.WindowOpacity)))
}

// setWindowOpacity keeps the new opacity and paints the window at it.
func (v *view) setWindowOpacity(opacity float64) {
	v.prefs.WindowOpacity = min(1, max(minWindowOpacity, opacity))
	v.changedSettings()
	v.applyWindowOpacity()
}

// setBackgroundOpacity keeps the new opacity of the background picture.
func (v *view) setBackgroundOpacity(value float64) {
	v.prefs.BackgroundOpacity = value
	v.changedSettings()
}
