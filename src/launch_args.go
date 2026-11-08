package main

import "strings"

// launchOptions are the flags a launch can carry on its command line.
type launchOptions struct {
	// silent starts the app without showing the main window.
	silent bool
	// atCursor shows the window centered on the pointer.
	atCursor bool
}

// parseLaunchArgs reads the launcher's own flags; anything else is left to
// the rest of the app. Silent wins when both ask for a window.
func parseLaunchArgs(args []string) launchOptions {
	var opts launchOptions
	for _, arg := range args {
		switch {
		case strings.EqualFold(arg, "--silent"):
			opts.silent = true
		case strings.EqualFold(arg, "--at-cursor"):
			opts.atCursor = true
		}
	}
	if opts.silent {
		opts.atCursor = false
	}
	return opts
}
