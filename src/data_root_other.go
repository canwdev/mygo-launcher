//go:build !windows

package main

// installedDataBase is Windows-only: elsewhere the data stays beside the
// executable.
func installedDataBase() (string, bool) { return "", false }
