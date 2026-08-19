# Gorilla OpenCode v0.1.105 — it can read your screenshots

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.105_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.105_amd64.deb` |
| `gorilla-opencode-0.1.105-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.105.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.
Needs `tesseract-ocr` for the new feature — the program tells you the exact
command if you do not have it.

---

## Screenshots - the gap, and the model routing around it

*Click any image for the full-resolution original. Unscaled, uncropped, nothing staged.*

**Before.** With no OCR in the view tool, the model went looking for tesseract itself and wrote its own shell pipeline to run it over every file. The ability was there; the route to it was awful.

[![The agent probing for tesseract with command -v and then building its own shell loop running identify and tesseract over every PNG in the screenshots folder](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-model-built-its-own-ocr-pipeline.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-model-built-its-own-ocr-pipeline.png)

**Before.** Every attempt needed its own permission prompt, because every attempt was a shell command.

[![A permission dialog for the bash tool showing a cd into the Screenshots folder followed by a for loop over PNG files, one of several such prompts during the attempt](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-permission-prompt-per-attempt.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-permission-prompt-per-attempt.png)

**Before.** It did eventually work - that is the ARSENAL page read back out of a screenshot - after several turns and a malformed command it had to apologise for. This release makes it one call.

[![The agent successfully reading the ARSENAL page text out of a PNG screenshot using tesseract through bash, showing the capability list and package manager line transcribed](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-ocr-through-bash-worked-slowly.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.105/docs/screenshots/gallery/v0105-before-ocr-through-bash-worked-slowly.png)

---

## Plain-language track

### The thing that started all of this

Yesterday someone asked this program to read a screenshot. It said it could not
read images.

That was true of the AI and false of the computer. **Tesseract — the tool that
reads text out of pictures — was already installed on that machine and had been
for months.** Nothing had ever connected the two.

That one gap produced everything built since: the tooling research, the
`/arsenal` capability map, and now this. The line in the source read
`// TODO: handle images`, and the program's answer was "use a different tool to
process images" — a different tool that does not exist.

### And then it did it anyway, the hard way

Before this was written, the owner watched a model **work around the problem on
its own**. Asked what was in a folder of screenshots, it went looking for
tesseract, found it, wrote its own command to run it over every file, and read
the text back out.

It worked. It also took several attempts, produced one broken command it had to
apologise for, and needed permission for every try.

So the ability was there and the **route to it was awful.** This release is not
new power. It is the same power without the fumbling: **an image now reads the
way a file reads.**

### What you get

Point it at a screenshot, a scanned page, a photo of a document:

> **you:** view that screenshot and tell me what the install command is
>
> **it:** `sudo apt-get install -y libreoffice-calc libreoffice-core libreoffice-writer zbar-tools`
>
> *One caveat: the OCR rendered the third package with a capital L, but Debian
> package names are always lowercase — that's an OCR artifact, not real
> capitalisation.*

That caveat is not decoration. The answer arrives labelled as a **transcription,
not the picture**, with a warning that OCR confuses similar shapes and loses
layout — and the model used it correctly, unprompted, on the first try.

### Three things it is careful about

**It tells you what OCR is not.** The text comes back marked as a transcription.
It reads *words*; it cannot tell you what a photograph shows, what colour
something is, or where things sit on the screen. Without that marking, an AI
will happily quote OCR output as though it had looked at the image.

**"No text found" is an answer, not a failure.** A photograph of a landscape has
no words in it. That is a completely different thing from the reading having
gone wrong, and they lead somewhere different.

**If your machine has no OCR, it tells you the exact command.** Not "I can't do
that" — the one line that fixes it, free, no account, running entirely on your
own machine so nothing but the words ever leaves it:

```
sudo apt-get install -y tesseract-ocr tesseract-ocr-eng
```

### The part that nearly shipped broken

OCR was wired in, tested, working — and a live model **still refused**, saying
the view tool reports PNG files as binary and cannot display them. It never
called the tool at all.

Because the tool's own description still said *"Cannot display binary files or
images"*. The model read that and believed it, correctly.

This program already had that exact failure written down: its web-fetching tool
used to be called `fetch`, and models routinely told users they had no way to
read a web page while the tool sat enabled and ready. **A capability the
description denies does not exist, however well it is built.**

---

## Developer track

### `internal/llm/tools/ocr.go`

`OCRAvailable()` is `exec.LookPath("tesseract")` — measured, never assumed, which
is the entire point. `OCRImage` runs `tesseract <path> -` so output goes to
stdout and nothing is written beside the image (`TestOCRWritesNothingBesideTheImage`).
45-second bound, 20,000-character cap.

**No preprocessing on this path.** Deskew and threshold genuinely help a
photographed page, but blanket upscaling was measured to make three of five test
images *worse*, so useful preprocessing is image-dependent and belongs in a step
the model asks for — not in the path that runs on every screenshot.

`ocrHeader` states the transcription caveat; `noOCRMessage` gives the apt and
pacman commands plus a pointer to `/arsenal`. Both are tested for content
rather than existence.

### `view.go`

The image branch replaces the `// TODO`. `Empty` returns a `NewTextResponse`
(an answer) rather than an error, because "this picture has no words in it" and
"the read failed" must never be conflated.

### The description

`TestTheViewDescriptionDoesNotDenyOCR` fails the build if the description
reasserts that images cannot be read, and requires it to state both the
capability and its limit. Added after a live model refused a working feature on
the strength of a stale sentence.

### Verified

Live, `deepseek-v4-flash`, the identical request that had failed minutes
earlier: exact command extracted from a 1594x892 PNG, plus an unprompted and
correct diagnosis of an OCR artifact in its own output.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| The TODO and the "use a different tool" refusal | 📄 stated in input | `view.go` before this commit. |
| A model built its own tesseract pipeline through bash | 📄 stated in input | User screenshots, embedded above. |
| A live model refused because of the description | 📄 stated in input | Run captured before and after the description change. |
| Exact command extracted, artifact self-diagnosed | 📄 stated in input | Live run, output quoted verbatim. |
| Blanket upscaling made 3 of 5 images worse | 📄 stated in input | Measured in the tooling research, not re-measured here. |
| No preprocessing is the right default | 🤖 model inference | Follows from the above; the alternative was not A/B tested here. |
| OCR quality on your images | 🚫 not established | Depends entirely on the image. Terminal screenshots read well; re-photographed screens read badly. |
