package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/lsp"
)

type ViewParams struct {
	FilePath string  `json:"file_path"`
	Offset   FlexInt `json:"offset"`
	Limit    FlexInt `json:"limit"`
}

type viewTool struct {
	lspClients map[string]*lsp.Client
}

type ViewResponseMetadata struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

const (
	ViewToolName     = "view"
	MaxReadSize      = 5 * 1024 * 1024 // 5MB (was 250*1024; bumped to fit large JSON catalogues — DefaultReadLimit still caps to 2000 lines/read)
	DefaultReadLimit = 2000
	MaxLineLength    = 2000
	viewDescription  = `File viewing tool that reads and displays the contents of files with line numbers, allowing you to examine code, logs, or text data.

WHEN TO USE THIS TOOL:
- Use when you need to read the contents of a specific file
- Helpful for examining source code, configuration files, or log files
- Perfect for looking at text-based file formats

HOW TO USE:
- Provide the path to the file you want to view
- Optionally specify an offset to start reading from a specific line
- Optionally specify a limit to control how many lines are read

FEATURES:
- Displays file contents with line numbers for easy reference
- Can read from any position in a file using the offset parameter
- Handles large files by limiting the number of lines read
- Automatically truncates very long lines for better display
- Suggests similar file names when the requested file isn't found

IMAGES AND SCREENSHOTS:
- Point this at a .png/.jpg/.webp screenshot or scan and it RETURNS THE TEXT IN IT, read
  locally with OCR. Never say you cannot read an image without trying: try it first.
- What comes back is a TRANSCRIPTION, not the picture. It can misread similar shapes and
  loses layout, so quote it as "the image appears to say" and flag anything garbled.
- It reads WORDS only. It cannot tell you what a photograph depicts, what colour something
  is, or where things sit on screen.
- If this machine has no OCR installed, the answer says so and gives the one command that
  fixes it. That is a real answer - pass it on rather than guessing at the contents.

LIMITATIONS:
- Maximum file size is 5MB
- Default reading limit is 2000 lines
- Lines longer than 2000 characters are truncated
- Cannot display binary files that are not images

TIPS:
- Use the find tool first to locate files, then View to examine them
- find already returns matching lines with context, so View is only needed when you want MORE than the match window
- When viewing large files, use the offset parameter to read specific sections`
)

func NewViewTool(lspClients map[string]*lsp.Client) BaseTool {
	return &viewTool{
		lspClients,
	}
}

func (v *viewTool) Info() ToolInfo {
	return ToolInfo{
		Name:        ViewToolName,
		Description: viewDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "The line number to start reading from (0-based)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "The number of lines to read (defaults to 2000)",
			},
		},
		Required: []string{"file_path"},
	}
}

// Run implements Tool.
func (v *viewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ViewParams
	logging.Debug("view tool params", "params", call.Input)
	if err := UnmarshalToolInput(call.Input, &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	// Handle relative paths
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(config.WorkingDirectory(), filePath)
	}

	// GORILLA OVERRIDE (2026-08-18): credential files outside the project are
	// refused. Anything read here reaches the provider and the session database.
	// See sensitive.go — a blocklist, deliberately not a sandbox.
	if why := RefuseSensitiveRead(filePath); why != "" {
		return NewTextErrorResponse(why), nil
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to offer suggestions for similarly named files
			dir := filepath.Dir(filePath)
			base := filepath.Base(filePath)

			dirEntries, dirErr := os.ReadDir(dir)
			if dirErr == nil {
				var suggestions []string
				for _, entry := range dirEntries {
					if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(base)) ||
						strings.Contains(strings.ToLower(base), strings.ToLower(entry.Name())) {
						suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
						if len(suggestions) >= 3 {
							break
						}
					}
				}

				if len(suggestions) > 0 {
					return NewTextErrorResponse(fmt.Sprintf("File not found: %s\n\nDid you mean one of these?\n%s",
						filePath, strings.Join(suggestions, "\n"))), nil
				}
			}

			return NewTextErrorResponse(fmt.Sprintf("File not found: %s", filePath)), nil
		}
		return ToolResponse{}, fmt.Errorf("error accessing file: %w", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
	}

	// Check file size
	if fileInfo.Size() > MaxReadSize {
		return NewTextErrorResponse(fmt.Sprintf("File is too large (%d bytes). Maximum size is %d bytes",
			fileInfo.Size(), MaxReadSize)), nil
	}

	// Set default limit if not provided
	if params.Limit <= 0 {
		params.Limit = FlexInt(DefaultReadLimit)
	}

	// GORILLA OVERRIDE (2026-08-19): read the words out of it.
	//
	// This was `// TODO: handle images` and a refusal telling the model to
	// "use a different tool" — a different tool that does not exist. That
	// refusal is what produced the whole tooling proposal: a model reported it
	// could not read a screenshot while tesseract sat installed on the same
	// machine, unused, because nothing connected the two.
	//
	// Before this was written the owner watched a model route around it
	// anyway: `command -v tesseract`, then its own `for f in *.png; do
	// tesseract "$f" -` pipeline through bash. It worked, after several turns,
	// one malformed command and a permission prompt per attempt. The
	// capability was reachable and the route was terrible. This is the same
	// power without the trial and error.
	if isImage, imageType := isImageFile(filePath); isImage {
		if !OCRAvailable() {
			return NewTextErrorResponse(noOCRMessage(filePath, imageType)), nil
		}
		res, err := OCRImage(ctx, filePath)
		if err != nil {
			return NewTextErrorResponse(fmt.Sprintf(
				"Could not read text out of %s (%s): %v", filePath, imageType, err)), nil
		}
		if res.Empty {
			// A real answer, not a failure. A photograph of a landscape has no
			// words in it, and saying "no text found" is different from
			// saying the read failed.
			return NewTextResponse(fmt.Sprintf(
				"No text found in %s (%s). OCR ran successfully and the image contains no "+
					"words it could recognise - it may be a photograph, a diagram, or text too "+
					"small or low-contrast to read.", filePath, imageType)), nil
		}
		recordFileRead(filePath)
		return WithResponseMetadata(
			NewTextResponse(ocrHeader(filePath, res)+"\n<image_text>\n"+res.Text+"\n</image_text>\n"),
			ViewResponseMetadata{FilePath: filePath},
		), nil
	}

	// GORILLA FIX (2026-08-19), search-strategy audit extended to view: refuse
	// a binary file instead of dumping its bytes into the conversation.
	//
	// MEASURED before this: viewing a 7-byte binary returned
	//     <file>     1|\x00\x01\x02\x03\x00\xff\x00</file>
	// — raw bytes rendered as if they were source. The file-size limit is
	// 5 MB, so a real binary could put five megabytes of garbage into the
	// context, where it would then be re-sent on every later turn. The tool's
	// own description already claimed it "cannot display binary files"; it
	// displayed them, badly.
	//
	// Images are handled above and are NOT binary as far as this tool is
	// concerned — they are read with OCR.
	if kind := binaryFileKind(filePath); kind != "" {
		return NewTextErrorResponse(fmt.Sprintf(
			"%s is a binary file (%s), so its contents are not text and were not read.\n\n"+
				"Dumping raw bytes here would fill the conversation with unreadable data and "+
				"cost tokens on every later turn.\n\n"+
				"If you need to know what is INSIDE it, use bash with a tool built for that: "+
				"`file`, `strings`, `xxd | head`, `7z l`, `readelf`, or `binwalk`. "+
				"Run /arsenal to see which of those this machine has.", filePath, kind)), nil
	}

	// Read the file content
	content, lineCount, err := readTextFile(filePath, params.Offset.Int(), params.Limit.Int())
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error reading file: %w", err)
	}

	// GORILLA FIX (2026-08-19), audit: an empty result must say WHY it is
	// empty. Both of these used to return "<file>\n\n</file>" — a shape that
	// looks like a failed read, and that a model reports as "the file appears
	// to be empty" whichever of the two it actually was.
	if strings.TrimSpace(content) == "" {
		if fileInfo.Size() == 0 {
			return NewTextResponse(fmt.Sprintf(
				"%s is empty (0 bytes). The read succeeded; there is nothing in the file.",
				filePath)), nil
		}
		if params.Offset.Int() > 0 {
			// readTextFile resets its counter to the requested offset, so it
			// cannot report the REAL length here — and the real length is the
			// entire point of this message. Counted separately, on this error
			// path only, where one extra pass costs nothing.
			total := countFileLines(filePath)
			return NewTextErrorResponse(fmt.Sprintf(
				"%s has %d line(s), so offset %d is past the end of it and nothing was returned. "+
					"This is NOT an empty file — read it from offset 0, or from a line below %d.",
				filePath, total, params.Offset.Int(), total)), nil
		}
	}

	notifyLspOpenFile(ctx, filePath, v.lspClients)
	output := "<file>\n"
	// Format the output with line numbers
	output += addLineNumbers(content, params.Offset.Int()+1)

	// Add a note if the content was truncated
	if lineCount > params.Offset.Int()+len(strings.Split(content, "\n")) {
		output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)",
			params.Offset.Int()+len(strings.Split(content, "\n")))
	}
	output += "\n</file>\n"
	output += getDiagnostics(filePath, v.lspClients)
	recordFileRead(filePath)
	return WithResponseMetadata(
		NewTextResponse(output),
		ViewResponseMetadata{
			FilePath: filePath,
			Content:  content,
		},
	), nil
}

func addLineNumbers(content string, startLine int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")

	var result []string
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")

		lineNum := i + startLine
		numStr := fmt.Sprintf("%d", lineNum)

		if len(numStr) >= 6 {
			result = append(result, fmt.Sprintf("%s|%s", numStr, line))
		} else {
			paddedNum := fmt.Sprintf("%6s", numStr)
			result = append(result, fmt.Sprintf("%s|%s", paddedNum, line))
		}
	}

	return strings.Join(result, "\n")
}

func readTextFile(filePath string, offset, limit int) (string, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	lineCount := 0

	scanner := NewLineScanner(file)
	if offset > 0 {
		for lineCount < offset && scanner.Scan() {
			lineCount++
		}
		if err = scanner.Err(); err != nil {
			return "", 0, err
		}
	}

	if offset == 0 {
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return "", 0, err
		}
	}

	var lines []string
	lineCount = offset

	for scanner.Scan() && len(lines) < limit {
		lineCount++
		lineText := scanner.Text()
		if len(lineText) > MaxLineLength {
			lineText = lineText[:MaxLineLength] + "..."
		}
		lines = append(lines, lineText)
	}

	// Continue scanning to get total line count
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return "", 0, err
	}

	return strings.Join(lines, "\n"), lineCount, nil
}

func isImageFile(filePath string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return true, "JPEG"
	case ".png":
		return true, "PNG"
	case ".gif":
		return true, "GIF"
	case ".bmp":
		return true, "BMP"
	case ".svg":
		return true, "SVG"
	case ".webp":
		return true, "WebP"
	default:
		return false, ""
	}
}

type LineScanner struct {
	scanner *bufio.Scanner
}

func NewLineScanner(r io.Reader) *LineScanner {
	return &LineScanner{
		scanner: bufio.NewScanner(r),
	}
}

func (s *LineScanner) Scan() bool {
	return s.scanner.Scan()
}

func (s *LineScanner) Text() string {
	return s.scanner.Text()
}

func (s *LineScanner) Err() error {
	return s.scanner.Err()
}

// binaryFileKind names a binary payload, or "" if the file looks like text.
//
// Deliberately conservative: it reads only the first chunk and looks for the
// one thing that never appears in text, a NUL byte, plus a handful of magic
// numbers for the formats people actually try to open by mistake. A false
// positive here refuses a readable file, which is worse than the bytes it
// prevents, so the test is narrow on purpose.
func binaryFileKind(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}
	head := buf[:n]

	for magic, name := range map[string]string{
		"\x7fELF":            "ELF executable or library",
		"MZ":                 "Windows executable",
		"PK\x03\x04":         "zip-based archive (zip, jar, docx, apk)",
		"\x1f\x8b":           "gzip archive",
		"BZh":                "bzip2 archive",
		"\xfd7zXZ":           "xz archive",
		"(\xb5/\xfd":         "zstd archive",
		"7z\xbc\xaf\x27\x1c": "7-zip archive",
		"!<arch>":            "ar archive (.deb, .a)",
		"%PDF":               "PDF",
		"\x00asm":            "WebAssembly module",
		"SQLite format 3":    "SQLite database",
	} {
		if bytes.HasPrefix(head, []byte(magic)) {
			return name
		}
	}

	// A NUL byte in the first chunk is the classic test, and the one every
	// other tool uses. Valid UTF-8 text never contains one.
	if bytes.IndexByte(head, 0) >= 0 {
		return "binary data"
	}
	// Not valid UTF-8 at all, and not obviously a known format.
	if !utf8.Valid(head) && n > 16 {
		return "not valid UTF-8 text"
	}
	return ""
}

// countFileLines returns the number of lines in a file, or 0 if it cannot be read.
// Used only to make an error message truthful, never on the success path.
func countFileLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := NewLineScanner(f)
	for sc.Scan() {
		n++
	}
	return n
}
