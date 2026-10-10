package config

// GORILLA OVERRIDE (2026-10-09): lifecycle hooks are loaded exactly as written,
// rejected loudly when malformed, and never lost by a later config write.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// A malformed hook stops the load, and the message names WHICH hook: its index
// and its command. "invalid config" alone would send the user hunting.
func TestMalformedHooksAreRejectedByIndexAndCommand(t *testing.T) {
	cases := []struct {
		name  string
		hooks []Hook
		want  []string
	}{
		{"unknown event", []Hook{{Event: "before_tool", Command: "true"}, {Event: "before_everything", Command: "notify-send hi"}},
			[]string{"hooks[1]", `"notify-send hi"`, "before_everything"}},
		{"empty command", []Hook{{Event: "turn_end", Command: "   "}},
			[]string{"hooks[0]", "command is empty"}},
		{"timeout too long", []Hook{{Event: "after_tool", Command: "log.sh", TimeoutSeconds: 601}},
			[]string{"hooks[0]", `"log.sh"`, "601", "out of range"}},
		{"negative timeout", []Hook{{Event: "after_tool", Command: "log.sh", TimeoutSeconds: -1}},
			[]string{"hooks[0]", "out of range"}},
		{"empty tool name", []Hook{{Event: "before_tool", Command: "gate.sh", Tools: []string{"bash", ""}}},
			[]string{"hooks[0]", `"gate.sh"`, "tools is empty"}},
	}
	for _, c := range cases {
		err := validateHooks(c.hooks)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: error %q does not contain %q", c.name, err, w)
			}
		}
	}

	good := []Hook{
		{Event: HookBeforeTool, Tools: []string{"bash"}, Command: "gate.sh"},
		{Event: HookAfterTool, Command: "log.sh", TimeoutSeconds: 600},
		{Event: HookTurnEnd, Command: "notify-send done", TimeoutSeconds: 1},
	}
	if err := validateHooks(good); err != nil {
		t.Errorf("valid hooks rejected: %v", err)
	}
	if err := validateHooks(nil); err != nil {
		t.Errorf("no hooks rejected: %v", err)
	}
}

// Validate() is where the load fails. A bad hook must reach it.
func TestValidateRefusesABadHook(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	prev := cfg.Hooks
	t.Cleanup(func() { cfg.Hooks = prev })
	cfg.Hooks = []Hook{{Event: "on_save", Command: "echo hi"}}
	if err := Validate(); err == nil || !strings.Contains(err.Error(), "hooks[0]") {
		t.Fatalf("Validate() = %v, want the bad hook named", err)
	}
	cfg.Hooks = nil
	// With no provider configured (a clean CI runner) Validate can still fail
	// for that reason; it must not fail because of hooks.
	if err := Validate(); err != nil && strings.Contains(err.Error(), "hooks[") {
		t.Fatalf("Validate() with no hooks blamed a hook: %v", err)
	}
}

// viper decodes with mapstructure tags, not json ones; "timeout_seconds" must
// still land in TimeoutSeconds, and the tools list must survive.
func TestHooksDecodeFromConfigJSON(t *testing.T) {
	raw := `{"hooks":[
		{"event":"before_tool","tools":["bash","edit"],"command":"./gate.sh","timeout_seconds":45},
		{"event":"turn_end","command":"notify-send done"}]}`
	v := viper.New()
	v.SetConfigType("json")
	if err := v.ReadConfig(strings.NewReader(raw)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(c.Hooks) != 2 {
		t.Fatalf("decoded %d hooks, want 2", len(c.Hooks))
	}
	h := c.Hooks[0]
	if h.Event != HookBeforeTool || h.Command != "./gate.sh" || h.TimeoutSeconds != 45 ||
		strings.Join(h.Tools, ",") != "bash,edit" {
		t.Errorf("first hook decoded as %+v", h)
	}
	if c.Hooks[1].Timeout() != HookDefaultTimeoutSeconds {
		t.Errorf("absent timeout_seconds gives %d, want the default %d", c.Hooks[1].Timeout(), HookDefaultTimeoutSeconds)
	}
	if !h.AppliesToTool("edit") || h.AppliesToTool("view") {
		t.Error("the tools filter is not honoured")
	}
	if !c.Hooks[1].AppliesToTool("anything") {
		t.Error("a hook with no tools list must cover every tool")
	}
}

// updateCfgFile round-trips config.json through the Config struct. A key the
// struct does not declare is DELETED by any /connect or /add-dir. The hooks the
// user wrote must survive an unrelated settings write.
func TestAnUnrelatedConfigWriteKeepsTheHooks(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	path := GorillaConfigFile()
	before, readErr := os.ReadFile(path)
	t.Cleanup(func() {
		if readErr == nil {
			_ = os.WriteFile(path, before, 0o600)
		} else {
			_ = os.Remove(path)
		}
	})
	if err := ensureConfigDir(ConfigBase()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"hooks":[{"event":"turn_end","command":"notify-send done","timeout_seconds":5}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateCfgFile(func(c *Config) { c.Debug = false }); err != nil {
		t.Fatalf("updateCfgFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk struct {
		Hooks []Hook `json:"hooks"`
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Hooks) != 1 || onDisk.Hooks[0].Command != "notify-send done" || onDisk.Hooks[0].TimeoutSeconds != 5 {
		t.Fatalf("hooks after an unrelated write: %+v\n%s", onDisk.Hooks, data)
	}
}
