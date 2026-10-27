//go:build !windows

package tools

import (
	"context"
	"errors"
)

// The Windows OCR engine exists only on Windows (ocr_windows.go).

const windowsOCREngine = "Windows OCR"

func windowsOCRAvailable() (bool, string) { return false, "not Windows" }

func windowsOCR(context.Context, string) (OCRResult, error) {
	return OCRResult{}, errors.New("Windows OCR exists only on Windows")
}
