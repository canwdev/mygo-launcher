package main

// iconMetrics is how large the grid's items are drawn: the icon size
// preference picks a step.
type iconMetrics struct {
	gridIcon   float32 // an item's icon
	cellWidth  float32 // a cell's least width
	cellHeight float32 // a cell's height
	gridFont   float32 // an item's name
}

// icons returns the metrics of the icon size the preference asks for; medium,
// the default, is what the view drew before the preference existed.
func (v *view) icons() iconMetrics {
	switch v.prefs.IconSize {
	case "small":
		return iconMetrics{gridIcon: 30, cellWidth: 72, cellHeight: 78, gridFont: 11.5}
	case "large":
		return iconMetrics{gridIcon: 54, cellWidth: 108, cellHeight: 112, gridFont: 13}
	}
	return iconMetrics{gridIcon: 40, cellWidth: 86, cellHeight: 92, gridFont: 12}
}
