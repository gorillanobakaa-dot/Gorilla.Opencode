// GORILLA OVERRIDE: this file did not exist upstream. It is the answer to the
// `// TODO: handle images` that has sat in view.go since the fork.
//
// WHY IT IS HERE, and it is not a hypothetical. On 2026-08-18 a model was
// handed a screenshot and reported that it could not read images — true of the
// model, false of the machine, because tesseract was installed and had been for
// months. That single gap is what produced the whole tooling proposal and the
// /arsenal command.
//
// Then on 2026-08-19, before this was written, the owner watched a model do it
// ANYWAY: asked what was in a folder of screenshots, it ran `command -v
// tesseract`, found it, built its own `for f in *.png; do tesseract "$f" -`
// pipeline through bash, and read the text back out of the images. It worked.
// It also took several turns, produced one malformed command it had to
// apologise for, and needed a permission prompt for every attempt.
//
// So the capability was reachable and the ROUTE was terrible. That is the
// argument for putting it behind `view`: not new power, the same power without
// the trial and error. An image now reads like a file reads.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ocrTimeout bounds a single page. Tesseract on a 1600x900 screenshot takes
// well under a second on the machine this is built for; anything past this is
// a pathological image, not a slow one.
const ocrTimeout = 45 * time.Second

// maxOCRChars caps what OCR may put into the conversation. A dense page of text
// is a few thousand characters; a mis-thresholded photograph can produce tens
// of thousands of characters of garbage, and every one of them would ride every
// later turn.
const maxOCRChars = 20000

// OCRAvailable reports whether this machine can read text out of an image.
//
// Checked, never assumed — the entire reason this file exists is a capability
// that was present and unknown. The counterpart is /arsenal, which tells the
// USER the same thing.
func OCRAvailable() bool {
	_, err := exec.LookPath("tesseract")
	return err == nil
}

// OCRResult is what came back, and how much to trust it.
type OCRResult struct {
	Text string
	// Truncated says the text was cut at maxOCRChars.
	Truncated bool
	// Empty says tesseract ran fine and found no words. That is a real
	// answer — a photograph of a landscape has no text — and it must not be
	// reported as a failure.
	Empty bool
}

// OCRImage reads the text out of an image using tesseract.
//
// The image is NOT preprocessed here. Deskewing and thresholding with
// imagemagick genuinely helps a photographed page, and blanket upscaling was
// MEASURED to make three of five test images worse — so the useful
// preprocessing is image-dependent and belongs in a deliberate step the model
// asks for, not in the path that runs on every screenshot.
func OCRImage(ctx context.Context, path string) (OCRResult, error) {
	if !OCRAvailable() {
		return OCRResult{}, fmt.Errorf("tesseract is not installed")
	}
	runCtx, cancel := context.WithTimeout(ctx, ocrTimeout)
	defer cancel()

	// "-" sends the text to stdout instead of writing a file next to the
	// image. Reading a file must never leave anything behind.
	cmd := exec.CommandContext(runCtx, "tesseract", path, "-")
	var out, errBuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return OCRResult{}, fmt.Errorf("OCR timed out after %s", ocrTimeout)
		}
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return OCRResult{}, fmt.Errorf("tesseract failed: %s", firstLine(msg))
	}

	text := strings.TrimSpace(out.String())
	res := OCRResult{Text: text, Empty: text == ""}
	if len(text) > maxOCRChars {
		res.Text = text[:maxOCRChars]
		res.Truncated = true
	}
	return res, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ocrHeader is what the model is told ABOUT the text, and it does most of the
// work of this feature.
//
// OCR output is a TRANSCRIPTION, not the image. It drops layout, it invents
// characters, and it degrades badly on anything that is not clean text — the
// owner's own run produced "Terminal Qe - o x" from a screenshot of a
// screenshot, which is exactly the failure mode worth warning about. A model
// handed OCR text with no marking will quote it as though it read the picture.
func ocrHeader(path string, res OCRResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "TEXT READ OUT OF THE IMAGE %s using OCR (tesseract).\n", path)
	b.WriteString("This is a TRANSCRIPTION, not the image. OCR mistakes similar shapes " +
		"(l/1/I, 0/O, rn/m), loses layout and columns, and degrades badly on small, " +
		"low-contrast or re-photographed text. Quote it as \"the image appears to say\", " +
		"never as certain, and say so if a detail looks garbled.\n")
	b.WriteString("It cannot describe what the picture SHOWS - only words it found in it.\n")
	if res.Truncated {
		fmt.Fprintf(&b, "TRUNCATED at %d characters; there was more text in the image.\n", maxOCRChars)
	}
	return b.String()
}

// noOCRMessage is what a machine without tesseract is told.
//
// It names the capability, the cost and the exact command, in the same voice as
// /arsenal — because a refusal that does not say how to fix it is how a
// capability stays missing for months.
func noOCRMessage(path, imageType string) string {
	return fmt.Sprintf(
		"This is an image (%s), and this machine cannot read text out of images yet.\n\n"+
			"To fix that, install OCR - it is small, free, needs no account, and runs entirely on\n"+
			"your machine, so nothing but the words ever leaves it:\n\n"+
			"    sudo apt-get install -y tesseract-ocr tesseract-ocr-eng\n"+
			"    (Arch: sudo pacman -S --needed tesseract tesseract-data-eng)\n\n"+
			"Then view %s again and the text will come back. Run /arsenal to see what else\n"+
			"this machine could do and what it would cost.",
		imageType, path)
}
