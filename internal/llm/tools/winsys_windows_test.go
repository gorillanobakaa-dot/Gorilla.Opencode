package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// runWindowsTool calls the tool as the agent does, with no session and no
// permission service: it must need neither.
func runWindowsTool(t *testing.T, p WindowsParams) (map[string]any, string) {
	t.Helper()
	in, _ := json.Marshal(p)
	resp, err := NewWindowsTool().Run(context.Background(), ToolCall{Input: string(in)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.IsError {
		return nil, resp.Content
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resp.Content), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, resp.Content)
	}
	return out, ""
}

func items(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, _ := out["items"].([]any)
	var got []map[string]any
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			got = append(got, m)
		}
	}
	return got
}

// TestWindowsToolRegistryReadsOneValue: a known value of a known key, with
// its type, in any spelling of the root.
func TestWindowsToolRegistryReadsOneValue(t *testing.T) {
	for _, path := range []string{`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		`HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, `HKEY_LOCAL_MACHINE/SOFTWARE/Microsoft/Windows NT/CurrentVersion`} {
		out, errText := runWindowsTool(t, WindowsParams{Action: "registry", Path: path, Name: "CurrentBuild"})
		if errText != "" {
			t.Fatalf("%s: %s", path, errText)
		}
		it := items(t, out)
		if len(it) != 1 || it[0]["type"] != "SZ" || it[0]["data"] == "" {
			t.Errorf("%s: %+v", path, it)
		}
	}
}

// TestWindowsToolRegistryListsAndCaps: a big key lists 20 subkeys, says it
// cut, and full=true shows more.
func TestWindowsToolRegistryListsAndCaps(t *testing.T) {
	out, errText := runWindowsTool(t, WindowsParams{Action: "registry", Path: `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion`})
	if errText != "" {
		t.Fatal(errText)
	}
	subs, _ := out["subkeys"].([]any)
	n, _ := out["subkey_count"].(float64)
	if len(subs) != winsysDefaultItems || int(n) <= winsysDefaultItems {
		t.Fatalf("want %d of more than %d subkeys, got %d of %v", winsysDefaultItems, winsysDefaultItems, len(subs), n)
	}
	if c, _ := out["capped"].(string); !strings.Contains(c, "full=true") {
		t.Errorf("the cut is not announced: %q", c)
	}
	full, _ := runWindowsTool(t, WindowsParams{Action: "registry", Path: `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion`, Full: true})
	if fs, _ := full["subkeys"].([]any); len(fs) <= winsysDefaultItems {
		t.Errorf("full=true still shows %d subkeys", len(fs))
	}
}

// TestWindowsToolRegistryMissingKeyIsAnError: a wrong path is said plainly.
func TestWindowsToolRegistryMissingKeyIsAnError(t *testing.T) {
	if _, errText := runWindowsTool(t, WindowsParams{Action: "registry", Path: `HKCU\Software\No-Such-Key-Gorilla-Test`}); !strings.Contains(errText, "no key") {
		t.Errorf("got %q", errText)
	}
	if _, errText := runWindowsTool(t, WindowsParams{Action: "registry", Path: `C:\Windows`}); !strings.Contains(errText, "registry root") {
		t.Errorf("got %q", errText)
	}
}

// TestWindowsToolServicesWithoutAdmin: the event log service is found by a
// part of its name, running, with its start type, as a normal user.
func TestWindowsToolServicesWithoutAdmin(t *testing.T) {
	out, errText := runWindowsTool(t, WindowsParams{Action: "services", Name: "eventlog"})
	if errText != "" {
		t.Fatal(errText)
	}
	var found map[string]any
	for _, it := range items(t, out) {
		if it["name"] == "EventLog" {
			found = it
		}
	}
	if found == nil || found["state"] != "Running" || found["start"] == nil {
		t.Fatalf("EventLog not found running with its start type: %+v", out)
	}
	all, _ := runWindowsTool(t, WindowsParams{Action: "services"})
	if total, _ := all["total"].(float64); total < 50 || len(items(t, all)) != winsysDefaultItems {
		t.Errorf("listing every service: total %v, shown %d", total, len(items(t, all)))
	}
}

// TestWindowsToolEventLogReturnsEntries: the System log of the last week has
// entries, newest first, each with time, id, level and source.
func TestWindowsToolEventLogReturnsEntries(t *testing.T) {
	out, errText := runWindowsTool(t, WindowsParams{Action: "eventlog", Log: "System", Level: "all", Hours: 168})
	if errText != "" {
		t.Fatal(errText)
	}
	it := items(t, out)
	if len(it) == 0 {
		t.Fatal("no System events in a week")
	}
	for _, k := range []string{"time", "id", "level", "source"} {
		if _, ok := it[0][k]; !ok {
			t.Errorf("an event lacks %q: %+v", k, it[0])
		}
	}
	if len(it) == winsysDefaultItems {
		if c, _ := out["capped"].(string); !strings.Contains(c, "newest") {
			t.Errorf("a full page does not say there are more: %q", c)
		}
	}
	if _, errText := runWindowsTool(t, WindowsParams{Action: "eventlog", Log: "No-Such-Log-Gorilla"}); errText == "" {
		t.Error("a log that does not exist gave no error")
	}
}

// TestWindowsToolTasksListsWindowsOwnTasks: Windows' own scheduled tasks
// are listed with state and what they run.
func TestWindowsToolTasksListsWindowsOwnTasks(t *testing.T) {
	out, errText := runWindowsTool(t, WindowsParams{Action: "tasks", Name: `\Microsoft\Windows\`})
	if errText != "" {
		t.Fatal(errText)
	}
	it := items(t, out)
	if len(it) == 0 {
		t.Fatal("no Microsoft tasks")
	}
	if _, ok := it[0]["state"]; !ok {
		t.Errorf("a task has no state: %+v", it[0])
	}
	if total, _ := out["total"].(float64); int(total) < len(it) {
		t.Errorf("total %v below shown %d", total, len(it))
	}
}

// TestWindowsToolCIMReadsTheListOnly: a listed class answers with its fixed
// properties; an unlisted one (Win32_Product repairs installs when queried)
// is refused with the list.
func TestWindowsToolCIMReadsTheListOnly(t *testing.T) {
	out, errText := runWindowsTool(t, WindowsParams{Action: "cim", Class: "Win32_OperatingSystem"})
	if errText != "" {
		t.Fatal(errText)
	}
	it := items(t, out)
	if len(it) != 1 || !strings.Contains(it[0]["Caption"].(string), "Windows") {
		t.Fatalf("Win32_OperatingSystem: %+v", it)
	}
	if s, _ := it[0]["LastBootUpTime"].(string); !strings.Contains(s, "T") {
		t.Errorf("dates are not ISO 8601: %q", s)
	}
	bios, errText := runWindowsTool(t, WindowsParams{Action: "cim", Class: "Win32_BIOS"})
	if errText != "" {
		t.Fatal(errText)
	}
	if b := items(t, bios); len(b) == 0 || b[0]["SerialNumber"] != nil {
		t.Errorf("Win32_BIOS: %+v (the serial number must not be read)", b)
	}
	if _, errText := runWindowsTool(t, WindowsParams{Action: "cim", Class: "Win32_Product"}); !strings.Contains(errText, "Win32_OperatingSystem") {
		t.Errorf("Win32_Product was not refused with the list: %q", errText)
	}
}

// TestWindowsToolBadInputIsSaidPlainly: no action, an unknown action, a bad
// level.
func TestWindowsToolBadInputIsSaidPlainly(t *testing.T) {
	for p, want := range map[WindowsParams]string{
		{}:                                   "action is",
		{Action: "write"}:                    "action is",
		{Action: "registry"}:                 "path is required",
		{Action: "eventlog", Level: "debug"}: "level is",
	} {
		if _, errText := runWindowsTool(t, p); !strings.Contains(errText, want) {
			t.Errorf("%+v: got %q, want %q", p, errText, want)
		}
	}
}
