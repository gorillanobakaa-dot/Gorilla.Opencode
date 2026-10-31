package basic

import (
	"archive/zip"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// wwwPrefix marks a Files entry that goes to the folder the http service
// serves, which is outside the job's folder so the model cannot cheat by
// copying it locally.
const wwwPrefix = "@www/"

// Prepared is a job's world: its folder, its placeholders, and a Stop that
// ends every helper the setup started.
type Prepared struct {
	Dir  string // the job's working folder, what -c gets
	Www  string // the folder served by the http service ("" if none)
	Vars Vars
	stop func()
}

// Stop ends the job's helpers (the http server, the port listener, the log
// appender). Safe to call twice.
func (p *Prepared) Stop() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

// Prepare makes the job's fresh folder under root and starts its service.
// fixtures is the folder of committed binary fixtures.
func Prepare(j Job, root, fixtures string) (*Prepared, error) {
	dir := filepath.Join(root, j.ID)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := &Prepared{Dir: dir, Vars: Vars{"SCRATCH": dir}}
	var stops []func()
	p.stop = func() {
		for _, s := range stops {
			s()
		}
	}
	fail := func(err error) (*Prepared, error) {
		p.Stop()
		return nil, err
	}

	// Services first: their placeholders feed the prompt and the files.
	switch j.Service {
	case "http":
		www := filepath.Join(root, j.ID+"-www")
		if err := os.RemoveAll(www); err != nil {
			return fail(err)
		}
		if err := os.MkdirAll(www, 0o755); err != nil {
			return fail(err)
		}
		p.Www = www
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fail(err)
		}
		srv := &http.Server{Handler: http.FileServer(http.Dir(www))}
		go srv.Serve(ln)
		stops = append(stops, func() { srv.Close() })
		p.Vars["URL"] = "http://" + ln.Addr().String()
		p.Vars["PORT"] = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	case "port":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fail(err)
		}
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				c.Close()
			}
		}()
		stops = append(stops, func() { ln.Close() })
		p.Vars["PORT"] = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		p.Vars["PID"] = strconv.Itoa(os.Getpid())
	}

	write := func(rel, content string) error {
		base := dir
		if strings.HasPrefix(rel, wwwPrefix) {
			base, rel = p.Www, strings.TrimPrefix(rel, wwwPrefix)
			if base == "" {
				return fmt.Errorf("%s needs service: http", wwwPrefix)
			}
		}
		full := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(p.Vars.Expand(content)), 0o644)
	}
	for rel, content := range j.Files {
		if err := write(rel, content); err != nil {
			return fail(err)
		}
	}
	for _, d := range j.Dirs {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755); err != nil {
			return fail(err)
		}
	}
	for rel, spec := range j.Lines {
		n, format, ok := strings.Cut(spec, ":")
		count, err := strconv.Atoi(n)
		if !ok || err != nil {
			return fail(fmt.Errorf("lines %s: want N:format, got %q", rel, spec))
		}
		var b strings.Builder
		for i := 1; i <= count; i++ {
			fmt.Fprintf(&b, format+"\n", i)
		}
		if err := write(rel, b.String()); err != nil {
			return fail(err)
		}
	}
	for rel, members := range j.Zips {
		if err := writeZip(filepath.Join(dir, filepath.FromSlash(rel)), members, p.Vars); err != nil {
			return fail(err)
		}
	}
	for dest, name := range j.Fixtures {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			return fail(fmt.Errorf("fixture %s: %w", name, err))
		}
		full := filepath.Join(dir, filepath.FromSlash(dest))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return fail(err)
		}
	}
	for _, rel := range j.Hidden {
		if err := hide(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			return fail(err)
		}
	}

	if j.Service == "growing_log" {
		log := filepath.Join(dir, "app.log")
		var b strings.Builder
		for i := 0; i < 5; i++ {
			fmt.Fprintf(&b, "tick-%03d service started\n", i)
		}
		if err := os.WriteFile(log, []byte(b.String()), 0o644); err != nil {
			return fail(err)
		}
		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 5; i < 60; i++ {
				select {
				case <-done:
					return
				case <-time.After(700 * time.Millisecond):
				}
				if f, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
					fmt.Fprintf(f, "tick-%03d heartbeat\n", i)
					f.Close()
				}
			}
		}()
		stops = append(stops, func() { close(done); wg.Wait() })
	}
	return p, nil
}

// hide marks a file hidden: the attribute on Windows, a leading dot is already
// in the name on Linux (the job files that use hidden: are named that way).
func hide(path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if out, err := exec.Command("attrib", "+h", path).CombinedOutput(); err != nil {
		return fmt.Errorf("attrib +h %s: %v: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeZip(path string, members map[string]string, vars Vars) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for name, content := range members {
		w, err := zw.Create(name)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write([]byte(vars.Expand(content))); err != nil {
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
