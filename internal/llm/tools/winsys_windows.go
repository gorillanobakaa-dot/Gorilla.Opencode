package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/mgr"
)

// winsysTimeout bounds one PowerShell query (a full scheduled-task listing
// takes a few seconds on a busy machine).
const winsysTimeout = 60 * time.Second

func windowsQuery(ctx context.Context, p WindowsParams, limit int) (winsysResult, error) {
	switch p.Action {
	case "registry":
		return registryRead(p, limit)
	case "services":
		return servicesRead(p, limit)
	case "eventlog":
		return eventlogRead(ctx, p, limit)
	case "tasks":
		return tasksRead(ctx, p, limit)
	case "cim":
		return cimRead(ctx, p, limit)
	}
	return winsysResult{}, fmt.Errorf("unknown action %q", p.Action)
}

// ---- registry: golang.org/x/sys/windows/registry, read rights only ----

var registryRoots = map[string]registry.Key{
	"HKLM": registry.LOCAL_MACHINE, "HKEY_LOCAL_MACHINE": registry.LOCAL_MACHINE,
	"HKCU": registry.CURRENT_USER, "HKEY_CURRENT_USER": registry.CURRENT_USER,
	"HKCR": registry.CLASSES_ROOT, "HKEY_CLASSES_ROOT": registry.CLASSES_ROOT,
	"HKU": registry.USERS, "HKEY_USERS": registry.USERS,
	"HKCC": registry.CURRENT_CONFIG, "HKEY_CURRENT_CONFIG": registry.CURRENT_CONFIG,
}

// splitRegistryPath accepts HKLM\X, HKLM:\X (PowerShell) and
// HKEY_LOCAL_MACHINE\X, with either slash.
func splitRegistryPath(path string) (registry.Key, string, string, error) {
	p := strings.Trim(strings.ReplaceAll(strings.TrimSpace(path), "/", `\`), `\`)
	p = strings.TrimPrefix(p, `Registry::`)
	root, rest, _ := strings.Cut(p, `\`)
	root = strings.TrimSuffix(strings.ToUpper(root), ":")
	k, ok := registryRoots[root]
	if !ok {
		return 0, "", "", fmt.Errorf("%q does not start with a registry root (HKLM, HKCU, HKCR, HKU, HKCC)", path)
	}
	return k, rest, root, nil
}

var registryTypeNames = map[uint32]string{
	registry.NONE: "NONE", registry.SZ: "SZ", registry.EXPAND_SZ: "EXPAND_SZ", registry.BINARY: "BINARY",
	registry.DWORD: "DWORD", registry.DWORD_BIG_ENDIAN: "DWORD_BIG_ENDIAN", registry.LINK: "LINK",
	registry.MULTI_SZ: "MULTI_SZ", registry.RESOURCE_LIST: "RESOURCE_LIST", registry.QWORD: "QWORD",
}

func registryValue(k registry.Key, name string, full bool) (map[string]any, error) {
	_, typ, err := k.GetValue(name, nil)
	if err != nil {
		return nil, err
	}
	item := map[string]any{"name": name, "type": registryTypeNames[typ]}
	if name == "" {
		item["name"] = "(Default)"
	}
	switch typ {
	case registry.SZ, registry.EXPAND_SZ:
		s, _, err := k.GetStringValue(name)
		if err != nil {
			return nil, err
		}
		item["data"] = s
	case registry.MULTI_SZ:
		s, _, err := k.GetStringsValue(name)
		if err != nil {
			return nil, err
		}
		item["data"] = s
	case registry.DWORD, registry.QWORD:
		n, _, err := k.GetIntegerValue(name)
		if err != nil {
			return nil, err
		}
		item["data"] = n
	default:
		b, _, err := k.GetBinaryValue(name)
		if err != nil {
			return nil, err
		}
		max := 64
		if full {
			max = 2048
		}
		item["bytes"] = len(b)
		if len(b) > max {
			b = b[:max]
			item["cut"] = true
		}
		item["data"] = hex.EncodeToString(b)
	}
	return item, nil
}

func registryRead(p WindowsParams, limit int) (winsysResult, error) {
	root, sub, rootName, err := splitRegistryPath(p.Path)
	if err != nil {
		return winsysResult{}, err
	}
	k, err := registry.OpenKey(root, sub, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return winsysResult{}, fmt.Errorf(`no key %s\%s`, rootName, sub)
		}
		return winsysResult{}, fmt.Errorf(`cannot open %s\%s: %v`, rootName, sub, err)
	}
	defer k.Close()
	key := rootName
	if sub != "" {
		key += `\` + sub
	}
	if p.Name != "" {
		v, err := registryValue(k, p.Name, p.Full)
		if err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				return winsysResult{}, fmt.Errorf("key %s has no value %q", key, p.Name)
			}
			return winsysResult{}, err
		}
		return winsysResult{Total: 1, Items: []map[string]any{v}, Extra: map[string]any{"key": key}}, nil
	}
	names, err := k.ReadValueNames(-1)
	if err != nil {
		return winsysResult{}, err
	}
	sort.Strings(names)
	subkeys, _ := k.ReadSubKeyNames(-1)
	sort.Strings(subkeys)
	var items []map[string]any
	for _, n := range names {
		if len(items) > limit {
			break
		}
		if v, err := registryValue(k, n, p.Full); err == nil {
			items = append(items, v)
		}
	}
	if subkeys == nil {
		subkeys = []string{}
	}
	return winsysResult{Total: len(names), Items: items,
		Extra: map[string]any{"key": key, "subkeys": subkeys, "subkey_count": len(subkeys)}}, nil
}

// ---- services: the service manager with connect/enumerate/query rights ----

var serviceStates = map[uint32]string{
	windows.SERVICE_STOPPED: "Stopped", windows.SERVICE_START_PENDING: "StartPending",
	windows.SERVICE_STOP_PENDING: "StopPending", windows.SERVICE_RUNNING: "Running",
	windows.SERVICE_CONTINUE_PENDING: "ContinuePending", windows.SERVICE_PAUSE_PENDING: "PausePending",
	windows.SERVICE_PAUSED: "Paused",
}

var serviceStarts = map[uint32]string{
	mgr.StartAutomatic: "Automatic", mgr.StartManual: "Manual", mgr.StartDisabled: "Disabled",
	windows.SERVICE_BOOT_START: "Boot", windows.SERVICE_SYSTEM_START: "System",
}

func servicesRead(p WindowsParams, limit int) (winsysResult, error) {
	h, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT|windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return winsysResult{}, fmt.Errorf("cannot open the service manager: %v", err)
	}
	defer windows.CloseServiceHandle(h)

	var needed, count, resume uint32
	var buf []byte
	for {
		var bp *byte
		if len(buf) > 0 {
			bp = &buf[0]
		}
		err = windows.EnumServicesStatusEx(h, windows.SC_ENUM_PROCESS_INFO, windows.SERVICE_WIN32,
			windows.SERVICE_STATE_ALL, bp, uint32(len(buf)), &needed, &count, &resume, nil)
		if err == nil {
			break
		}
		if err != windows.ERROR_MORE_DATA || needed <= uint32(len(buf)) {
			return winsysResult{}, fmt.Errorf("cannot list services: %v", err)
		}
		buf = make([]byte, needed)
	}
	var all []windows.ENUM_SERVICE_STATUS_PROCESS
	if count > 0 {
		all = unsafe.Slice((*windows.ENUM_SERVICE_STATUS_PROCESS)(unsafe.Pointer(&buf[0])), count)
	}
	type svcRow struct {
		name, display string
		st            windows.SERVICE_STATUS_PROCESS
	}
	var rows []svcRow
	for _, s := range all {
		name, display := windows.UTF16PtrToString(s.ServiceName), windows.UTF16PtrToString(s.DisplayName)
		if winsysNameMatches(p.Name, name, display) {
			rows = append(rows, svcRow{name, display, s.ServiceStatusProcess})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name) })

	var items []map[string]any
	for i, r := range rows {
		if i > limit {
			break
		}
		item := map[string]any{"name": r.name, "display": r.display, "state": serviceStates[r.st.CurrentState]}
		if r.st.ProcessId != 0 {
			item["pid"] = r.st.ProcessId
		}
		if cfg, ok := serviceConfig(h, r.name); ok {
			start := serviceStarts[cfg.StartType]
			if cfg.DelayedAutoStart {
				start += " (delayed)"
			}
			item["start"] = start
			item["binary"] = cfg.BinaryPathName
			item["account"] = cfg.ServiceStartName
		}
		items = append(items, item)
	}
	return winsysResult{Total: len(rows), Items: items}, nil
}

// serviceConfig reads one service's configuration with query rights only
// (mgr.Mgr.OpenService asks for every right, which a normal user lacks).
func serviceConfig(h windows.Handle, name string) (mgr.Config, bool) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return mgr.Config{}, false
	}
	sh, err := windows.OpenService(h, n, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return mgr.Config{}, false
	}
	s := &mgr.Service{Name: name, Handle: sh}
	defer s.Close()
	cfg, err := s.Config()
	return cfg, err == nil
}


// ---- event log, scheduled tasks, CIM: fixed PowerShell scripts ----

// winsysPSPrelude is shared: UTF-8 out, stop on errors, ISO dates.
const winsysPSPrelude = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false
function Iso($v) { if ($v -is [datetime]) { $v.ToString('o') } else { $v } }
`

const eventlogScript = winsysPSPrelude + `
$f = @{ LogName = $env:GORILLA_WIN_LOG; StartTime = (Get-Date).AddHours(-[double]$env:GORILLA_WIN_HOURS) }
$levels = @($env:GORILLA_WIN_LEVELS -split ',' | Where-Object { $_ } | ForEach-Object { [int]$_ })
if ($levels.Count -gt 0) { $f.Level = $levels }
$ev = @()
try { $ev = @(Get-WinEvent -FilterHashtable $f -MaxEvents ([int]$env:GORILLA_WIN_MAX) -ErrorAction Stop) }
catch { if ($_.FullyQualifiedErrorId -notlike 'NoMatchingEventsFound*') { throw } }
$items = @(foreach ($e in $ev) {
  [ordered]@{ time = (Iso $e.TimeCreated); id = $e.Id; level = $e.LevelDisplayName; source = $e.ProviderName; message = [string]$e.Message }
})
ConvertTo-Json -InputObject $items -Depth 3 -Compress
`

const tasksScript = winsysPSPrelude + `
$n = $env:GORILLA_WIN_NAME
$all = @(Get-ScheduledTask)
$t = @($all | Where-Object { -not $n -or ($_.TaskPath + $_.TaskName).IndexOf($n, [StringComparison]::OrdinalIgnoreCase) -ge 0 } | Sort-Object TaskPath, TaskName)
$items = @(foreach ($x in ($t | Select-Object -First ([int]$env:GORILLA_WIN_MAX))) {
  $i = $x | Get-ScheduledTaskInfo -ErrorAction SilentlyContinue
  [ordered]@{ path = $x.TaskPath; name = $x.TaskName; state = [string]$x.State
    last_run = (Iso $i.LastRunTime); last_result = $i.LastTaskResult; next_run = (Iso $i.NextRunTime)
    runs = ((@($x.Actions) | ForEach-Object { ([string]$_.Execute + ' ' + [string]$_.Arguments).Trim() }) -join '; ') }
})
ConvertTo-Json -InputObject ([ordered]@{ total = $t.Count; items = $items }) -Depth 4 -Compress
`

const cimScript = winsysPSPrelude + `
$props = $env:GORILLA_WIN_PROPS -split ','
$q = @{ ClassName = $env:GORILLA_WIN_CLASS }
if ($env:GORILLA_WIN_FILTER) { $q.Filter = $env:GORILLA_WIN_FILTER }
$items = @(foreach ($o in @(Get-CimInstance @q -Property $props)) {
  $h = [ordered]@{}
  foreach ($p in $props) { $h[$p] = Iso $o.$p }
  $h
})
ConvertTo-Json -InputObject $items -Depth 3 -Compress
`

// runWinsysPS runs one fixed script with its inputs in the environment.
func runWinsysPS(ctx context.Context, script string, env ...string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, winsysTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, windowsPowerShell51(), "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePS(script))
	cmd.Env = append(os.Environ(), env...)
	var out, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out after %s", winsysTimeout)
		}
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(firstLine(msg))
	}
	return []byte(strings.TrimSpace(out.String())), nil
}

func decodeItems(b []byte) ([]map[string]any, error) {
	var items []map[string]any
	if len(b) == 0 {
		return items, nil
	}
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("unreadable PowerShell output: %v", err)
	}
	return items, nil
}

func eventlogRead(ctx context.Context, p WindowsParams, limit int) (winsysResult, error) {
	levels := map[string]string{"": "1,2", "error": "1,2", "warning": "1,2,3", "all": ""}[p.Level]
	out, err := runWinsysPS(ctx, eventlogScript, "GORILLA_WIN_LOG="+p.Log, "GORILLA_WIN_LEVELS="+levels,
		"GORILLA_WIN_HOURS="+strconv.Itoa(p.Hours), "GORILLA_WIN_MAX="+strconv.Itoa(limit+1))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") || strings.Contains(err.Error(), "denied") {
			return winsysResult{}, fmt.Errorf("the %s log needs administrator rights to read: %v", p.Log, err)
		}
		return winsysResult{}, err
	}
	items, err := decodeItems(out)
	if err != nil {
		return winsysResult{}, err
	}
	r := winsysResult{Total: -1, Items: items, Extra: map[string]any{"log": p.Log, "hours": p.Hours}}
	if len(items) <= limit {
		r.Total = len(items)
	}
	return r, nil
}

func tasksRead(ctx context.Context, p WindowsParams, limit int) (winsysResult, error) {
	out, err := runWinsysPS(ctx, tasksScript, "GORILLA_WIN_NAME="+p.Name, "GORILLA_WIN_MAX="+strconv.Itoa(limit+1))
	if err != nil {
		return winsysResult{}, err
	}
	var got struct {
		Total int              `json:"total"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		return winsysResult{}, fmt.Errorf("unreadable PowerShell output: %v", err)
	}
	return winsysResult{Total: got.Total, Items: got.Items, More: got.Total > len(got.Items)}, nil
}

func cimRead(ctx context.Context, p WindowsParams, limit int) (winsysResult, error) {
	c := cimClasses[p.Class]
	out, err := runWinsysPS(ctx, cimScript, "GORILLA_WIN_CLASS="+p.Class,
		"GORILLA_WIN_PROPS="+strings.Join(c.props, ","), "GORILLA_WIN_FILTER="+c.filter)
	if err != nil {
		return winsysResult{}, err
	}
	all, err := decodeItems(out)
	if err != nil {
		return winsysResult{}, err
	}
	var items []map[string]any
	for _, it := range all {
		var fields []string
		for _, k := range []string{"Name", "Caption", "Description", "DeviceID", "HotFixID", "DeviceName", "Model", "DriveLetter"} {
			if s, ok := it[k].(string); ok {
				fields = append(fields, s)
			}
		}
		if winsysNameMatches(p.Name, fields...) {
			items = append(items, it)
		}
	}
	return winsysResult{Total: len(items), Items: items, Extra: map[string]any{"class": p.Class}}, nil
}
