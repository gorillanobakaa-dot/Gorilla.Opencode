//go:build !windows

package tools

import "os"

func makeDirLink(link, target string) error { return os.Symlink(target, link) }
