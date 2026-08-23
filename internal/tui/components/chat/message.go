package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/diff"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type uiMessageType int

const (
	userMessageType uiMessageType = iota
	assistantMessageType
	toolMessageType

	maxResultHeight = 10
)

type uiMessage struct {
	ID          string
	messageType uiMessageType
	position    int
	height      int
	content     string
}

func toMarkdown(content string, focused bool, width int) string {
	r := styles.GetMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return rendered
}

// renderMessageFlat is the message as ordinary terminal text: no fill, no border,
// no bubble.
//
// GORILLA OVERRIDE. Outside the alternate screen a message is not a panel — it is a
// paragraph printed into the terminal's scrollback, which will still be there in an
// hour and can be selected with the mouse. Wrapping it in a coloured slab with a
// thick left rule made every reply look like a widget floating on the terminal's own
// background, which is what "get rid of the ridiculous greys" was about.
//
// The two speakers are told apart by TYPE rather than by decoration, the way agy and
// Gemini CLI do it: what you typed is prefixed and emphasised, what the model said
// is plain prose. That survives being copied into a text file, which a background
// colour does not.
func renderMessageFlat(msg string, isUser bool, width int, info ...string) string {
	t := theme.CurrentTheme()

	var body string
	if isUser {
		// Deliberately NOT Markdown. Running a question through a document renderer
		// re-indents it and styles stray punctuation as formatting, so what you see
		// is not quite what you typed. A prefix per line keeps a multi-line question
		// visually attached to itself.
		style := lipgloss.NewStyle().Bold(true).Foreground(t.Secondary())
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		for i, l := range lines {
			lines[i] = style.Render("> " + l)
		}
		body = strings.Join(lines, "\n")
	} else {
		// The model's answer keeps Markdown: code blocks and lists are the point of
		// it. No background is stamped on, so glamour's own colours sit directly on
		// the terminal.
		body = strings.TrimSuffix(toMarkdown(msg, false, width), "\n")
	}

	// Hard-wrap as a final guarantee, not as formatting.
	//
	// Dropping the panel also dropped the lipgloss Width that used to bound every
	// line, and an existing test caught the consequence immediately: a 440-character
	// unbreakable token — a long URL, a base64 blob, a minified line — came out 441
	// columns wide. Markdown wrapping does not help, because there is no space to
	// break on. An over-wide line is not merely ugly here: the terminal wraps it into
	// rows the renderer did not count, so every erase after it lands in the wrong
	// place.
	//
	// ansi.Hardwrap rather than lipgloss's Width because the text already carries
	// colour sequences, and a wrapper that cannot see them splits escapes in half.
	parts := make([]string, 0, 1+len(info))
	for _, part := range append([]string{body}, info...) {
		parts = append(parts, ansi.Hardwrap(part, width, false))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderMessage(msg string, isUser bool, isFocused bool, width int, info ...string) string {
	t := theme.CurrentTheme()

	if !config.AlternateScreenEnabled() {
		return renderMessageFlat(msg, isUser, width, info...)
	}

	style := styles.BaseStyle().
		Width(width - 1).
		BorderLeft(true).
		Foreground(t.TextMuted()).
		BorderForeground(t.Primary()).
		BorderStyle(lipgloss.ThickBorder())

	if isUser {
		style = style.BorderForeground(t.Secondary())
	}

	// Apply markdown formatting and handle background color
	parts := []string{
		styles.ApplyPanelBackground(toMarkdown(msg, isFocused, width)),
	}

	// Remove newline at the end
	parts[0] = strings.TrimSuffix(parts[0], "\n")
	if len(info) > 0 {
		parts = append(parts, info...)
	}

	rendered := style.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			parts...,
		),
	)

	return rendered
}

func renderUserMessage(msg message.Message, isFocused bool, width int, position int) uiMessage {
	var styledAttachments []string
	t := theme.CurrentTheme()
	attachmentStyles := styles.BaseStyle().
		MarginLeft(1).
		Background(t.TextMuted()).
		Foreground(t.Text())
	for _, attachment := range msg.BinaryContent() {
		file := filepath.Base(attachment.Path)
		var filename string
		if len(file) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, file[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, file)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	// GORILLA OVERRIDE: a clock on every message, so a timeline can be read live
	// rather than only after exporting. Off is a real choice — see
	// config.Extras — but it costs nothing, because created_at has been recorded
	// since the first migration.
	info := []string{}
	if config.ExtraEnabled("extras-timestamps-show") {
		if ts := messageTime(msg.CreatedAt); ts != "" {
			info = append(info, styles.BaseStyle().
				Width(width-1).
				Foreground(t.TextMuted()).
				Render(" "+ts))
		}
	}

	content := ""
	if len(styledAttachments) > 0 {
		attachmentContent := styles.BaseStyle().Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...))
		content = renderMessage(msg.Content().String(), true, isFocused, width, append([]string{attachmentContent}, info...)...)
	} else {
		content = renderMessage(msg.Content().String(), true, isFocused, width, info...)
	}
	userMsg := uiMessage{
		ID:          msg.ID,
		messageType: userMessageType,
		position:    position,
		height:      lipgloss.Height(content),
		content:     content,
	}
	return userMsg
}

// Returns multiple uiMessages because of the tool calls
func renderAssistantMessage(
	msg message.Message,
	msgIndex int,
	allMessages []message.Message, // we need this to get tool results and the user message
	messagesService message.Service, // We need this to get the task tool messages
	focusedUIMessageId string,
	isSummary bool,
	width int,
	position int,
	// GORILLA OVERRIDE: in scrollback mode the reasoning has ALREADY been
	// printed into the terminal, line by line, as it arrived. Rendering the
	// quote here too would print the whole block a second time.
	skipReasoning bool,
) []uiMessage {
	messages := []uiMessage{}
	content := msg.Content().String()
	thinking := msg.IsThinking()
	thinkingContent := msg.ReasoningContent().Thinking
	finished := msg.IsFinished()
	finishData := msg.FinishPart()
	info := []string{}

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	// Add finish info if available
	if finished {
		switch finishData.Reason {
		case message.FinishReasonEndTurn:
			took := formatTimestampDiff(msg.CreatedAt, finishData.Time)
			info = append(info, baseStyle.
				Width(width-1).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, took)),
			)
		case message.FinishReasonCanceled:
			info = append(info, baseStyle.
				Width(width-1).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "canceled")),
			)
		case message.FinishReasonError:
			info = append(info, baseStyle.
				Width(width-1).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "error")),
			)
		case message.FinishReasonPermissionDenied:
			info = append(info, baseStyle.
				Width(width-1).
				Foreground(t.TextMuted()).
				Render(fmt.Sprintf(" %s (%s)", models.SupportedModels[msg.Model].Name, "permission denied")),
			)
		}
	}
	// GORILLA OVERRIDE: same clock on the assistant side. Prepended so the time
	// reads first, before the model name and duration already shown there.
	if config.ExtraEnabled("extras-timestamps-show") {
		if ts := messageTime(msg.CreatedAt); ts != "" {
			info = append([]string{baseStyle.
				Width(width - 1).
				Foreground(t.TextMuted()).
				Render(" " + ts)}, info...)
		}
	}

	// GORILLA FIX: this used to read
	//     content != "" || (finished && finishData.Reason == EndTurn)
	// so a turn that finished for ANY OTHER reason with no text rendered
	// absolutely nothing — no error, no model line, not even a timestamp. In
	// scrollback mode that is total silence: the reasoning had already been
	// printed live, then the close marker, then nothing at all, and the prompt
	// came back. There was no way to tell a failed turn from a finished one.
	//
	// A turn that ends without an answer is exactly when the user most needs to
	// be told why. Every finish reason now renders, and the empty-content
	// placeholder names the reason instead of the generic "finished".
	if content != "" || finished {
		if content == "" {
			switch finishData.Reason {
			case message.FinishReasonError:
				// GORILLA FIX: show the provider's own words, and when we have
				// them do NOT also guess at the cause.
				//
				// The "context might be over 100%" line was written when a failed
				// turn left nothing behind — a guess beat silence. Now that the
				// real error is stored, that guess sits directly above an answer
				// that often contradicts it: observed 2026-08-05 with the footer
				// reading "context 0 (0%)" while the text speculated about an
				// oversized request, immediately above a 404 saying the model was
				// not enabled for the account. A confident wrong explanation next
				// to the right one is worse than no explanation.
				//
				// So: details when we have them, the guess only when we do not.
				// Fenced, so a URL or JSON fragment is not mangled by markdown and
				// is visibly the machine's words rather than ours.
				// GORILLA OVERRIDE (2026-08-18): bookend the error header with
				// util.NoticeDeco, the same 🦍⚠️ ⚠️ 🦍 the cold-start notice uses in
				// the transcript, so a Gorilla notice reads the same whatever path
				// it came through — provider error here, echo elsewhere.
				// The closing bookend goes AFTER the provider's text, not after the
				// header — it closes the whole notice, so the reader can see where
				// it ends. A header sandwiched between both marks looked like the
				// message had finished before the error had even been printed.
				if d := strings.TrimSpace(finishData.Details); d != "" {
					content = util.NoticeDeco + " *The model returned an error and produced no answer:*" +
						"\n\n```\n" + d + "\n```\n\n" + util.NoticeDeco
				} else {
					content = util.NoticeDeco + " *The model returned an error and produced no answer. " +
						"If the context percentage in the footer is over 100%, the request " +
						"was almost certainly rejected for being too large.* " + util.NoticeDeco
				}
			case message.FinishReasonMaxTokens:
				content = "*Stopped at the model's output limit before finishing the answer.*"
			case message.FinishReasonCanceled:
				content = "*Canceled — no answer was produced.*"
			case message.FinishReasonPermissionDenied:
				content = "*Permission denied, so nothing was run and no answer was produced.*"
			case message.FinishReasonToolUse:
				// GORILLA FIX: this is the COMMON case, and it was falling through
				// to "Finished without output" — which reads like a failure for
				// what is actually the model working normally. A turn that ends in
				// tool_use with no prose means the model said nothing and went
				// straight to running something; the tool call and its result are
				// rendered directly below, so the turn is not empty at all.
				content = "*No message — the model went straight to running a tool (below).*"
			default:
				content = "*Finished without output*"
			}
		}
		// GORILLA OVERRIDE: show the reasoning next to the answer it produced.
		// Previously the thinking was visible only while streaming and then
		// vanished the moment the turn finished, so the one thing that explains
		// how a conclusion was reached was the one thing you could not go back and
		// read. Free to display: it has already been generated and paid for.
		if !skipReasoning && config.ExtraEnabled("extras-reasoning-show") {
			if q := reasoningQuote(thinkingContent); q != "" {
				content = q + "\n\n" + content
			}
		}
		if isSummary {
			info = append(info, baseStyle.Width(width-1).Foreground(t.TextMuted()).Render(" (summary)"))
		}

		content = renderMessage(content, false, true, width, info...)
		messages = append(messages, uiMessage{
			ID:          msg.ID,
			messageType: assistantMessageType,
			position:    position,
			height:      lipgloss.Height(content),
			content:     content,
		})
		position += messages[0].height
		position++ // for the space
	} else if thinking && thinkingContent != "" && !skipReasoning {
		// GORILLA FIX: this branch was NOT gated by skipReasoning, unlike the one
		// above, so a message that was still thinking and had no answer text yet
		// had its reasoning pushed through renderMessage -> toMarkdown -> glamour.
		// Two things went wrong with that in scrollback mode:
		//   1. glamour turns a literal "---" in the model's reasoning into a
		//      horizontal rule (styles/markdown.go HorizontalRule), so rules
		//      appeared inside the printed thinking.
		//   2. it re-wrapped and re-flowed text that had already been printed
		//      verbatim, line by line, by the printer.
		// The reasoning is already in the terminal by this point. Rendering it
		// again here can only disagree with what was printed.
		content = renderMessage(thinkingContent, false, msg.ID == focusedUIMessageId, width)
	}

	// GORILLA OVERRIDE: tool calls can be hidden, but hiding them saves NOTHING —
	// the calls already happened and were already billed. The switch exists for
	// people who want a quieter screen, and it is labelled "free" precisely so
	// nobody turns it off believing it reduces their bill.
	toolCalls := msg.ToolCalls()
	if !config.ExtraEnabled("extras-toolcalls-show") {
		toolCalls = nil
	}
	for i, toolCall := range toolCalls {
		toolCallContent := renderToolMessage(
			toolCall,
			allMessages,
			messagesService,
			focusedUIMessageId,
			false,
			width,
			i+1,
		)
		messages = append(messages, toolCallContent)
		position += toolCallContent.height
		position++ // for the space
	}
	return messages
}

func findToolResponse(toolCallID string, futureMessages []message.Message) *message.ToolResult {
	for _, msg := range futureMessages {
		for _, result := range msg.ToolResults() {
			if result.ToolCallID == toolCallID {
				return &result
			}
		}
	}
	return nil
}

func toolName(name string) string {
	switch name {
	case agent.AgentToolName:
		return "Task"
	case tools.BashToolName:
		return "Bash"
	case tools.EditToolName:
		return "Edit"
	case tools.FetchToolName:
		return "Fetch"
	case tools.FindToolName:
		return "Find"
	// GORILLA OVERRIDE: glob/grep/ls were replaced by the find tool. The
	// literals stay so tool calls recorded in OLD sessions keep rendering
	// with their proper labels; nothing registers these names any more.
	case "glob":
		return "Glob"
	case "grep":
		return "Grep"
	case "ls":
		return "List"
	case tools.ViewToolName:
		return "View"
	case tools.WriteToolName:
		return "Write"
	case tools.PatchToolName:
		return "Patch"
	}
	return name
}

func getToolAction(name string) string {
	switch name {
	case agent.AgentToolName:
		return "Preparing prompt..."
	case agent.ResearchToolName:
		// GORILLA FIX (2026-08-23): a research run had no case at all, so the
		// longest operation in the program showed the generic "Working...".
		//
		// The wording is the owner's, and it is here rather than on the
		// permission dialog on purpose. A status line is where personality is
		// free: nobody is deciding anything while it spins. A joke on a consent
		// dialog reads as decoration and gets clicked through, which is the
		// exact failure the new web_search wording exists to avoid.
		return "Analyzing this... with science..."
	case tools.WebSearchToolName:
		return "Searching the web..."
	case tools.BashToolName:
		return "Building command..."
	case tools.EditToolName:
		return "Preparing edit..."
	case tools.FetchToolName:
		return "Writing fetch..."
	case tools.FindToolName:
		return "Searching..."
	case "glob":
		return "Finding files..."
	case "grep":
		return "Searching content..."
	case "ls":
		return "Listing directory..."
	case tools.ViewToolName:
		return "Reading file..."
	case tools.WriteToolName:
		return "Preparing write..."
	case tools.PatchToolName:
		return "Preparing patch..."
	}
	return "Working..."
}

// renders params, params[0] (params[1]=params[2] ....)
func renderParams(paramsWidth int, params ...string) string {
	if len(params) == 0 {
		return ""
	}
	mainParam := params[0]
	if len(mainParam) > paramsWidth {
		mainParam = mainParam[:paramsWidth-3] + "..."
	}

	if len(params) == 1 {
		return mainParam
	}
	otherParams := params[1:]
	// create pairs of key/value
	// if odd number of params, the last one is a key without value
	if len(otherParams)%2 != 0 {
		otherParams = append(otherParams, "")
	}
	parts := make([]string, 0, len(otherParams)/2)
	for i := 0; i < len(otherParams); i += 2 {
		key := otherParams[i]
		value := otherParams[i+1]
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	partsRendered := strings.Join(parts, ", ")
	remainingWidth := paramsWidth - lipgloss.Width(partsRendered) - 5 // for the space
	if remainingWidth < 30 {
		// No space for the params, just show the main
		return mainParam
	}

	if len(parts) > 0 {
		mainParam = fmt.Sprintf("%s (%s)", mainParam, strings.Join(parts, ", "))
	}

	return ansi.Truncate(mainParam, paramsWidth, "...")
}

func removeWorkingDirPrefix(path string) string {
	wd := config.WorkingDirectory()
	if strings.HasPrefix(path, wd) {
		path = strings.TrimPrefix(path, wd)
	}
	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}
	if strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	if strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	return path
}

func renderToolParams(paramWidth int, toolCall message.ToolCall) string {
	params := ""
	switch toolCall.Name {
	case agent.AgentToolName:
		var params agent.AgentParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		prompt := strings.ReplaceAll(params.Prompt, "\n", " ")
		return renderParams(paramWidth, prompt)
	case tools.BashToolName:
		var params tools.BashParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		command := strings.ReplaceAll(params.Command, "\n", " ")
		return renderParams(paramWidth, command)
	case tools.EditToolName:
		var params tools.EditParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		return renderParams(paramWidth, filePath)
	case tools.FetchToolName:
		var params tools.FetchParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		url := params.URL
		toolParams := []string{
			url,
		}
		if params.Format != "" {
			toolParams = append(toolParams, "format", params.Format)
		}
		if params.Timeout != 0 {
			toolParams = append(toolParams, "timeout", (time.Duration(params.Timeout) * time.Second).String())
		}
		return renderParams(paramWidth, toolParams...)
	case tools.FindToolName:
		var params tools.FindParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		first := params.Query
		if first == "" {
			first = params.Glob
		}
		if first == "" {
			first = params.Path
		}
		toolParams := []string{first}
		if params.Query != "" && params.Path != "" {
			toolParams = append(toolParams, "path", removeWorkingDirPrefix(params.Path))
		}
		if params.Query != "" && params.Glob != "" {
			toolParams = append(toolParams, "glob", params.Glob)
		}
		if params.Type != "" {
			toolParams = append(toolParams, "type", params.Type)
		}
		if params.View != "" {
			toolParams = append(toolParams, "view", params.View)
		}
		if params.Fuzzy {
			toolParams = append(toolParams, "fuzzy", "true")
		}
		if params.Recent {
			toolParams = append(toolParams, "recent", "true")
		}
		if params.ModifiedOnly {
			toolParams = append(toolParams, "modified_only", "true")
		}
		if params.FilesOnly {
			toolParams = append(toolParams, "files_only", "true")
		}
		return renderParams(paramWidth, toolParams...)
	// GORILLA OVERRIDE: glob/grep/ls exist only in old session transcripts now.
	// Their inputs are decoded into anonymous structs so history renders
	// without keeping the retired tool implementations compiled.
	case "glob":
		var params struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		json.Unmarshal([]byte(toolCall.Input), &params)
		toolParams := []string{params.Pattern}
		if params.Path != "" {
			toolParams = append(toolParams, "path", params.Path)
		}
		return renderParams(paramWidth, toolParams...)
	case "grep":
		var params struct {
			Pattern     string `json:"pattern"`
			Path        string `json:"path"`
			Include     string `json:"include"`
			LiteralText bool   `json:"literal_text"`
		}
		json.Unmarshal([]byte(toolCall.Input), &params)
		toolParams := []string{params.Pattern}
		if params.Path != "" {
			toolParams = append(toolParams, "path", params.Path)
		}
		if params.Include != "" {
			toolParams = append(toolParams, "include", params.Include)
		}
		if params.LiteralText {
			toolParams = append(toolParams, "literal", "true")
		}
		return renderParams(paramWidth, toolParams...)
	case "ls":
		var params struct {
			Path string `json:"path"`
		}
		json.Unmarshal([]byte(toolCall.Input), &params)
		path := params.Path
		if path == "" {
			path = "."
		}
		return renderParams(paramWidth, path)
	case tools.ViewToolName:
		var params tools.ViewParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		toolParams := []string{
			filePath,
		}
		if params.Limit != 0 {
			toolParams = append(toolParams, "limit", fmt.Sprintf("%d", params.Limit))
		}
		if params.Offset != 0 {
			toolParams = append(toolParams, "offset", fmt.Sprintf("%d", params.Offset))
		}
		return renderParams(paramWidth, toolParams...)
	case tools.WriteToolName:
		var params tools.WriteParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		filePath := removeWorkingDirPrefix(params.FilePath)
		return renderParams(paramWidth, filePath)
	default:
		input := strings.ReplaceAll(toolCall.Input, "\n", " ")
		params = renderParams(paramWidth, input)
	}
	return params
}

func truncateHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	return content
}

func renderToolResponse(toolCall message.ToolCall, response message.ToolResult, width int) string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	if response.IsError {
		errContent := fmt.Sprintf("Error: %s", strings.ReplaceAll(response.Content, "\n", " "))
		errContent = ansi.Truncate(errContent, width-1, "...")
		return baseStyle.
			Width(width).
			Foreground(t.Error()).
			Render(errContent)
	}

	resultContent := truncateHeight(response.Content, maxResultHeight)
	switch toolCall.Name {
	case agent.AgentToolName:
		return styles.ApplyPanelBackground(toMarkdown(resultContent, false, width))
	case tools.BashToolName:
		resultContent = fmt.Sprintf("```bash\n%s\n```", resultContent)
		return styles.ApplyPanelBackground(toMarkdown(resultContent, true, width))
	case tools.EditToolName:
		metadata := tools.EditResponseMetadata{}
		json.Unmarshal([]byte(response.Metadata), &metadata)
		truncDiff := truncateHeight(metadata.Diff, maxResultHeight)
		formattedDiff, _ := diff.FormatDiff(truncDiff, diff.WithTotalWidth(width))
		return formattedDiff
	case tools.FetchToolName:
		var params tools.FetchParams
		json.Unmarshal([]byte(toolCall.Input), &params)
		mdFormat := "markdown"
		switch params.Format {
		case "text":
			mdFormat = "text"
		case "html":
			mdFormat = "html"
		}
		resultContent = fmt.Sprintf("```%s\n%s\n```", mdFormat, resultContent)
		return styles.ApplyPanelBackground(toMarkdown(resultContent, true, width))
	case tools.FindToolName:
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case "glob", "grep", "ls": // retired tools, still present in old transcripts
		return baseStyle.Width(width).Foreground(t.TextMuted()).Render(resultContent)
	case tools.ViewToolName:
		metadata := tools.ViewResponseMetadata{}
		json.Unmarshal([]byte(response.Metadata), &metadata)
		ext := filepath.Ext(metadata.FilePath)
		if ext == "" {
			ext = ""
		} else {
			ext = strings.ToLower(ext[1:])
		}
		resultContent = fmt.Sprintf("```%s\n%s\n```", ext, truncateHeight(metadata.Content, maxResultHeight))
		return styles.ApplyPanelBackground(toMarkdown(resultContent, true, width))
	case tools.WriteToolName:
		params := tools.WriteParams{}
		json.Unmarshal([]byte(toolCall.Input), &params)
		metadata := tools.WriteResponseMetadata{}
		json.Unmarshal([]byte(response.Metadata), &metadata)
		ext := filepath.Ext(params.FilePath)
		if ext == "" {
			ext = ""
		} else {
			ext = strings.ToLower(ext[1:])
		}
		resultContent = fmt.Sprintf("```%s\n%s\n```", ext, truncateHeight(params.Content, maxResultHeight))
		return styles.ApplyPanelBackground(toMarkdown(resultContent, true, width))
	default:
		resultContent = fmt.Sprintf("```text\n%s\n```", resultContent)
		return styles.ApplyPanelBackground(toMarkdown(resultContent, true, width))
	}
}

func renderToolMessage(
	toolCall message.ToolCall,
	allMessages []message.Message,
	messagesService message.Service,
	focusedUIMessageId string,
	nested bool,
	width int,
	position int,
) uiMessage {
	if nested {
		width = width - 3
	}

	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	style := baseStyle.
		Width(width - 1).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		PaddingLeft(1).
		BorderForeground(t.TextMuted())

	response := findToolResponse(toolCall.ID, allMessages)
	toolNameText := baseStyle.Foreground(t.TextMuted()).
		Render(fmt.Sprintf("%s: ", toolName(toolCall.Name)))

	if !toolCall.Finished {
		// Get a brief description of what the tool is doing
		toolAction := getToolAction(toolCall.Name)

		progressText := baseStyle.
			Width(width - 2 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(fmt.Sprintf("%s", toolAction))

		content := style.Render(lipgloss.JoinHorizontal(lipgloss.Left, toolNameText, progressText))
		toolMsg := uiMessage{
			messageType: toolMessageType,
			position:    position,
			height:      lipgloss.Height(content),
			content:     content,
		}
		return toolMsg
	}

	params := renderToolParams(width-2-lipgloss.Width(toolNameText), toolCall)
	responseContent := ""
	if response != nil {
		responseContent = renderToolResponse(toolCall, *response, width-2)
		responseContent = strings.TrimSuffix(responseContent, "\n")
	} else {
		responseContent = baseStyle.
			Italic(true).
			Width(width - 2).
			Foreground(t.TextMuted()).
			Render("Waiting for response...")
	}

	parts := []string{}
	if !nested {
		formattedParams := baseStyle.
			Width(width - 2 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(params)

		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Left, toolNameText, formattedParams))
	} else {
		prefix := baseStyle.
			Foreground(t.TextMuted()).
			Render(" \\_ ")
		formattedParams := baseStyle.
			Width(width - 2 - lipgloss.Width(toolNameText)).
			Foreground(t.TextMuted()).
			Render(params)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Left, prefix, toolNameText, formattedParams))
	}

	if toolCall.Name == agent.AgentToolName {
		taskMessages, _ := messagesService.List(context.Background(), toolCall.ID)
		toolCalls := []message.ToolCall{}
		for _, v := range taskMessages {
			toolCalls = append(toolCalls, v.ToolCalls()...)
		}
		for _, call := range toolCalls {
			rendered := renderToolMessage(call, []message.Message{}, messagesService, focusedUIMessageId, true, width, 0)
			parts = append(parts, rendered.content)
		}
	}
	if responseContent != "" && !nested {
		parts = append(parts, responseContent)
	}

	content := style.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			parts...,
		),
	)
	if nested {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			parts...,
		)
	}
	toolMsg := uiMessage{
		messageType: toolMessageType,
		position:    position,
		height:      lipgloss.Height(content),
		content:     content,
	}
	return toolMsg
}

// Helper function to format the time difference between two Unix timestamps
// formatTimestampDiff renders how long a turn took.
//
// GORILLA OVERRIDE: the unit was wrong. Both arguments are UNIX SECONDS —
// messages.created_at is written by a SQLite trigger using strftime('%s'), and
// the Finish part's Time is seconds too — but this divided by 1000 as though they
// were milliseconds. Every duration ever shown was therefore 1000x too small: a
// 45-second turn displayed as "45ms", which made the agent look instantaneous and
// made the number useless for spotting a slow model.
//
// The cause was a comment. Three places in the initial migration described these
// columns as "Unix timestamp in milliseconds"; they never were. Those comments are
// now corrected, and this is what believing them cost.
func formatTimestampDiff(start, end int64) string {
	diffSeconds := float64(end - start)
	if diffSeconds < 0 {
		diffSeconds = 0
	}
	if diffSeconds < 1 {
		return "<1s"
	}
	if diffSeconds < 60 {
		return fmt.Sprintf("%.0fs", diffSeconds)
	}
	return fmt.Sprintf("%.1fm", diffSeconds/60)
}

// messageTime is the per-message clock shown when the timestamps extra is on.
//
// Time of day only, not the full date: at 80 columns a date on every row is
// noise, and a session almost always sits inside one day. The exported
// transcript carries the full date and an offset from the session start, which is
// the right place for a record that will be read weeks later.
func messageTime(unixSeconds int64) string {
	if unixSeconds <= 0 {
		return ""
	}
	return time.Unix(unixSeconds, 0).Format("15:04:05")
}

// reasoningQuote turns stored reasoning into a markdown blockquote so it reads as
// an aside rather than as the answer. Matches the shape /export uses.
func reasoningQuote(thinking string) string {
	thinking = strings.TrimSpace(thinking)
	if thinking == "" {
		return ""
	}
	lines := strings.Split(thinking, "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return "> **thinking**\n>\n" + strings.Join(lines, "\n")
}
