package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// GORILLA (2026-10-27, windows-capabilities job 09, REPORT.md section 3 step
// 4): the read-only `windows` tool. Registry, services, event log, scheduled
// tasks and a short list of CIM classes, as JSON.
//
// It only reads, so it asks no permission (the owner's rule for this tool).
// What keeps it read-only is how it is built, not a filter on what the model
// types: the registry is opened with query and enumerate rights only, the
// service manager with connect, enumerate and query rights only, and the
// PowerShell parts are fixed scripts that take their inputs from environment
// variables, never from text spliced into code. The CIM classes are a fixed
// list with fixed properties: Win32_Product is not on it (querying it makes
// Windows Installer check, and sometimes repair, every installed product), and
// serial numbers, MAC addresses and process command lines are not read,
// because whatever the tool returns goes to the model's provider.
//
// Registered only on Windows, so no other system pays its schema.

// WindowsToolName is the windows tool's wire name.
const WindowsToolName = "windows"

const (
	winsysDefaultItems = 20
	winsysFullItems    = 500
	winsysDefaultText  = 300
	winsysFullText     = 4000
)

// WindowsParams is the tool's input.
type WindowsParams struct {
	Action string `json:"action"`
	Path   string `json:"path,omitempty"`
	Name   string `json:"name,omitempty"`
	Log    string `json:"log,omitempty"`
	Level  string `json:"level,omitempty"`
	Hours  int    `json:"hours,omitempty"`
	Class  string `json:"class,omitempty"`
	Full   bool   `json:"full,omitempty"`
}

// cimClass is one allowed CIM class, the properties read from it, and an
// optional fixed WQL filter.
type cimClass struct {
	props  []string
	filter string
}

// cimClasses is the whole list the cim action may read.
var cimClasses = map[string]cimClass{
	"Win32_OperatingSystem": {props: []string{"Caption", "Version", "BuildNumber", "OSArchitecture", "InstallDate", "LastBootUpTime", "TotalVisibleMemorySize", "FreePhysicalMemory", "SystemDrive", "Locale"}},
	"Win32_ComputerSystem":  {props: []string{"Manufacturer", "Model", "SystemType", "NumberOfProcessors", "NumberOfLogicalProcessors", "TotalPhysicalMemory", "Domain", "PartOfDomain"}},
	"Win32_Processor":       {props: []string{"Name", "NumberOfCores", "NumberOfLogicalProcessors", "MaxClockSpeed", "CurrentClockSpeed", "LoadPercentage"}},
	"Win32_BIOS":            {props: []string{"Manufacturer", "SMBIOSBIOSVersion", "ReleaseDate"}},
	"Win32_LogicalDisk":     {props: []string{"DeviceID", "DriveType", "FileSystem", "Size", "FreeSpace", "VolumeName"}},
	"Win32_DiskDrive":       {props: []string{"Model", "InterfaceType", "MediaType", "Size", "Partitions", "Status"}},
	"Win32_PhysicalMemory":  {props: []string{"Manufacturer", "PartNumber", "Capacity", "Speed", "ConfiguredClockSpeed", "DeviceLocator"}},
	"Win32_VideoController": {props: []string{"Name", "DriverVersion", "DriverDate", "VideoProcessor", "AdapterRAM", "CurrentHorizontalResolution", "CurrentVerticalResolution"}},
	"Win32_NetworkAdapterConfiguration": {props: []string{"Description", "IPAddress", "IPSubnet", "DefaultIPGateway", "DNSServerSearchOrder", "DHCPEnabled"},
		filter: "IPEnabled = TRUE"},
	"Win32_Process":                         {props: []string{"ProcessId", "ParentProcessId", "Name", "ExecutablePath", "WorkingSetSize", "CreationDate"}},
	"Win32_QuickFixEngineering":             {props: []string{"HotFixID", "Description", "InstalledOn"}},
	"Win32_StartupCommand":                  {props: []string{"Name", "Command", "Location", "User"}},
	"Win32_PnPSignedDriver":                 {props: []string{"DeviceName", "DriverVersion", "DriverDate", "Manufacturer", "DriverProviderName"}},
	"Win32_Battery":                         {props: []string{"Name", "EstimatedChargeRemaining", "BatteryStatus", "EstimatedRunTime"}},
	"Win32_TimeZone":                        {props: []string{"Caption", "Bias", "StandardName"}},
	"Win32_NetworkAdapter":                  {props: []string{"Name", "NetConnectionID", "NetEnabled", "Speed", "AdapterType"}, filter: "PhysicalAdapter = TRUE"},
	"Win32_Volume":                          {props: []string{"DriveLetter", "Label", "FileSystem", "Capacity", "FreeSpace", "BootVolume", "SystemVolume"}},
	"Win32_ShadowCopy":                      {props: []string{"ID", "InstallDate", "VolumeName"}},
	"Win32_Share":                           {props: []string{"Name", "Path", "Description", "Type"}},
	"Win32_UserAccount":                     {props: []string{"Name", "LocalAccount", "Disabled", "Lockout", "PasswordRequired"}, filter: "LocalAccount = TRUE"},
	"Win32_Environment":                     {props: []string{"Name", "UserName", "SystemVariable"}},
	"Win32_SystemDriver":                    {props: []string{"Name", "State", "StartMode", "PathName"}},
	"Win32_PageFileUsage":                   {props: []string{"Name", "AllocatedBaseSize", "CurrentUsage", "PeakUsage"}},
	"Win32_PerfFormattedData_PerfOS_Memory": {props: []string{"AvailableMBytes", "CommittedBytes", "CommitLimit", "PercentCommittedBytesInUse"}},
}

// cimClassNames is the list, sorted, for the schema and error messages.
func cimClassNames() []string {
	names := make([]string, 0, len(cimClasses))
	for n := range cimClasses {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

type windowsTool struct{}

// NewWindowsTool builds the read-only windows tool.
func NewWindowsTool() BaseTool { return windowsTool{} }

// WindowsToolAvailable reports whether this system has the tool at all.
func WindowsToolAvailable() bool { return runtime.GOOS == "windows" }

func (windowsTool) Info() ToolInfo {
	return ToolInfo{
		Name: WindowsToolName,
		Description: "Read Windows system facts as JSON: registry keys and values, services, event log, scheduled tasks, " +
			"CIM/WMI classes (OS, CPU, disks, memory, network, processes, hotfixes, drivers). Read-only, so no permission " +
			"is asked. Results cap at 20 items and long text is cut, and the result says so; full=true lifts the caps.",
		Parameters: map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"registry", "services", "eventlog", "tasks", "cim"}},
			"path":   map[string]any{"type": "string", "description": `registry: key, e.g. HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`},
			"name":   map[string]any{"type": "string", "description": "registry: one value; services, tasks, cim: name contains this"},
			"log":    map[string]any{"type": "string", "description": "eventlog: System (default), Application, Setup, or any log name"},
			"level":  map[string]any{"type": "string", "enum": []string{"error", "warning", "all"}, "description": "eventlog: error (default) = critical+error"},
			"hours":  map[string]any{"type": "integer", "description": "eventlog: how far back, default 24"},
			"class":  map[string]any{"type": "string", "description": "cim: a Win32_ class, e.g. Win32_OperatingSystem, Win32_LogicalDisk, Win32_Process"},
			"full":   map[string]any{"type": "boolean"},
		},
		Required: []string{"action"},
	}
}

func (t windowsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var p WindowsParams
	if err := UnmarshalToolInput(call.Input, &p); err != nil {
		return NewTextErrorResponse("windows: could not read the parameters: " + err.Error()), nil
	}
	if !WindowsToolAvailable() {
		return NewTextErrorResponse("the windows tool reads Windows system facts and this is " + runtime.GOOS + "; it has nothing to read here"), nil
	}
	res, err := windowsRun(ctx, p)
	if err != nil {
		return NewTextErrorResponse("windows " + p.Action + ": " + err.Error()), nil
	}
	return NewTextResponse(res), nil
}

// windowsRun checks the input, runs the action and renders the capped JSON.
func windowsRun(ctx context.Context, p WindowsParams) (string, error) {
	switch p.Action {
	case "registry":
		if strings.TrimSpace(p.Path) == "" {
			return "", fmt.Errorf(`path is required, e.g. HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`)
		}
	case "services", "tasks":
	case "eventlog":
		if p.Log == "" {
			p.Log = "System"
		}
		if p.Hours <= 0 {
			p.Hours = 24
		}
		switch p.Level {
		case "", "error", "warning", "all":
		default:
			return "", fmt.Errorf("level is error, warning or all, not %q", p.Level)
		}
	case "cim":
		if _, ok := cimClasses[p.Class]; !ok {
			return "", fmt.Errorf("class %q is not on the read-only list: %s", p.Class, strings.Join(cimClassNames(), ", "))
		}
	default:
		return "", fmt.Errorf("action is registry, services, eventlog, tasks or cim, not %q", p.Action)
	}
	r, err := windowsQuery(ctx, p, winsysItemCap(p.Full))
	if err != nil {
		return "", err
	}
	return renderWinsys(p, r), nil
}

// winsysResult is what one action found, before the caps.
type winsysResult struct {
	// Total is how many matched, when the action can count them; -1 when it
	// can only say "at least this many" (the event log).
	Total int
	Items []map[string]any
	// Extra is shown above the items (the registry key's subkeys).
	Extra map[string]any
	// More says the source had more items than were fetched.
	More bool
}

func winsysItemCap(full bool) int {
	if full {
		return winsysFullItems
	}
	return winsysDefaultItems
}

// renderWinsys caps the items and their text and says what was cut.
func renderWinsys(p WindowsParams, r winsysResult) string {
	maxItems, maxText := winsysItemCap(p.Full), winsysDefaultText
	if p.Full {
		maxText = winsysFullText
	}
	var notes []string
	items := r.Items
	if len(items) > maxItems {
		items = items[:maxItems]
		r.More = true
	}
	cutText := false
	for _, it := range items {
		for k, v := range it {
			if s, ok := v.(string); ok && len(s) > maxText {
				it[k] = cutAtRune(s, maxText) + "...[cut]"
				cutText = true
			}
		}
	}
	for k, v := range r.Extra {
		if list, ok := v.([]string); ok && len(list) > maxItems {
			r.Extra[k] = list[:maxItems]
			notes = append(notes, fmt.Sprintf("%s: %d of %d shown", k, maxItems, len(list)))
		}
	}
	if r.More {
		if r.Total > 0 {
			notes = append(notes, fmt.Sprintf("items: %d of %d shown", len(items), r.Total))
		} else {
			notes = append(notes, fmt.Sprintf("items: the %d newest shown; there are more", len(items)))
		}
	}
	if cutText {
		notes = append(notes, fmt.Sprintf("text longer than %d characters was cut", maxText))
	}
	if len(notes) > 0 && !p.Full {
		notes = append(notes, "full=true shows more")
	}

	out := map[string]any{"action": p.Action, "shown": len(items), "items": items}
	if r.Total >= 0 {
		out["total"] = r.Total
	}
	for k, v := range r.Extra {
		out[k] = v
	}
	if len(notes) > 0 {
		out["capped"] = strings.Join(notes, "; ")
	}
	b, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(b)
}

// winsysNameMatches is the case-insensitive "contains" every name filter uses.
func winsysNameMatches(filter string, fields ...string) bool {
	if filter == "" {
		return true
	}
	f := strings.ToLower(filter)
	for _, s := range fields {
		if strings.Contains(strings.ToLower(s), f) {
			return true
		}
	}
	return false
}
