package web

import "embed"

// Files is the practice UI, compiled into the binary.
//
//go:embed index.html app.css app.js anzeige.html anzeige.css anzeige.js favicon.svg
var Files embed.FS
