//go:build !windows

package main

// extractIconResource reads icons out of Windows files; there are none.
func extractIconResource(string, int, int) []byte { return nil }
