package tools

// GORILLA OVERRIDE (2026-08-19): tests for the thing that started everything.
//
// A model was handed a screenshot on 2026-08-18 and said it could not read
// images, while tesseract sat installed on the same machine. That one gap
// produced the tooling proposal, /arsenal, and this file.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeTextImage draws a known string so the test asserts against something it
// controls rather than against whatever happens to be on disk.
func makeTextImage(t *testing.T, text string) string {
	t.Helper()
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("imagemagick not installed")
	}
	p := filepath.Join(t.TempDir(), "sample.png")
	cmd := exec.Command("convert", "-size", "900x220", "xc:white",
		"-pointsize", "44", "-fill", "black", "-annotate", "+30+110", text, p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not render a test image: %v\n%s", err, out)
	}
	return p
}

func TestOCRReadsTextOutOfAnImage(t *testing.T) {
	if !OCRAvailable() {
		t.Skip("tesseract not installed")
	}
	const want = "ARSENAL capabilities present"
	res, err := OCRImage(context.Background(), makeTextImage(t, want))
	if err != nil {
		t.Fatalf("OCRImage: %v", err)
	}
	if res.Empty {
		t.Fatal("reported no text in an image containing text")
	}
	for _, w := range strings.Fields(want) {
		if !strings.Contains(res.Text, w) {
			t.Errorf("OCR lost the word %q; got: %q", w, res.Text)
		}
	}
}

// A photograph of a landscape has no words in it. "No text found" is a real
// answer and must not be reported as a failure — the two lead somewhere
// completely different.
func TestAnImageWithNoTextIsAnAnswerNotAFailure(t *testing.T) {
	if !OCRAvailable() {
		t.Skip("tesseract not installed")
	}
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("imagemagick not installed")
	}
	p := filepath.Join(t.TempDir(), "blank.png")
	if out, err := exec.Command("convert", "-size", "400x300", "xc:skyblue", p).CombinedOutput(); err != nil {
		t.Skipf("could not render: %v\n%s", err, out)
	}
	res, err := OCRImage(context.Background(), p)
	if err != nil {
		t.Fatalf("an image with no text returned an ERROR rather than an empty result: %v", err)
	}
	if !res.Empty {
		t.Errorf("found text in a blank image: %q", res.Text)
	}
}

// The header does the real work: OCR output is a transcription, and a model
// handed it without marking will quote it as though it read the picture.
func TestTheHeaderSaysWhatOCRTextIsAndIsNot(t *testing.T) {
	h := ocrHeader("/tmp/shot.png", OCRResult{Text: "hello"})
	for _, want := range []string{"TRANSCRIPTION", "not the image", "appears to say", "cannot describe"} {
		if !strings.Contains(h, want) {
			t.Errorf("the header does not say %q:\n%s", want, h)
		}
	}
	if !strings.Contains(h, "/tmp/shot.png") {
		t.Error("the header does not name the file it read")
	}
	trunc := ocrHeader("/tmp/shot.png", OCRResult{Truncated: true})
	if !strings.Contains(trunc, "TRUNCATED") {
		t.Error("a truncated read did not say so — silent truncation is the bug this project keeps fixing")
	}
}

// A refusal that does not say how to fix it is how a capability stays missing
// for months. That is not hypothetical here: it is exactly what happened.
func TestTheRefusalNamesTheFixAndTheCost(t *testing.T) {
	m := noOCRMessage("/tmp/shot.png", "image/png")
	for _, want := range []string{"tesseract-ocr", "pacman", "no account", "/arsenal", "/tmp/shot.png"} {
		if !strings.Contains(m, want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, m)
		}
	}
	if strings.Contains(m, "Use a different tool") {
		t.Error("still telling the model to use a different tool that does not exist")
	}
}

// Detection is measured, never assumed — the whole feature exists because
// something installed was believed absent.
func TestOCRAvailabilityIsMeasured(t *testing.T) {
	_, err := exec.LookPath("tesseract")
	if got := OCRAvailable(); got != (err == nil) {
		t.Errorf("OCRAvailable() = %v but LookPath says %v", got, err == nil)
	}
}

// Reading a file must never leave anything behind.
func TestOCRWritesNothingBesideTheImage(t *testing.T) {
	if !OCRAvailable() {
		t.Skip("tesseract not installed")
	}
	p := makeTextImage(t, "leave nothing behind")
	dir := filepath.Dir(p)
	before, _ := os.ReadDir(dir)
	if _, err := OCRImage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		var names []string
		for _, e := range after {
			names = append(names, e.Name())
		}
		t.Errorf("OCR left files behind: %v", names)
	}
}

// GORILLA OVERRIDE (2026-08-19): the tool DESCRIPTION is part of the
// capability.
//
// OCR was wired into view and a live model still refused — "the view tool
// reports PNG files are binary and cannot be displayed" — without ever calling
// it. The description still said "Cannot display binary files or images", so
// the model correctly believed the schema and never tried.
//
// This codebase has the identical failure written up in fetch.go: the tool was
// called "fetch" and models routinely told users they had no way to read a web
// page while the tool sat enabled in their schema. A capability the schema
// denies does not exist, however well it is implemented.
func TestTheViewDescriptionDoesNotDenyOCR(t *testing.T) {
	d := viewDescription
	if strings.Contains(d, "Cannot display binary files or images") {
		t.Error("the description still says images cannot be displayed; a model reading that " +
			"will refuse without calling the tool")
	}
	if strings.Contains(d, "Images can be identified but not displayed") {
		t.Error("the description still says images can only be identified")
	}
	for _, want := range []string{"RETURNS THE TEXT IN IT", "OCR", "TRANSCRIPTION", "Never say you cannot read an image"} {
		if !strings.Contains(d, want) {
			t.Errorf("the description does not state the capability: missing %q", want)
		}
	}
	// It must also state the limit, or the model will be asked to describe a
	// photograph and will invent one.
	if !strings.Contains(d, "cannot tell you what a photograph depicts") {
		t.Error("the description does not say OCR reads words only")
	}
}
