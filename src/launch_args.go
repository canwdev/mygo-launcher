package main

import "strings"

// launchOptions are what a launch can ask for on its command line.
type launchOptions struct {
	// silent starts the app without showing the main window.
	silent bool
	// place is where this launch shows the window; empty follows the window
	// position setting.
	place windowPlace
}

const positionFlag = "--position"

// parseLaunchArgs reads the launcher's own flags; unknown arguments are left
// alone. Silent wins over a position.
func parseLaunchArgs(args []string) launchOptions {
	var opts launchOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.EqualFold(arg, "--silent"):
			opts.silent = true
		case strings.EqualFold(arg, positionFlag):
			// A flag after it is no place: the value is missing.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				opts.setPlace(args[i+1])
				i++
			}
		case hasPositionValue(arg):
			opts.setPlace(arg[len(positionFlag)+1:])
		}
	}
	if opts.silent {
		opts.place = ""
	}
	return opts
}

// hasPositionValue reports whether arg is --position=PLACE, ignoring case.
func hasPositionValue(arg string) bool {
	return len(arg) > len(positionFlag) && arg[len(positionFlag)] == '=' &&
		strings.EqualFold(arg[:len(positionFlag)], positionFlag)
}

// setPlace reads PLACE into the options; a place the launcher does not know
// leaves them as they were.
func (opts *launchOptions) setPlace(value string) {
	switch {
	case strings.EqualFold(value, string(placeLast)):
		opts.place = placeLast
	case strings.EqualFold(value, string(placeCursor)):
		opts.place = placeCursor
	case strings.EqualFold(value, string(placeCenter)):
		opts.place = placeCenter
	}
}

// existingInstanceOpts are what a second launch applies to the copy that is
// already running: the place --position names, the pointer otherwise, and
// nothing at all for --silent.
func existingInstanceOpts(opts launchOptions) launchOptions {
	if opts.silent {
		return launchOptions{}
	}
	if opts.place == "" {
		opts.place = placeCursor
	}
	return opts
}
