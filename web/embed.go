// Package web embeds the built frontend (web/dist) into the server binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the built frontend. It holds only a placeholder until
// `npm run build` has been run in this directory.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the embedded directory name is fixed at compile time
	}
	return sub
}
