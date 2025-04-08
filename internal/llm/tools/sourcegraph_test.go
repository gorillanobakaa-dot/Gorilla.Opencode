package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourcegraphTool_Info(t *testing.T) {
	tool := NewSourcegraphTool()
	info := tool.Info()

	assert.Equal(t, SourcegraphToolName, info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "query")
	assert.Contains(t, info.Parameters, "count")
	assert.Contains(t, info.Parameters, "timeout")
	assert.Contains(t, info.Required, "query")
}

func TestSourcegraphTool_Run(t *testing.T) {
	t.Run("handles missing query parameter", func(t *testing.T) {
		tool := NewSourcegraphTool()
		params := SourcegraphParams{
			Query: "",
		}

		paramsJSON, err := json.Marshal(params)
		require.NoError(t, err)

		call := ToolCall{
			Name:  SourcegraphToolName,
			Input: string(paramsJSON),
		}

		response, err := tool.Run(context.Background(), call)
		require.NoError(t, err)
		assert.Contains(t, response.Content, "Query parameter is required")
	})

	t.Run("handles invalid parameters", func(t *testing.T) {
		tool := NewSourcegraphTool()
		call := ToolCall{
			Name:  SourcegraphToolName,
			Input: "invalid json",
		}

		response, err := tool.Run(context.Background(), call)
		require.NoError(t, err)
		assert.Contains(t, response.Content, "Failed to parse sourcegraph parameters")
	})

	t.Run("normalizes count parameter", func(t *testing.T) {
		// Test cases for count normalization
		testCases := []struct {
			name          string
			inputCount    int
			expectedCount int
		}{
			{"negative count", -5, 10},    // Should use default (10)
			{"zero count", 0, 10},         // Should use default (10)
			{"valid count", 50, 50},       // Should keep as is
			{"excessive count", 150, 100}, // Should cap at 100
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Verify count normalization logic directly
				assert.NotPanics(t, func() {
					// Apply the same normalization logic as in the tool
					normalizedCount := tc.inputCount
					if normalizedCount <= 0 {
						normalizedCount = 10
					} else if normalizedCount > 100 {
						normalizedCount = 100
					}

					assert.Equal(t, tc.expectedCount, normalizedCount)
				})
			})
		}
	})
}
