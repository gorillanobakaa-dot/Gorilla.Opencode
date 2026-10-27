//go:build toolbox

package toolbox

import (
	"embed"
	"io/fs"
)

// bin/<goos>-<goarch>/ is filled by scripts/build-toolbox.ps1 before
// `go build -tags toolbox`; only bin/README.md is committed, so with nothing
// built this embeds no helper and every lookup is ErrNotEmbedded.
//
//go:embed bin
var embedded embed.FS

const tagged = true

func embeddedFS() fs.FS {
	sub, err := fs.Sub(embedded, "bin")
	if err != nil {
		return nil
	}
	return sub
}
