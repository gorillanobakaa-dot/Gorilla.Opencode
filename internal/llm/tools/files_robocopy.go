package tools

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// robocopyArgs builds the command line for one tree action. preview adds /L
// (list only: robocopy's own dry run) and leaves out /MT, whose output order
// is not needed for a list. The log goes to logPath as UTF-16 (/UNILOG), so
// non-ASCII names survive whatever the console code page is.
//
// purge alone is robocopy /PURGE with every copy class excluded (/XC /XN /XO
// /XL): measured on Windows 11 26200, it deletes the extras and copies
// nothing, which is what "delete what is no longer in the source" means.
func robocopyArgs(op syncOp, preview bool, logPath string) []string {
	args := []string{op.Src, op.Dst}
	if op.Pattern != "" {
		args = append(args, op.Pattern)
	}
	switch op.Action {
	case "copy":
		args = append(args, "/E")
	case "mirror":
		args = append(args, "/MIR")
	case "purge":
		args = append(args, "/E", "/PURGE", "/XC", "/XN", "/XO", "/XL")
	case "move":
		args = append(args, "/E", "/MOVE")
	case "move_files":
		args = append(args, "/E", "/MOV")
	}
	// /XJ: junctions and symbolic links are never followed (a junction loop
	// would otherwise copy forever). /R:2 /W:2: the defaults are a million
	// retries 30 s apart. /FP /BYTES /TS: full paths and exact sizes for the
	// parser; /NP /NJH /NJS: no progress or banners in the log.
	args = append(args, "/XJ", "/R:2", "/W:2", "/FP", "/BYTES", "/NP", "/NJH", "/NJS")
	if preview {
		args = append(args, "/L")
	} else {
		args = append(args, fmt.Sprintf("/MT:%d", op.Threads))
		if op.Restartable {
			args = append(args, "/Z")
		}
	}
	if logPath != "" {
		args = append(args, "/UNILOG:"+logPath)
	}
	return args
}

// decodeUnilog turns robocopy's /UNILOG bytes (UTF-16 LE, with a BOM) into a
// string. Bytes without a BOM are taken as they are.
func decodeUnilog(b []byte) string {
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		return strings.TrimPrefix(string(b), "\uFEFF")
	}
	b = b[2:]
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

// parseRobocopyLog reads a robocopy log written with robocopyArgs' switches.
// Each item line is tab separated: class (with a size for folders), then for
// files the size and time, and the full path last. Measured format:
//
//	\t*EXTRA Dir        -1\tC:\d\old\
//	\t  *EXTRA File \t\t       2 2026/10/27 12:01:42\tC:\d\old\y.txt
//	\t    New File  \t\t       2 2026/10/27 12:01:42\tC:\s\a.txt
//	\t  New Dir          1\tC:\s\sub\
//	\t                   1\tC:\s\          (a folder that already exists)
//
// For move and move_files every file copied is deleted from the source
// afterwards (measured: /MOV leaves "same" files where they are).
func parseRobocopyLog(log, action string) syncPlan {
	plan := syncPlan{Engine: "robocopy"}
	lines := strings.Split(strings.ReplaceAll(log, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r ")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, " ERROR ") || strings.HasPrefix(strings.TrimSpace(line), "ERROR ") {
			msg := strings.TrimSpace(line)
			// The reason is on the next line ("Access is denied.").
			if i+1 < len(lines) && !strings.Contains(lines[i+1], "\t") && strings.TrimSpace(lines[i+1]) != "" {
				msg += ": " + strings.TrimSpace(lines[i+1])
				i++
			}
			plan.Errors = append(plan.Errors, msg)
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		path := strings.TrimSpace(fields[len(fields)-1])
		class := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(fields[1]), "-0123456789 "))
		size := int64(0)
		if len(fields) >= 4 {
			fmt.Sscan(strings.TrimSpace(fields[len(fields)-2]), &size)
		}
		isDir := strings.HasSuffix(path, `\`)
		switch {
		case class == "":
			// an existing folder: a heading, not an action
		case strings.EqualFold(class, "*EXTRA Dir"):
			plan.DeleteDirs = append(plan.DeleteDirs, path)
		case strings.EqualFold(class, "*EXTRA File"):
			plan.Deletes = append(plan.Deletes, path)
		case strings.EqualFold(class, "New Dir"):
			plan.NewDirs = append(plan.NewDirs, path)
		case isDir, strings.EqualFold(class, "same"), strings.EqualFold(class, "skipped"):
		default:
			plan.Copies = append(plan.Copies, path)
			plan.Bytes += size
			if !strings.EqualFold(class, "New File") {
				plan.Overwrites++
			}
		}
	}
	if action == "move" || action == "move_files" {
		plan.Deletes = append(plan.Deletes, plan.Copies...)
	}
	return plan
}
