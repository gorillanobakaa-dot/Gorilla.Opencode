package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"
)

// GORILLA FIX (2026-10-27, job 09, gap #15): OCR without tesseract on
// Windows. Windows 10 and 11 carry an OCR engine (Windows.Media.Ocr), offline,
// using the language packs already installed for the user. It is a WinRT API;
// Windows PowerShell 5.1 can call WinRT directly, so it is reached through
// that, with no new dependency and no download. tesseract still wins when it
// is installed: it is the engine the rest of this file was measured with.
//
// PowerShell 7 cannot load WinRT types, so the 5.1 in System32 is named by
// its full path, never looked up on PATH.

const windowsOCREngine = "Windows OCR"

// windowsOCRScript reads the image named by $env:GORILLA_OCR_IMAGE and prints
// one recognized line per output line, UTF-8. Exit 3: no OCR language is
// installed for this user.
const windowsOCRScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime]
$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType = WindowsRuntime]
$null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics, ContentType = WindowsRuntime]
$asTask = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
  $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
  $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' })[0]
function Await($op, [Type]$type) {
  $task = $asTask.MakeGenericMethod($type).Invoke($null, @($op))
  [void]$task.Wait(-1)
  $task.Result
}
$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
if ($engine -eq $null) { [Console]::Error.WriteLine('no OCR language is installed for this user'); exit 3 }
if ($env:GORILLA_OCR_PROBE -eq '1') { 'ready ' + $engine.RecognizerLanguage.LanguageTag; exit 0 }
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($env:GORILLA_OCR_IMAGE)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
try {
  $decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
  $max = [Windows.Media.Ocr.OcrEngine]::MaxImageDimension
  if ($decoder.PixelWidth -gt $max -or $decoder.PixelHeight -gt $max) {
    [Console]::Error.WriteLine("the image is " + $decoder.PixelWidth + "x" + $decoder.PixelHeight + " pixels; Windows OCR reads at most " + $max + " on a side")
    exit 4
  }
  $bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
  $result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
  foreach ($line in $result.Lines) { [Console]::Out.WriteLine($line.Text) }
} finally { $stream.Dispose() }
`

// windowsPowerShell51 is the System32 Windows PowerShell, the one that can
// load WinRT types.
func windowsPowerShell51() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

// encodePS is the -EncodedCommand form: base64 of UTF-16LE.
func encodePS(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// runWindowsOCR runs the script with extra environment entries.
func runWindowsOCR(ctx context.Context, env ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, windowsPowerShell51(),
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePS(windowsOCRScript))
	cmd.Env = append(os.Environ(), env...)
	var out, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	return out.String(), strings.TrimSpace(errBuf.String()), err
}

var (
	windowsOCROnce  sync.Once
	windowsOCRReady bool
	windowsOCRWhy   string
)

// windowsOCRAvailable asks once per process whether the engine can be made
// for this user (a language pack with OCR is installed).
func windowsOCRAvailable() (bool, string) {
	windowsOCROnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ocrTimeout)
		defer cancel()
		out, errOut, err := runWindowsOCR(ctx, "GORILLA_OCR_PROBE=1")
		if err == nil && strings.HasPrefix(strings.TrimSpace(out), "ready") {
			windowsOCRReady = true
			windowsOCRWhy = strings.TrimSpace(out)
			return
		}
		windowsOCRWhy = firstLine(firstNonEmpty(errOut, fmt.Sprint(err)))
	})
	return windowsOCRReady, windowsOCRWhy
}

// windowsOCR reads the text out of one image with the Windows engine.
func windowsOCR(ctx context.Context, path string) (OCRResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return OCRResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, ocrTimeout)
	defer cancel()
	out, errOut, err := runWindowsOCR(runCtx, "GORILLA_OCR_IMAGE="+abs)
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return OCRResult{}, fmt.Errorf("OCR timed out after %s", ocrTimeout)
		}
		return OCRResult{}, fmt.Errorf("Windows OCR failed: %s", firstLine(firstNonEmpty(errOut, err.Error())))
	}
	return capOCR(strings.TrimSpace(strings.ReplaceAll(out, "\r\n", "\n")), windowsOCREngine), nil
}
