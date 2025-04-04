package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/kujtimiihoxha/termai/internal/lsp"
	"github.com/kujtimiihoxha/termai/internal/lsp/protocol"
)

type diagnosticsTool struct {
	lspClients map[string]*lsp.Client
}

const (
	DiagnosticsToolName = "diagnostics"
)

type DiagnosticsParams struct {
	FilePath string `json:"file_path"`
}

func (b *diagnosticsTool) Info() ToolInfo {
	return ToolInfo{
		Name:        DiagnosticsToolName,
		Description: "Get diagnostics for a file and/or project.",
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to get diagnostics for (leave w empty for project diagnostics)",
			},
		},
		Required: []string{},
	}
}

func (b *diagnosticsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DiagnosticsParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	lsps := b.lspClients

	if len(lsps) == 0 {
		return NewTextErrorResponse("no LSP clients available"), nil
	}

	if params.FilePath != "" {
		notifyLspOpenFile(ctx, params.FilePath, lsps)
	}

	output := appendDiagnostics(params.FilePath, lsps)

	return NewTextResponse(output), nil
}

func notifyLspOpenFile(ctx context.Context, filePath string, lsps map[string]*lsp.Client) {
	// Create a channel to receive diagnostic notifications
	diagChan := make(chan struct{}, 1)

	// Register a temporary diagnostic handler for each client
	for _, client := range lsps {
		// Store the original diagnostics map to detect changes
		originalDiags := make(map[protocol.DocumentUri][]protocol.Diagnostic)
		maps.Copy(originalDiags, client.GetDiagnostics())

		// Create a notification handler that will signal when diagnostics are received
		handler := func(params json.RawMessage) {
			lsp.HandleDiagnostics(client, params)
			var diagParams protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(params, &diagParams); err != nil {
				return
			}

			// If this is for our file or we've received any new diagnostics, signal completion
			if diagParams.URI.Path() == filePath || hasDiagnosticsChanged(client.GetDiagnostics(), originalDiags) {
				select {
				case diagChan <- struct{}{}:
					// Signal sent
				default:
					// Channel already has a value, no need to send again
				}
			}
		}

		// Register our temporary handler
		client.RegisterNotificationHandler("textDocument/publishDiagnostics", handler)

		// Open the file
		err := client.OpenFile(ctx, filePath)
		if err != nil {
			// If there's an error opening the file, continue to the next client
			continue
		}
	}

	// Wait for diagnostics with a reasonable timeout
	select {
	case <-diagChan:
		// Diagnostics received
	case <-time.After(10 * time.Second):
		// Timeout after 5 seconds - this is a fallback in case no diagnostics are published
	case <-ctx.Done():
		// Context cancelled
	}

	// Note: We're not unregistering our handler because the Client.RegisterNotificationHandler
	// replaces any existing handler, and we'll be replaced by the original handler when
	// the LSP client is reinitialized or when a new handler is registered.
}

// hasDiagnosticsChanged checks if there are any new diagnostics compared to the original set
func hasDiagnosticsChanged(current, original map[protocol.DocumentUri][]protocol.Diagnostic) bool {
	for uri, diags := range current {
		origDiags, exists := original[uri]
		if !exists || len(diags) != len(origDiags) {
			return true
		}
	}
	return false
}

func appendDiagnostics(filePath string, lsps map[string]*lsp.Client) string {
	fileDiagnostics := []string{}
	projectDiagnostics := []string{}

	// Enhanced format function that includes more diagnostic information
	formatDiagnostic := func(pth string, diagnostic protocol.Diagnostic, source string) string {
		// Base components
		severity := "Info"
		switch diagnostic.Severity {
		case protocol.SeverityError:
			severity = "Error"
		case protocol.SeverityWarning:
			severity = "Warn"
		case protocol.SeverityHint:
			severity = "Hint"
		}

		// Location information
		location := fmt.Sprintf("%s:%d:%d", pth, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)

		// Source information (LSP name)
		sourceInfo := ""
		if diagnostic.Source != "" {
			sourceInfo = diagnostic.Source
		} else if source != "" {
			sourceInfo = source
		}

		// Code information
		codeInfo := ""
		if diagnostic.Code != nil {
			codeInfo = fmt.Sprintf("[%v]", diagnostic.Code)
		}

		// Tags information
		tagsInfo := ""
		if len(diagnostic.Tags) > 0 {
			tags := []string{}
			for _, tag := range diagnostic.Tags {
				switch tag {
				case protocol.Unnecessary:
					tags = append(tags, "unnecessary")
				case protocol.Deprecated:
					tags = append(tags, "deprecated")
				}
			}
			if len(tags) > 0 {
				tagsInfo = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
			}
		}

		// Assemble the full diagnostic message
		return fmt.Sprintf("%s: %s [%s]%s%s %s",
			severity,
			location,
			sourceInfo,
			codeInfo,
			tagsInfo,
			diagnostic.Message)
	}

	for lspName, client := range lsps {
		diagnostics := client.GetDiagnostics()
		if len(diagnostics) > 0 {
			for location, diags := range diagnostics {
				isCurrentFile := location.Path() == filePath

				// Group diagnostics by severity for better organization
				for _, diag := range diags {
					formattedDiag := formatDiagnostic(location.Path(), diag, lspName)

					if isCurrentFile {
						fileDiagnostics = append(fileDiagnostics, formattedDiag)
					} else {
						projectDiagnostics = append(projectDiagnostics, formattedDiag)
					}
				}
			}
		}
	}

	// Sort diagnostics by severity (errors first) and then by location
	sort.Slice(fileDiagnostics, func(i, j int) bool {
		iIsError := strings.HasPrefix(fileDiagnostics[i], "Error")
		jIsError := strings.HasPrefix(fileDiagnostics[j], "Error")
		if iIsError != jIsError {
			return iIsError // Errors come first
		}
		return fileDiagnostics[i] < fileDiagnostics[j] // Then alphabetically
	})

	sort.Slice(projectDiagnostics, func(i, j int) bool {
		iIsError := strings.HasPrefix(projectDiagnostics[i], "Error")
		jIsError := strings.HasPrefix(projectDiagnostics[j], "Error")
		if iIsError != jIsError {
			return iIsError
		}
		return projectDiagnostics[i] < projectDiagnostics[j]
	})

	output := ""

	if len(fileDiagnostics) > 0 {
		output += "\n<file_diagnostics>\n"
		if len(fileDiagnostics) > 10 {
			output += strings.Join(fileDiagnostics[:10], "\n")
			output += fmt.Sprintf("\n... and %d more diagnostics", len(fileDiagnostics)-10)
		} else {
			output += strings.Join(fileDiagnostics, "\n")
		}
		output += "\n</file_diagnostics>\n"
	}

	if len(projectDiagnostics) > 0 {
		output += "\n<project_diagnostics>\n"
		if len(projectDiagnostics) > 10 {
			output += strings.Join(projectDiagnostics[:10], "\n")
			output += fmt.Sprintf("\n... and %d more diagnostics", len(projectDiagnostics)-10)
		} else {
			output += strings.Join(projectDiagnostics, "\n")
		}
		output += "\n</project_diagnostics>\n"
	}

	// Add summary counts
	if len(fileDiagnostics) > 0 || len(projectDiagnostics) > 0 {
		fileErrors := countSeverity(fileDiagnostics, "Error")
		fileWarnings := countSeverity(fileDiagnostics, "Warn")
		projectErrors := countSeverity(projectDiagnostics, "Error")
		projectWarnings := countSeverity(projectDiagnostics, "Warn")

		output += "\n<diagnostic_summary>\n"
		output += fmt.Sprintf("Current file: %d errors, %d warnings\n", fileErrors, fileWarnings)
		output += fmt.Sprintf("Project: %d errors, %d warnings\n", projectErrors, projectWarnings)
		output += "</diagnostic_summary>\n"
	}

	return output
}

// Helper function to count diagnostics by severity
func countSeverity(diagnostics []string, severity string) int {
	count := 0
	for _, diag := range diagnostics {
		if strings.HasPrefix(diag, severity) {
			count++
		}
	}
	return count
}

func NewDiagnosticsTool(lspClients map[string]*lsp.Client) BaseTool {
	return &diagnosticsTool{
		lspClients,
	}
}
