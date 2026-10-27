package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const windowsOCRWant = "Gorilla reads this offline"

func init() { platformTextImage = makeWindowsTextImage }

// makeWindowsTextImage draws a known line into a small PNG with
// System.Drawing, which every Windows has (the ImageMagick helper in
// ocr_test.go is rarely installed there).
func makeWindowsTextImage(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ocr-sample.png")
	script := `Add-Type -AssemblyName System.Drawing
$bmp = New-Object System.Drawing.Bitmap 1000, 140
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.Clear([System.Drawing.Color]::White)
$g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAlias
$font = New-Object System.Drawing.Font 'Arial', 40
$g.DrawString($env:GORILLA_OCR_TEXT, $font, [System.Drawing.Brushes]::Black, 20, 35)
$g.Dispose()
$bmp.Save($env:GORILLA_OCR_OUT, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()`
	cmd := exec.Command(windowsPowerShell51(), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePS(script))
	cmd.Env = append(os.Environ(), "GORILLA_OCR_TEXT="+text, "GORILLA_OCR_OUT="+p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("could not draw the test image: %v\n%s", err, out)
	}
	return p
}

// TestWindowsOCRReadsAGeneratedPNG: the engine built into Windows reads a
// PNG drawn on this machine, offline, with no tesseract involved.
func TestWindowsOCRReadsAGeneratedPNG(t *testing.T) {
	if ok, why := windowsOCRAvailable(); !ok {
		t.Fatalf("Windows OCR cannot start on this machine: %s", why)
	}
	res, err := windowsOCR(context.Background(), makeWindowsTextImage(t, windowsOCRWant))
	if err != nil {
		t.Fatalf("windowsOCR: %v", err)
	}
	if res.Engine != windowsOCREngine || res.Empty {
		t.Fatalf("result = %+v", res)
	}
	for _, w := range strings.Fields(windowsOCRWant) {
		if !strings.Contains(strings.ToLower(res.Text), strings.ToLower(w)) {
			t.Errorf("Windows OCR lost the word %q; got %q", w, res.Text)
		}
	}
}

// TestWindowsOCRIsTheFallbackWithoutTesseract: with tesseract nowhere,
// view's OCR still works and its header names the engine that read it.
func TestWindowsOCRIsTheFallbackWithoutTesseract(t *testing.T) {
	img := makeWindowsTextImage(t, windowsOCRWant)
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	t.Setenv("ProgramFiles", empty)
	t.Setenv("ProgramFiles(x86)", empty)
	if p := tesseractPath(); p != "" {
		t.Skipf("tesseract is installed at a fixed path (%s); the fallback cannot be reached", p)
	}
	if !OCRAvailable() {
		t.Fatal("OCRAvailable() = false with Windows OCR present")
	}
	res, err := OCRImage(context.Background(), img)
	if err != nil {
		t.Fatalf("OCRImage: %v", err)
	}
	if res.Engine != windowsOCREngine || !strings.Contains(strings.ToLower(res.Text), "offline") {
		t.Errorf("result = %+v", res)
	}
	if h := ocrHeader(img, res); !strings.Contains(h, "(Windows OCR)") {
		t.Errorf("the header does not name the engine:\n%s", h)
	}
	if s := strings.Join(OCRStatus(), "\n"); !strings.Contains(s, windowsOCREngine) {
		t.Errorf("doctor does not name the engine: %s", s)
	}
}
