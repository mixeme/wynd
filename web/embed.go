package web

import "embed"

// Frontend assets must exist before go build (npm run build in web/).
// all: includes chunk files like _-82-yAF.js (dist/* skips names starting with _).
//
//go:embed all:dist
var Build embed.FS
