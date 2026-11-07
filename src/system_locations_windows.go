//go:build windows

package main

import "go-launcher/locales"

// systemLocations are the shell folders the add picker offers. They keep the
// shell's parsing names, CLSIDs or shell: aliases: ShellExecute opens them,
// and the shell draws their icons.
var systemLocations = []systemLocation{
	{`::{645FF040-5081-101B-9F08-00AA002F954E}`, func(m locales.Messages) string { return m.RecycleBin() }},
	{`::{20D04FE0-3AEA-1069-A2D8-08002B30309D}`, func(m locales.Messages) string { return m.ThisPC() }},
	{`::{26EE0668-A00A-44D7-9371-BEB064C98683}`, func(m locales.Messages) string { return m.ControlPanel() }},
	{`shell:Profile`, func(m locales.Messages) string { return m.LocationUser() }},
	{`::{F02C1A0D-BE21-4350-88B0-7367FC96EF3C}`, func(m locales.Messages) string { return m.LocationNetwork() }},
	{`shell:Startup`, func(m locales.Messages) string { return m.LocationStartup() }},
	{`shell:Recent`, func(m locales.Messages) string { return m.LocationRecent() }},
	{`shell:Desktop`, func(m locales.Messages) string { return m.LocationDesktop() }},
	{`shell:Downloads`, func(m locales.Messages) string { return m.LocationDownloads() }},
	{`shell:Personal`, func(m locales.Messages) string { return m.LocationDocuments() }},
	{`shell:My Pictures`, func(m locales.Messages) string { return m.LocationPictures() }},
	{`shell:My Music`, func(m locales.Messages) string { return m.LocationMusic() }},
	{`shell:My Video`, func(m locales.Messages) string { return m.LocationVideos() }},
}
