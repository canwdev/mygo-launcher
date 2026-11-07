//go:build !windows

package main

// runningWindow is a program that has a window on screen. Only Windows lists
// them; elsewhere the picker stays empty.
type runningWindow struct {
	Name  string
	Title string
	Path  string
	PID   uint32
}

func listRunningWindows() []runningWindow { return nil }

func appDisplayName(string) string { return "" }
