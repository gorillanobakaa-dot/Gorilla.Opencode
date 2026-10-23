package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The macOS launcher (packaging/macos/Gorilla OpenCode.command) sizes the Terminal
// window and centers it in the screen's visible frame, the area without the menu
// bar and the Dock. On the 11-inch MacBook Air (1366x768) it once opened at 169x33
// with its right part past the screen edge, because it only grew the window and
// never placed it. Its arithmetic is plain bash, sourced here with the launch
// itself skipped, so it is checked on every system CI runs, not only on a Mac.

func launcherBash(t *testing.T, fn string, args ...int) []int {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on PATH")
	}
	if runtime.GOOS == "windows" && strings.Contains(strings.ToLower(bash), `\windows\system32\`) {
		t.Skip("bash here is WSL's, not Git Bash")
	}
	launcher, err := filepath.Abs("../packaging/macos/Gorilla OpenCode.command")
	if err != nil {
		t.Fatal(err)
	}
	// The path goes in the environment: as $0 it would make the launcher think it
	// was run, not sourced, and start the program.
	argv := []string{"-c", `source "$LAUNCHER" || exit 9; ` + fn + ` "$@"`, "bash"}
	for _, a := range args {
		argv = append(argv, strconv.Itoa(a))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, argv...)
	cmd.Env = append(os.Environ(), "LAUNCHER="+filepath.ToSlash(launcher))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", fn, args, err, out)
	}
	var got []int
	for _, f := range strings.Fields(string(out)) {
		n, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("%s %v printed %q", fn, args, out)
		}
		got = append(got, n)
	}
	return got
}

func TestMacLauncherWindowPlan(t *testing.T) {
	type frame struct{ x, y, w, h int }
	cases := []struct {
		name         string
		vf           frame
		cols, rows   int // the window now: cells and points
		winW, winH   int
		cellW, cellH int // hundredths of a point
		wantC, wantR int
		out          []int // cols rows left top right bottom
	}{
		// Menu bar 22, Dock at the bottom, Menlo 11 (6.62x13): 167x39 fits, centered.
		{"Air, Dock at the bottom", frame{0, 22, 1366, 694}, 80, 24, 540, 338, 662, 1300, 167, 39,
			[]int{167, 39, 125, 102, 1241, 635}},
		// Dock on the left, a bigger font: 167 columns do not fit, 162 do, flush right.
		{"Air, Dock on the left, 8x17 cells", frame{60, 22, 1306, 746}, 80, 24, 650, 434, 800, 1700, 167, 39,
			[]int{162, 39, 60, 50, 1366, 739}},
		// A hidden Dock leaves a 4-point strip at the bottom: more height, still centered.
		{"Air, Dock hidden", frame{0, 22, 1366, 742}, 80, 24, 540, 338, 662, 1300, 167, 39,
			[]int{167, 39, 125, 126, 1241, 659}},
		// GORILLA_WINDOW=300x80 is the person's choice, still capped to the screen.
		{"override bigger than the screen", frame{0, 22, 1366, 694}, 80, 24, 540, 338, 662, 1300, 300, 80,
			[]int{204, 51, 2, 24, 1363, 713}},
		// The window is already past the screen: the cap goes below its current size.
		{"window wider than the screen", frame{0, 22, 1000, 694}, 200, 39, 1340, 533, 662, 1300, 167, 39,
			[]int{148, 39, 2, 102, 998, 635}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := launcherBash(t, "gorilla_window_plan", c.vf.x, c.vf.y, c.vf.w, c.vf.h,
				c.cols, c.rows, c.winW, c.winH, c.cellW, c.cellH, c.wantC, c.wantR)
			if len(got) != 6 {
				t.Fatalf("got %v", got)
			}
			l, top, r, b := got[2], got[3], got[4], got[5]
			if l < c.vf.x || top < c.vf.y || r > c.vf.x+c.vf.w || b > c.vf.y+c.vf.h {
				t.Errorf("bounds %v leave the visible frame %+v", got[2:], c.vf)
			}
			if d := (l - c.vf.x) - (c.vf.x + c.vf.w - r); d < -1 || d > 1 {
				t.Errorf("bounds %v are not centered across %+v", got[2:], c.vf)
			}
			if d := (top - c.vf.y) - (c.vf.y + c.vf.h - b); d < -1 || d > 1 {
				t.Errorf("bounds %v are not centered down %+v", got[2:], c.vf)
			}
			for i := range c.out {
				if got[i] != c.out[i] {
					t.Fatalf("got %v, want %v", got, c.out)
				}
			}
		})
	}
}

// The cell size comes from two measurements, which cancels the title bar and the
// margins; with nothing to compare it overestimates, so the window errs small.
func TestMacLauncherCellSize(t *testing.T) {
	if got := launcherBash(t, "gorilla_cell", 80, 24, 540, 338, 167, 39, 1116, 533); got[0] != 663 || got[1] != 1300 {
		t.Errorf("two sizes: got %v, want [663 1300]", got)
	}
	if got := launcherBash(t, "gorilla_cell", 167, 39, 1116, 533, 167, 39, 1116, 533); got[0] != 669 || got[1] != 1367 {
		t.Errorf("one size: got %v, want [669 1367]", got)
	}
}

func TestMacLauncherCenterClampsToTheFrame(t *testing.T) {
	// Too big to fit: pinned to the top-left corner of the frame, not centered off it.
	got := launcherBash(t, "gorilla_center", 0, 22, 800, 500, 1000, 600)
	if len(got) != 4 || got[0] != 0 || got[1] != 22 || got[2] != 1000 || got[3] != 622 {
		t.Errorf("got %v, want [0 22 1000 622]", got)
	}
}
