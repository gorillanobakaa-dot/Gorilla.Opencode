//go:build !toolbox

package toolbox

import "io/fs"

// Built without `-tags toolbox`: no helper is embedded.
const tagged = false

func embeddedFS() fs.FS { return nil }
