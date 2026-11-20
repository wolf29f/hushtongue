//go:build !embed

package web

import (
	"embed"
	"io/fs"
)

//go:embed noembed
var noembed embed.FS

// Dist returns a front made of a page telling that the front isn't built.
func Dist() fs.FS {
	sub, err := fs.Sub(noembed, "noembed")
	if err != nil {
		panic(err)
	}
	return sub
}
