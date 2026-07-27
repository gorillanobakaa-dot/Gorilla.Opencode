package layout

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// sizeSpy records the size its parent container hands it.
type sizeSpy struct{ w, h int }

func (s *sizeSpy) Init() tea.Cmd                       { return nil }
func (s *sizeSpy) Update(tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s *sizeSpy) SetSize(w, h int) tea.Cmd            { s.w, s.h = w, h; return nil }
func (s *sizeSpy) GetSize() (int, int)                 { return s.w, s.h }
func (s *sizeSpy) BindingKeys() []key.Binding          { return nil }
func (s *sizeSpy) View() string                        { return "" }

// VerticalChrome must match what SetSize actually subtracts, otherwise a caller
// sizing a container from a desired CONTENT height squeezes the content to zero
// rows — which is how the message input box was made to disappear entirely.
func TestVerticalChromeMatchesSetSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ContainerOption
	}{
		{"bare", nil},
		{"top border only (the editor's config)", []ContainerOption{WithBorder(true, false, false, false)}},
		{"all borders", []ContainerOption{WithBorder(true, true, true, true)}},
		{"padding all round", []ContainerOption{WithPadding(1, 1, 1, 1)}},
		{"padding + top border", []ContainerOption{WithPadding(2, 0, 1, 0), WithBorder(true, false, false, false)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &sizeSpy{}
			c := NewContainer(spy, tc.opts...)

			const askHeight = 10
			c.SetSize(40, askHeight)

			chrome := c.VerticalChrome()
			if want := askHeight - chrome; spy.h != want {
				t.Errorf("content height = %d, want %d (chrome reported %d)", spy.h, want, chrome)
			}

			// The real invariant: sizing from a desired CONTENT height of 1 must
			// leave the content at least 1 row — never 0 (invisible).
			c.SetSize(40, 1+chrome)
			if spy.h < 1 {
				t.Errorf("content squeezed to %d rows when asked for 1+chrome — the widget would be invisible", spy.h)
			}
		})
	}
}
