module go-launcher

go 1.27.3

require (
	github.com/egoist/mygo v0.7.4
	github.com/google/uuid v1.6.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// The native UI is built on a local checkout of MyGo, which carries the
// package ui APIs this app uses. Drop the replace directive to build against
// the published module instead.
replace github.com/egoist/mygo => ../mygo

tool github.com/egoist/mygo/cmd/mygo
