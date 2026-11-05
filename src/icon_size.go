package main

// iconMetrics is how large the items are drawn in both layouts: the icon size
// preference picks a step, and the list and the grid take their icons, rows
// and cells from it.
type iconMetrics struct {
	gridIcon   float32 // an item's icon in the grid
	cellWidth  float32 // a grid cell's least width
	cellHeight float32 // a grid cell's height
	gridFont   float32 // an item's name in the grid
	listIcon   float32 // an item's icon in the list
	rowHeight  float32 // a list row's height
	listFont   float32 // an item's name in the list
}

// icons returns the metrics of the icon size the preference asks for; medium,
// the default, is what the views drew before the preference existed.
func (v *view) icons() iconMetrics {
	switch v.prefs.IconSize {
	case "small":
		return iconMetrics{gridIcon: 30, cellWidth: 72, cellHeight: 78, gridFont: 11.5, listIcon: 24, rowHeight: 40, listFont: 13}
	case "large":
		return iconMetrics{gridIcon: 54, cellWidth: 108, cellHeight: 112, gridFont: 13, listIcon: 38, rowHeight: 54, listFont: 15}
	}
	return iconMetrics{gridIcon: 40, cellWidth: 86, cellHeight: 92, gridFont: 12, listIcon: 30, rowHeight: 46, listFont: 14}
}
