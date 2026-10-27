// SPDX-License-Identifier: AGPL-3.0-or-later
//
// pfind — Unified hybrid local search, content extraction, and filesystem exploration.
// Copyright (C) 2026  gorillanobakaa
//
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option) any
// later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License along
// with this program. If not, see <https://www.gnu.org/licenses/>.
//
// The deal (copyleft, in one line): use it, fork it, change it, sell it — but any
// version you SHIP or RUN AS A SERVICE must stay open under this same license, so
// every improvement can flow back and the tool keeps growing. Improvements welcome
// upstream: https://github.com/gorillanobakaa-dot/pfind
//
// VERSION: 4.0.0 | UPDATED: 2026-10-27 | STATUS: live
// CHANGELOG:
//   4.0.0 (2026-10-27) — pfind is a native program. pfind.py 3.3.0 ported to Rust, built on
//       ripgrep's OWN crates (ignore, globset, grep-regex, grep-searcher, grep-matcher) instead of
//       running rg.exe, so neither Python nor ripgrep has to be installed: one pfind.exe holds the
//       walker, the matcher and the searcher, and searches on every core. Same flags, same output,
//       same --json schema (engine "ripgrep": it is ripgrep's engine), same exit codes
//       (0 match, 1 none, 2 error); tools/pfind-rs/tests/run_parity.py runs pfind.py and pfind.exe
//       side by side and fails on any difference. What changed, on purpose:
//       (1) -z/--search-zip decompresses in-process (.gz .bz2 .xz .lzma .zst, .zip and .tar
//           members) with pfind.py's shared decompression budget; ripgrep's -z needed gzip/xz
//           programs that Windows does not have, and skipped .zip.
//       (2) --doctor offers nothing to install: the exe has everything it needs. The Chroma
//           "Second Brain" seam (--brain semantic recall) is a Python library: pfind.exe keeps the
//           flag, searches the brain folder lexically, and says so (stderr and --doctor).
//       (3) --pcre2 / --engine pcre2 are not built in (PCRE2 is C code); the default engine is
//           used and pfind says so, as pfind.py did when ripgrep was missing.
//       (4) Listings come back in a stable order (root, then path components) rather than the
//           order rg's threads finished in; a content search that hits --timeout keeps what it
//           found and says the result is PARTIAL (pfind.py discarded everything).
//       (5) Output lines end in LF on every platform, as ripgrep's do.
//   3.3.0 (2026-10-20) — One pfind again (see internal/llm/tools/pfind.py for 3.3.0 and older).
//
// pfind — find stuff, analyze code, and explore filesystems in one shot.
//   1. ripgrep's crates: SIMD regex, mmap, context windowing, encodings, binary heuristics.
//   2. eza-style views (pfind's own reimplementation): git badges, LOC lexer, trees, long table.
//   3. pfind: hybrid RRF fusion, term coverage ranking, snippet matching, presets.
// First time here? Run:  pfind --doctor
// Agents: --json gives one ranked object, no colour, no prose.

mod archive;
mod args;
mod data;
mod json;
mod meta;
mod timefmt;
mod zipread;
mod rank;
mod render;
mod search;
mod util;
mod walk;

use args::Args;
use render::Ctx;
use std::io::{IsTerminal, Read};
use std::path::PathBuf;
use std::time::Instant;
use walk::WalkSpec;

pub const VERSION: &str = "4.0.0";

struct Presets {
    roots: Vec<(String, PathBuf)>,
    chroma: Option<PathBuf>,
}

fn preset_file() -> PathBuf {
    if cfg!(windows) {
        if let Some(app) = std::env::var_os("APPDATA") {
            let win = PathBuf::from(app).join("pfind").join("presets.json");
            if win.is_file() {
                return win;
            }
        }
    }
    let base = std::env::var_os("XDG_CONFIG_HOME")
        .filter(|v| !v.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| util::home_dir().unwrap_or_default().join(".config"));
    base.join("pfind").join("presets.json")
}

fn expandvars(s: &str) -> String {
    // os.path.expandvars: $VAR, ${VAR} and, on Windows, %VAR%
    let mut out = s.to_string();
    let rx = regex::Regex::new(r"\$(\w+)|\$\{([^}]*)\}|%(\w+)%").unwrap();
    out = rx
        .replace_all(&out, |c: &regex::Captures| {
            let name = c.get(1).or(c.get(2)).or(c.get(3)).unwrap().as_str();
            if c.get(3).is_some() && !cfg!(windows) {
                return c[0].to_string();
            }
            std::env::var(name).unwrap_or_else(|_| c[0].to_string())
        })
        .into_owned();
    out
}

fn load_presets() -> (Presets, String) {
    let mut roots: Vec<(String, PathBuf)> = Vec::new();
    let mut chroma = None;
    let mut collection = "core_memory".to_string();
    let pf = preset_file();
    match std::fs::read_to_string(&pf) {
        Ok(text) => match json::parse_object(&text) {
            Ok(v) => {
                // pfind.py: `if data.get(name)` (truthy), then str(value)
                let get = |k: &str| -> Option<String> {
                    match v.get(k) {
                        Some(json::Val::Str(s)) if !s.is_empty() => Some(s.clone()),
                        Some(json::Val::Other(s)) if s != "false" && s != "0" && s != "[]" && s != "{}" => Some(s.clone()),
                        _ => None,
                    }
                };
                for name in ["brain", "work", "src"] {
                    if let Some(s) = get(name) {
                        roots.push((name.into(), util::expanduser(&expandvars(&s))));
                    }
                }
                if let Some(s) = get("chroma") {
                    chroma = Some(util::expanduser(&expandvars(&s)));
                }
                if let Some(s) = get("collection") {
                    collection = s;
                }
            }
            Err(e) => errln!("pfind: ignoring {}: {}", pf.display(), e),
        },
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(e) => errln!("pfind: ignoring {}: {}", pf.display(), e),
    }
    for (name, env) in [("brain", "PFIND_BRAIN"), ("work", "PFIND_WORK"), ("src", "PFIND_SRC")] {
        if let Ok(v) = std::env::var(env) {
            if !v.is_empty() {
                roots.retain(|(n, _)| n != name);
                roots.push((name.into(), util::expanduser(&v)));
            }
        }
    }
    for env in ["PFIND_BRAIN_DB", "PFIND_CHROMA"] {
        if let Ok(v) = std::env::var(env) {
            if !v.is_empty() {
                chroma = Some(util::expanduser(&v));
            }
        }
    }
    if let Ok(v) = std::env::var("PFIND_COLLECTION") {
        if !v.is_empty() {
            collection = v;
        }
    }
    (Presets { roots, chroma }, collection)
}

fn preset_help() -> String {
    format!(
        "configure it in {} (e.g. {{\"brain\": \"~/notes\", \"work\": \"~/projects\", \"src\": \"~/src\", \"chroma\": \"~/notes/chroma_db\"}}) or set PFIND_BRAIN / PFIND_WORK / PFIND_SRC",
        preset_file().display()
    )
}

fn resolve_roots(paths: &[String], presets: &[&str], cfg: &Presets) -> Vec<PathBuf> {
    let mut roots: Vec<PathBuf> = Vec::new();
    for name in presets {
        match cfg.roots.iter().find(|(n, _)| n == name) {
            None => errln!("pfind: preset --{} is not configured on this machine; {}", name, preset_help()),
            Some((_, p)) => {
                if p.exists() {
                    roots.push(p.clone());
                } else {
                    errln!("pfind: preset --{} points at {}, which does not exist; {}", name, p.display(), preset_help());
                }
            }
        }
    }
    for raw in paths {
        let p = util::resolve(raw);
        if p.exists() {
            roots.push(p);
        } else {
            errln!("pfind: path does not exist: {}", p.display());
        }
    }
    if roots.is_empty() {
        roots.push(util::strip_verbatim(std::env::current_dir().unwrap_or_default()));
    }
    let mut out: Vec<PathBuf> = Vec::new();
    for r in roots {
        if !out.iter().any(|x| x == &r) {
            out.push(r);
        }
    }
    out
}

fn type_globs(a: &Args, types: &[(String, Vec<String>)]) -> Vec<String> {
    let mut g = Vec::new();
    let lookup = |t: &str| types.iter().find(|(k, _)| k == &t.to_lowercase()).map(|(_, v)| v.clone());
    for t in &a.types {
        match lookup(t) {
            Some(exts) => g.extend(exts.iter().map(|e| format!("*{}", e))),
            None => g.push(if t.starts_with('.') { format!("*{}", t) } else { format!("*.{}", t) }),
        }
    }
    for t in &a.types_not {
        match lookup(t) {
            Some(exts) => g.extend(exts.iter().map(|e| format!("!*{}", e))),
            None => g.push(if t.starts_with('.') { format!("!*{}", t) } else { format!("!*.{}", t) }),
        }
    }
    g
}

fn print_type_list(types: &[(String, Vec<String>)]) {
    outln!("Language types supported:");
    outln!();
    let mut sorted: Vec<&(String, Vec<String>)> = types.iter().collect();
    sorted.sort_by(|a, b| a.0.cmp(&b.0));
    for (lang, exts) in sorted {
        outln!("  {:<20} {}", lang, exts.join(", "));
    }
    outln!();
    outln!("Total: {} language types", types.len());
    outln!("Usage: pfind 'pattern' -t python -t rust");
}

#[cfg(windows)]
fn enable_vt() -> bool {
    use windows_sys::Win32::System::Console::*;
    unsafe {
        let h = GetStdHandle(STD_OUTPUT_HANDLE);
        let mut mode: u32 = 0;
        if GetConsoleMode(h, &mut mode) == 0 {
            return false;
        }
        SetConsoleMode(h, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING) != 0
    }
}
#[cfg(not(windows))]
fn enable_vt() -> bool {
    true
}

fn setup_console() -> bool {
    std::io::stdout().is_terminal() && enable_vt()
}

fn hostname() -> String {
    if let Ok(m) = std::env::var("PFIND_MACHINE") {
        if !m.is_empty() {
            return m;
        }
    }
    #[cfg(unix)]
    {
        let mut buf = [0u8; 256];
        if unsafe { libc::gethostname(buf.as_mut_ptr() as *mut libc::c_char, buf.len()) } == 0 {
            let n = buf.iter().position(|b| *b == 0).unwrap_or(buf.len());
            let s = String::from_utf8_lossy(&buf[..n]).into_owned();
            if !s.is_empty() {
                return s;
            }
        }
    }
    #[cfg(windows)]
    {
        if let Ok(n) = std::env::var("COMPUTERNAME") {
            if !n.is_empty() {
                return n;
            }
        }
    }
    "localhost".into()
}

fn resolve_hostname_bin(cmd: &str, default: &str) -> String {
    let parts: Vec<&str> = cmd.split_whitespace().collect();
    if parts.is_empty() {
        return default.to_string();
    }
    let prog = util::which(parts[0]).unwrap_or_else(|| PathBuf::from(parts[0]));
    match meta::run_capture(&prog, &parts[1..], None, std::time::Duration::from_secs(5)) {
        Some((0, out, _)) if !out.trim().is_empty() => out.trim().lines().next().unwrap_or("").trim().to_string(),
        Some(_) => {
            errln!("pfind: --hostname-bin {:?} produced no hostname; keeping {}", cmd, default);
            default.to_string()
        }
        None => {
            errln!("pfind: --hostname-bin failed (could not run {:?}); keeping {}", cmd, default);
            default.to_string()
        }
    }
}

fn doctor(as_json: bool) -> i32 {
    let pf = preset_file();
    let git = meta::git_bin();
    let python = util::which("python3").or_else(|| util::which("python"));
    let mut checks: Vec<(String, bool, String, String)> = vec![
        ("pfind engine".into(), true,
         format!("pfind.exe {} at {} (ripgrep's ignore/grep crates built in)", VERSION,
                 std::env::current_exe().map(|p| p.display().to_string()).unwrap_or_default()),
         String::new()),
        ("ripgrep (rg)".into(), true, "not needed: its search crates are inside pfind.exe".into(), String::new()),
        ("git".into(), git.is_some(),
         if git.is_some() { "found".into() } else { "not installed".into() },
         if git.is_some() { String::new() } else { "optional: only for --git status badges and --git-ignore.".into() }),
        ("Console text".into(), true, "UTF-8 (pfind.exe writes UTF-8 bytes)".into(),
         "any file content is safe to print.".into()),
        ("Presets file".into(), pf.is_file(), pf.display().to_string(),
         if pf.is_file() { String::new() } else { "optional: map --brain/--work/--src to your own folders.".into() }),
    ];
    checks.push(("Semantic search (--brain)".into(), false,
                 "not built into pfind.exe: the Chroma store is a Python library".into(),
                 match &python {
                     Some(_) => "pfind.exe searches --brain lexically; for semantic recall run pfind.py --brain with chromadb installed (pip install chromadb).".into(),
                     None => "pfind.exe searches --brain lexically; semantic recall needs Python 3 + chromadb and pfind.py.".into(),
                 }));
    checks.push(("PCRE2 (--pcre2)".into(), false, "not built in (C library)".into(),
                 "the default regex engine is used; it covers everything except look-around and backreferences.".into()));
    if as_json {
        let items: Vec<String> = checks
            .iter()
            .map(|(n, ok, d, adv)| {
                format!("  {{\n   \"name\": {},\n   \"ok\": {},\n   \"detail\": {},\n   \"advice\": {}\n  }}",
                        util::json_str(n), ok, util::json_str(d), util::json_str(adv))
            })
            .collect();
        outln!("{{\n \"checks\": [\n{}\n ],\n \"install_ripgrep\": []\n}}", items.join(",\n"));
        return 0;
    }
    let os = if cfg!(windows) { "Windows" } else if cfg!(target_os = "macos") { "darwin" } else { "linux" };
    outln!("pfind doctor - {}\n", os);
    for (name, ok, detail, advice) in &checks {
        outln!("  [{}] {:<26} {}", if *ok { "ok" } else { "--" }, name, detail);
        if !advice.is_empty() {
            outln!("       {}", advice);
        }
    }
    outln!("\nEverything pfind needs is here; nothing to install.");
    0
}

fn read_stdin() -> String {
    let mut buf = Vec::new();
    let _ = std::io::stdin().read_to_end(&mut buf);
    String::from_utf8_lossy(&buf).into_owned()
}

fn main() {
    let code = run();
    util::flush();
    std::process::exit(code);
}

fn run() -> i32 {
    let start = Instant::now();
    let (cfg, default_collection) = load_presets();
    let cpus = std::thread::available_parallelism().map(|n| n.get() as i64).unwrap_or(4);
    let argv: Vec<String> = std::env::args().skip(1).collect();
    let mut a = args::parse(argv, cpus);
    if a.collection == "core_memory" {
        a.collection = default_collection;
    }
    if a.help {
        util::out(args::HELP);
        return 0;
    }
    if a.version {
        outln!("pfind {}", VERSION);
        return 0;
    }
    if a.doctor {
        return doctor(a.json);
    }
    if a.query_file.is_some() || a.query.as_deref() == Some("-") {
        let mut q = if let Some(qf) = &a.query_file {
            if let Some(q) = a.query.take() {
                a.paths.insert(0, q);
            }
            match std::fs::read(qf) {
                Ok(b) => String::from_utf8_lossy(&b).into_owned(),
                Err(e) => {
                    errln!("pfind: error: FileNotFoundError: {}: '{}'", e, qf);
                    return 2;
                }
            }
        } else {
            read_stdin()
        };
        q = q.replace("\r\n", "\n");
        if q.ends_with('\n') && q.matches('\n').count() > 1 {
            q = q.trim_end_matches('\n').to_string();
        }
        if q.trim().is_empty() {
            args::fail("a query is needed: the query file or stdin was empty");
        }
        a.query = Some(q);
    }

    let mut types = data::language_types();
    for tc in &a.type_clears {
        let t = tc.to_lowercase();
        if t == "all" {
            types.clear();
        } else {
            types.retain(|(k, _)| *k != t);
        }
    }
    for ta in &a.type_adds {
        if let Some((name, globs)) = ta.split_once(':') {
            let exts: Vec<String> = globs
                .split(',')
                .map(|g| g.trim())
                .filter(|g| !g.is_empty())
                .map(|e| {
                    if e.starts_with('.') {
                        e.to_string()
                    } else if !e.starts_with('*') {
                        format!(".{}", e)
                    } else {
                        e.trim_start_matches('*').to_string()
                    }
                })
                .collect();
            let key = name.trim().to_lowercase();
            match types.iter_mut().find(|(k, _)| *k == key) {
                Some(slot) => slot.1 = exts,
                None => types.push((key, exts)),
            }
        }
    }
    if a.type_list {
        print_type_list(&types);
        return 0;
    }

    let colors = render::parse_color_specs(&a.color_specs);
    let mut host = hostname();
    if let Some(hb) = a.hostname_bin.clone() {
        host = resolve_hostname_bin(&hb, &host);
    }
    if a.changed {
        a.time = "changed".into();
    } else if a.accessed {
        a.time = "accessed".into();
    } else if a.created {
        a.time = "created".into();
    } else if a.modified {
        a.time = "modified".into();
    }
    if a.engine == "pcre2" {
        a.pcre2 = true;
    }
    if a.pcre2 && a.engine == "default" {
        a.engine = "pcre2".into();
    }
    if a.binary_units && a.metric_units {
        errln!("pfind: --binary-units and --metric-units conflict; using binary (1024)");
        a.metric_units = false;
    }
    if a.mmap && a.no_mmap {
        errln!("pfind: --mmap and --no-mmap conflict; letting ripgrep choose");
        a.mmap = false;
        a.no_mmap = false;
    }
    if a.stdin {
        let text = read_stdin();
        let extra: Vec<String> = text.lines().filter(|l| !l.trim().is_empty()).map(|l| l.to_string()).collect();
        if extra.is_empty() {
            errln!("pfind: --stdin given but no paths on stdin");
            return 1;
        }
        a.paths.extend(extra);
    }
    if a.pcre2 && !a.no_messages {
        errln!("pfind: --pcre2 is not built into pfind.exe (PCRE2 is a C library); using the default regex engine");
    }
    util::set_line_buffered(a.line_buffered);

    let mut use_color = if a.color == "never" || a.no_color {
        false
    } else if a.color == "always" || a.color == "ansi" {
        true
    } else {
        setup_console() && !a.no_color
    };
    if a.json {
        use_color = false;
    }
    if a.pretty {
        use_color = true;
        a.heading = true;
        a.line_number = true;
    }
    if a.no_git {
        a.git = false;
    }
    if a.unrestricted >= 1 {
        a.no_ignore = true;
    }
    if a.unrestricted >= 2 {
        a.hidden = true;
    }
    if a.unrestricted >= 3 {
        a.text = true;
    }
    if a.git_repos_no_status {
        a.git_repos = true;
    }
    let mut presets: Vec<&str> = ["brain", "work", "src"]
        .into_iter()
        .filter(|n| match *n {
            "brain" => a.brain,
            "work" => a.work,
            _ => a.src,
        })
        .collect();
    if a.all {
        presets = vec!["brain", "work", "src"];
    }

    let a = a; // frozen from here on
    let tg = type_globs(&a, &types);
    let list_files = |roots: &[PathBuf]| -> Vec<String> {
        let spec = WalkSpec { roots, ext_filter: a.ext.clone(), type_globs: tg.clone(), hidden: a.hidden,
                              no_ignore: a.no_ignore, max_depth: a.max_depth, args: &a };
        let mut files = walk::rg_list_files(&spec);
        for r in roots {
            let rs = r.to_string_lossy().into_owned();
            if r.is_file() && !files.contains(&rs) {
                files.push(rs);
            }
        }
        files
    };
    let mk_ctx = |roots: Vec<PathBuf>, git: Option<meta::GitStatusCache>| Ctx {
        a: &a,
        use_color,
        colors: colors.clone(),
        host: host.clone(),
        roots,
        git,
        dir_sizes: Default::default(),
        repo_cache: Default::default(),
    };

    if let Some(mode) = &a.code {
        let mut path_list = a.paths.clone();
        if let Some(q) = &a.query {
            path_list.insert(0, q.clone());
        }
        let roots = resolve_roots(&path_list, &presets, &cfg);
        let files = list_files(&roots);
        render::render_code_summary(&files, use_color, mode);
        return 0;
    }

    if a.files {
        let mut path_list = a.paths.clone();
        if let Some(q) = &a.query {
            path_list.insert(0, q.clone());
        }
        let roots = resolve_roots(&path_list, &presets, &cfg);
        let files = list_files(&roots);
        let shown: &[String] = if a.limit > 0 { &files[..files.len().min(a.limit as usize)] } else { &files };
        for f in shown {
            outln!("{}", f);
        }
        if files.len() > shown.len() {
            outln!("\u{2026} and {} more file(s) not shown \u{2014} this list is INCOMPLETE; narrow it with path, glob or type)", files.len() - shown.len());
        } else if shown.len() > 1 && (!a.globs.is_empty() || !a.iglobs.is_empty()) {
            outln!("[{} files found; this list is complete]", shown.len());
        }
        return 0;
    }

    let mut a_paths = a.paths.clone();
    let mut query = a.query.clone();
    let mut is_exploratory = query.as_deref().map_or(true, |q| q.is_empty() || q == ".");
    if !is_exploratory
        && (a.tree || a.long || a.grid || a.oneline || a.recurse || a.treat_dirs_as_files)
        && a_paths.is_empty()
        && util::exists(query.as_deref().unwrap())
    {
        a_paths = vec![query.take().unwrap()];
        is_exploratory = true;
    }

    if is_exploratory {
        let roots = resolve_roots(&a_paths, &presets, &cfg);
        let git = if a.git { Some(meta::GitStatusCache::new(&roots)) } else { None };
        let mut ctx = mk_ctx(roots.clone(), git);
        if a.treat_dirs_as_files {
            let all: Vec<String> = roots.iter().map(|r| r.to_string_lossy().into_owned()).collect();
            let limit = if a.limit > 0 { a.limit as usize } else { 100 };
            render::render_long_table(&mut ctx, &all[..all.len().min(limit)], &Default::default());
            return 0;
        }
        let mut all = if a.only_dirs {
            let spec = WalkSpec { roots: &roots, ext_filter: a.ext.clone(), type_globs: tg.clone(), hidden: a.hidden,
                                  no_ignore: a.no_ignore, max_depth: a.max_depth, args: &a };
            walk::py_walk(&spec, true, a.hidden || a.almost_all).into_iter().filter(|f| util::is_dir(f)).collect()
        } else {
            list_files(&roots)
        };
        if a.only_files {
            all.retain(|f| util::is_file(f) || (a.show_symlinks && util::is_link(f)));
        }
        if a.no_symlinks {
            all.retain(|f| !util::is_link(f));
        }
        if let Some(ig) = &a.ignore_glob {
            let pats: Vec<regex::Regex> = ig
                .split('|')
                .filter_map(|p| regex::Regex::new(&format!("^(?:{})", p.replace('*', ".*"))).ok())
                .collect();
            all.retain(|f| !pats.iter().any(|r| r.is_match(util::basename(f))));
        }
        if a.git_dirty {
            if let Some(g) = &ctx.git {
                all.retain(|f| g.get(f).is_some());
            }
        }
        let tree_limit = if a.limit > 0 { a.limit } else { 200 };
        if a.tree && all.len() as i64 > tree_limit && a.level.is_none() && !a.only_dirs && !a.only_files {
            render::render_folder_overview(&all, &roots, tree_limit);
            return 0;
        }
        let sort_field = a.sortr.as_deref().or(a.sort.as_deref());
        let reverse = a.sortr.is_some() || a.reverse;
        let mut all = rank::sort_files(all, sort_field, reverse, a.dirs_first, a.dirs_last);
        let limit = if a.limit > 0 { a.limit as usize } else if a.tree { 200 } else { 100 };
        all.truncate(limit);
        if a.recurse {
            for f in &all {
                let is_dir = util::is_dir(f);
                let icon = if a.icons { format!("{} ", data::icon_for(f, is_dir)) } else { String::new() };
                let dn = if is_dir { util::c("1;34", f, use_color) } else { util::c("1;32", f, use_color) };
                if a.null {
                    util::out(&format!("{}{}\0", icon, dn));
                } else {
                    outln!("{}{}", icon, dn);
                }
            }
        } else if a.grid {
            render::render_grid(&ctx, &all);
        } else if a.oneline {
            render::render_oneline(&ctx, &all);
        } else if a.tree {
            render::render_tree_view(&ctx, &all, &Default::default());
        } else {
            render::render_long_table(&mut ctx, &all, &Default::default());
        }
        return 0;
    }

    let roots = resolve_roots(&a_paths, &presets, &cfg);
    let git = if a.git { Some(meta::GitStatusCache::new(&roots)) } else { None };

    let mut patterns: Vec<String> = Vec::new();
    if let Some(q) = &query {
        if !q.is_empty() {
            patterns.push(q.clone());
        }
    }
    patterns.extend(a.patterns.iter().cloned());
    if let Some(pf) = &a.pattern_file {
        match std::fs::read(pf) {
            Ok(b) => {
                let text = String::from_utf8_lossy(&b).into_owned();
                patterns.extend(text.lines().map(|l| l.trim().to_string()).filter(|l| !l.is_empty()));
            }
            Err(e) => {
                errln!("pfind: error reading pattern file: {}", e);
                return 1;
            }
        }
    }
    let mut use_regex_flag = a.regex;
    let query: String = if patterns.len() > 1 {
        let q = patterns
            .iter()
            .map(|p| format!("({})", if a.regex { p.clone() } else { rank::re_escape(p) }))
            .collect::<Vec<_>>()
            .join("|");
        use_regex_flag = true;
        q
    } else if let Some(p) = patterns.first() {
        p.clone()
    } else {
        errln!("pfind: no query specified");
        return 1;
    };

    let snippet_mode = a.exact || a.loose || query.contains('\n');
    let do_names = !a.content && !snippet_mode;
    let do_content = !a.names || snippet_mode;
    let (pattern, use_regex, use_fixed, multiline) = if a.loose {
        (rank::build_loose_regex(&query), true, false, true)
    } else if snippet_mode {
        (rank::build_exact_regex(&query), true, false, true)
    } else {
        (query.clone(), use_regex_flag, a.fixed_strings || !use_regex_flag, a.multiline || a.multiline_dotall)
    };

    let mut name_ranked: Vec<String> = Vec::new();
    if do_names {
        let files = list_files(&roots);
        match rank::match_names(&query, &files, use_regex, a.ignore_case, a.fuzzy, &roots, a.case_sensitive) {
            Ok(scored) => name_ranked = scored.into_iter().map(|(p, _)| p).collect(),
            Err(e) => {
                errln!("pfind: error: error: {}", e.lines().last().unwrap_or(&e));
                return 2;
            }
        }
    }

    let mut hits = search::Hits::new();
    if do_content {
        let ctx_b = if a.context != 0 { a.context } else { a.before_context };
        let ctx_a = if a.context != 0 { a.context } else { a.after_context };
        let spec = WalkSpec { roots: &roots, ext_filter: a.ext.clone(), type_globs: tg.clone(), hidden: a.hidden,
                              no_ignore: a.no_ignore, max_depth: a.max_depth, args: &a };
        let q = search::Query { pattern: &pattern, regex: use_regex, fixed: use_fixed, multiline, ctx_before: ctx_b, ctx_after: ctx_a };
        hits = search::content(&spec, &q, a.max.max(0) as usize, a.timeout);
    }

    let mut content_ranked: Vec<(&String, &search::Hit)> = hits.iter().collect();
    content_ranked.sort_by(|x, y| {
        y.1.terms.len().cmp(&x.1.terms.len()).then(y.1.count.cmp(&x.1.count)).then_with(|| x.0.cmp(y.0))
    });
    let content_ranked: Vec<String> = content_ranked.into_iter().map(|(p, _)| p.clone()).collect();

    let mut brain_note = false;
    if a.brain {
        // The Chroma "Second Brain" seam is a Python library; the exe cannot open it.
        brain_note = true;
        let store = match &cfg.chroma {
            Some(p) => format!(" (store {})", p.display()),
            None => String::new(),
        };
        errln!("pfind: --brain semantic recall needs Python + chromadb{}; pfind.exe searched the brain folder lexically only. Run pfind.py --brain for semantic recall (see pfind --doctor).", store);
    }
    let _ = brain_note;

    let mut git_ranked: Vec<String> = Vec::new();
    if a.git_dirty || a.dirty_boost {
        if let Some(g) = &git {
            let mut cands: Vec<String> = name_ranked.iter().chain(content_ranked.iter()).cloned().collect();
            cands.sort();
            cands.dedup();
            git_ranked = cands.into_iter().filter(|p| g.get(p).is_some()).collect();
        }
    }
    let mut recency_ranked: Vec<String> = Vec::new();
    if a.recency_boost {
        let mut cands: Vec<String> = name_ranked.iter().chain(content_ranked.iter()).cloned().collect();
        cands.sort();
        cands.dedup();
        let mut keyed: Vec<(f64, String)> = cands
            .into_iter()
            .map(|p| {
                let k = std::fs::metadata(&p).ok().and_then(|m| util::systime_secs(m.modified())).map(|t| -t).unwrap_or(0.0);
                (k, p)
            })
            .collect();
        keyed.sort_by(|x, y| x.0.partial_cmp(&y.0).unwrap_or(std::cmp::Ordering::Equal));
        recency_ranked = keyed.into_iter().map(|(_, p)| p).collect();
    }

    let mut rankings: Vec<(&str, Vec<String>)> = Vec::new();
    if !name_ranked.is_empty() {
        rankings.push(("name", name_ranked));
    }
    if !content_ranked.is_empty() {
        rankings.push(("content", content_ranked));
    }
    if !git_ranked.is_empty() {
        rankings.push(("git", git_ranked));
    }
    if !recency_ranked.is_empty() {
        rankings.push(("recency", recency_ranked));
    }
    if rankings.is_empty() {
        if a.json {
            render::render_json(&Vec::new(), &hits, 0);
        } else if !a.quiet && !a.no_messages {
            errln!("no matches.");
        }
        return 1;
    }

    let mut fused = rank::rrf_fuse(&rankings, a.rrf_k);
    if a.sort.is_some() || a.sortr.is_some() || a.dirs_first || a.dirs_last {
        let (field, reverse) = if a.sort.is_some() || a.sortr.is_some() {
            (a.sortr.as_deref().or(a.sort.as_deref()), a.sortr.is_some() || a.reverse)
        } else {
            (None, false)
        };
        let paths: Vec<String> = fused.iter().map(|f| f.0.clone()).collect();
        let sorted = rank::sort_files(paths, field, reverse, a.dirs_first, a.dirs_last);
        let mut by: std::collections::HashMap<String, (f64, Vec<String>)> =
            fused.into_iter().map(|(p, s, l)| (p, (s, l))).collect();
        fused = sorted.into_iter().filter_map(|p| by.remove(&p).map(|(s, l)| (p, s, l))).collect();
    }

    if a.json {
        render::render_json(&fused, &hits, a.limit);
        return 0;
    }
    let mut ctx = mk_ctx(roots, git);
    let code = render::render(&mut ctx, &fused, &hits, Some(&pattern));

    if a.stats {
        let total: u64 = hits.values().map(|h| h.count).sum();
        errln!("\n{}", util::c("1;37", "Search Statistics:", use_color));
        errln!("  Duration:          {:.3}s", start.elapsed().as_secs_f64());
        errln!("  Files with hits:   {}", hits.len());
        errln!("  Total match count: {}", total);
        errln!("  Engine:            pfind.exe {} (ripgrep crates, native)", VERSION);
    }
    code
}
