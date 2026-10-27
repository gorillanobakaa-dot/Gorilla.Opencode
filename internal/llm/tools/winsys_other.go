//go:build !windows

package tools

import (
	"context"
	"errors"
)

// The windows tool is registered only on Windows (winsys.go).
func windowsQuery(context.Context, WindowsParams, int) (winsysResult, error) {
	return winsysResult{}, errors.New("the windows tool exists only on Windows")
}
