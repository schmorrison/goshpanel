package web

import "embed"

// Static contains compiled CSS, JavaScript, and other browser assets.
//
//go:embed all:static
var Static embed.FS
