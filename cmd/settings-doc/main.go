// GORILLA OVERRIDE: generates docs/SETTINGS.md from config.Settings.
//
// The doc is generated rather than written because a hand-maintained reference
// drifts on the first change. A test asserts the checked-in file matches this
// output, so a new setting cannot land undocumented.
//
//	go run ./cmd/settings-doc > docs/SETTINGS.md
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/config"
)

func main() {
	var b strings.Builder

	b.WriteString("# Settings reference\n\n")
	b.WriteString("Generated from `internal/config/settings.go` by `go run ./cmd/settings-doc`.\n")
	b.WriteString("Do not edit by hand — a test compares this file against the registry.\n\n")
	b.WriteString("Open the same list in the app with **`/settings`**, where every row shows its\n")
	b.WriteString("current value, what it accepts, and its default.\n\n")

	for _, g := range config.GroupOrder {
		var rows []*config.Setting
		for i := range config.Settings {
			if config.Settings[i].Group == g {
				rows = append(rows, &config.Settings[i])
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g)
		for _, s := range rows {
			fmt.Fprintf(&b, "### %s\n\n", s.Name)
			fmt.Fprintf(&b, "%s\n\n", s.Layman)
			if s.Kind == config.KindBool {
				fmt.Fprintf(&b, "- **ON:** %s\n- **OFF:** %s\n", s.WhenOn, s.WhenOff)
			}
			fmt.Fprintf(&b, "- **Setting:** `%s`\n", s.ID)
			fmt.Fprintf(&b, "- **Type:** %s\n", s.Kind)
			fmt.Fprintf(&b, "- **Accepts:** %s\n", config.SettingRange(s))
			fmt.Fprintf(&b, "- **Default:** `%s`\n", config.FormatSettingValue(s.Default))
			if s.Unit != "" {
				fmt.Fprintf(&b, "- **Unit:** %s\n", s.Unit)
			}
			if s.ReadOnly {
				fmt.Fprintf(&b, "- **Read-only:** %s\n", s.ReadOnlyWhy)
			}
			if s.Restart {
				b.WriteString("- **Takes effect:** next launch\n")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Owned by other commands\n\n")
	b.WriteString("These are deliberately not in `/settings`, so there is one owner per setting:\n\n")
	for _, e := range config.ModelOwnedElsewhere {
		fmt.Fprintf(&b, "- **%s** — %s. Use `%s`.\n", e.Name, e.Why, e.Owner)
	}
	b.WriteString("\n")

	os.Stdout.WriteString(b.String())
}
