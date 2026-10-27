// SPDX-License-Identifier: AGPL-3.0-or-later
//
// File metadata for the eza-style views (long table, grid, tree): stat in the
// shape pfind.py saw from os.lstat on each platform, permissions, sizes,
// timestamps, git status (by running git, as pfind.py and eza's --git do),
// and the comment-aware LOC counter. These are pfind.py's own
// reimplementations, ported; no eza source is copied (see LICENSES.md).

use crate::util::{self, which};
use std::collections::HashMap;
use std::io::{Read, Write};
use std::path::Path;
use std::process::{Command, Stdio};
use std::sync::OnceLock;
use std::time::{Duration, Instant};

pub const S_IFMT: u32 = 0o170000;
pub const S_IFDIR: u32 = 0o040000;
pub const S_IFREG: u32 = 0o100000;
pub const S_IFLNK: u32 = 0o120000;
pub const S_IFIFO: u32 = 0o010000;
pub const S_IFSOCK: u32 = 0o140000;

#[derive(Clone, Default)]
#[allow(dead_code)] // ino/nlink are read on unix only
pub struct Stat {
    pub mode: u32,
    pub ino: u64,
    pub nlink: u64,
    pub uid: u32,
    pub gid: u32,
    pub size: u64,
    pub blocks: u64,
    pub atime: f64,
    pub mtime: f64,
    pub ctime: f64,
    pub birthtime: f64,
}

impl Stat {
    pub fn is_dir(&self) -> bool {
        self.mode & S_IFMT == S_IFDIR
    }
    pub fn is_link(&self) -> bool {
        self.mode & S_IFMT == S_IFLNK
    }
    pub fn is_reg(&self) -> bool {
        self.mode & S_IFMT == S_IFREG
    }
}

#[cfg(unix)]
fn from_meta(_p: &Path, m: &std::fs::Metadata) -> Stat {
    use std::os::unix::fs::MetadataExt;
    let birth = if cfg!(target_os = "macos") {
        util::systime_secs(m.created()).unwrap_or(m.ctime() as f64)
    } else {
        m.ctime() as f64 + m.ctime_nsec() as f64 / 1e9
    };
    Stat {
        mode: m.mode(),
        ino: m.ino(),
        nlink: m.nlink(),
        uid: m.uid(),
        gid: m.gid(),
        size: m.size(),
        blocks: m.blocks(),
        atime: m.atime() as f64 + m.atime_nsec() as f64 / 1e9,
        mtime: m.mtime() as f64 + m.mtime_nsec() as f64 / 1e9,
        ctime: m.ctime() as f64 + m.ctime_nsec() as f64 / 1e9,
        birthtime: birth,
    }
}

#[cfg(windows)]
fn from_meta(p: &Path, m: &std::fs::Metadata) -> Stat {
    use std::os::windows::fs::MetadataExt;
    // Python's os.stat on Windows: mode from the attributes, 0o111 for
    // directories and for .exe/.bat/.cmd/.com, uid/gid 0, ctime = creation.
    let attrs = m.file_attributes();
    let mut mode: u32;
    if m.file_type().is_symlink() {
        mode = S_IFLNK;
    } else if attrs & 0x10 != 0 {
        mode = S_IFDIR | 0o111;
    } else {
        mode = S_IFREG;
    }
    mode |= if attrs & 0x1 != 0 { 0o444 } else { 0o666 };
    if mode & S_IFMT == S_IFREG {
        let ext = util::splitext_ext(&p.to_string_lossy()).to_lowercase();
        if matches!(ext.as_str(), ".exe" | ".bat" | ".cmd" | ".com") {
            mode |= 0o111;
        }
    }
    let ft = |t: u64| -> f64 { (t as f64 - 116444736000000000.0) / 1e7 };
    let created = ft(m.creation_time());
    Stat {
        mode,
        ino: 0,
        nlink: 1,
        uid: 0,
        gid: 0,
        size: m.file_size(),
        blocks: 0,
        atime: ft(m.last_access_time()),
        mtime: ft(m.last_write_time()),
        ctime: created,
        birthtime: created,
    }
}

/// os.lstat / os.stat (with --dereference).
pub fn stat(path: &str, dereference: bool) -> std::io::Result<Stat> {
    let p = Path::new(path);
    if dereference {
        if let Ok(m) = std::fs::metadata(p) {
            return Ok(from_meta(p, &m));
        }
    }
    let m = std::fs::symlink_metadata(p)?;
    Ok(from_meta(p, &m))
}

/// Inode and link count. On Windows these need the file handle (what Python's
/// os.lstat reads), so they are fetched only when the --inode/--links columns ask.
#[cfg(windows)]
pub fn ino_nlink(path: &str, st: &Stat) -> (u64, u64) {
    use std::os::windows::ffi::OsStrExt;
    use windows_sys::Win32::Foundation::{CloseHandle, INVALID_HANDLE_VALUE};
    use windows_sys::Win32::Storage::FileSystem::*;
    let _ = st;
    let wide: Vec<u16> = std::ffi::OsStr::new(path).encode_wide().chain(Some(0)).collect();
    unsafe {
        let h = CreateFileW(wide.as_ptr(), 0, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
                            std::ptr::null(), OPEN_EXISTING,
                            FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT, std::ptr::null_mut());
        if h == INVALID_HANDLE_VALUE {
            return (0, 1);
        }
        let mut info: BY_HANDLE_FILE_INFORMATION = std::mem::zeroed();
        let ok = GetFileInformationByHandle(h, &mut info);
        CloseHandle(h);
        if ok == 0 {
            return (0, 1);
        }
        (((info.nFileIndexHigh as u64) << 32) | info.nFileIndexLow as u64, info.nNumberOfLinks as u64)
    }
}
#[cfg(not(windows))]
pub fn ino_nlink(_path: &str, st: &Stat) -> (u64, u64) {
    (st.ino, st.nlink)
}

pub fn format_size(size: u64, binary: bool) -> String {
    let base = if binary { 1024.0 } else { 1000.0 };
    let suffixes: [&str; 5] = if binary { ["B", "KiB", "MiB", "GiB", "TiB"] } else { ["B", "KB", "MB", "GB", "TB"] };
    let mut val = size as f64;
    for (i, s) in suffixes.iter().enumerate() {
        if val < base || i == suffixes.len() - 1 {
            if *s == "B" {
                return format!("{:>5} B", val as i64);
            }
            return format!("{:>5.1} {}", val, s);
        }
        val /= base;
    }
    format!("{:.1} B", val)
}

pub fn format_permissions(mode: u32, octal: bool) -> String {
    if octal {
        return format!("{:04o}", mode & 0o7777);
    }
    let mut s = String::with_capacity(10);
    s.push(if mode & S_IFMT == S_IFDIR { 'd' } else { '-' });
    let bits = [(0o400, 'r'), (0o200, 'w'), (0o100, 'x'), (0o040, 'r'), (0o020, 'w'), (0o010, 'x'),
                (0o004, 'r'), (0o002, 'w'), (0o001, 'x')];
    for (b, ch) in bits {
        s.push(if mode & b != 0 { ch } else { '-' });
    }
    s
}

use crate::timefmt::strftime;

pub fn format_timestamp(ts: f64, style: &str) -> String {
    let dt = crate::timefmt::local(ts);
    if let Some(f) = style.strip_prefix('+') {
        return strftime(&dt, f).unwrap_or_else(|| strftime(&dt, "%Y-%m-%d %H:%M").unwrap());
    }
    match style {
        "iso" => return strftime(&dt, "%Y-%m-%d %H:%M:%S").unwrap(),
        "long-iso" => return strftime(&dt, "%Y-%m-%d %H:%M").unwrap(),
        "full-iso" => return strftime(&dt, "%Y-%m-%d %H:%M:%S.%f %z").unwrap(),
        _ => {}
    }
    let secs = util::now_secs() - ts;
    if secs < 0.0 {
        return strftime(&dt, "%Y-%m-%d").unwrap();
    }
    let s = secs as i64;
    if secs < 60.0 {
        format!("{}s ago", s)
    } else if secs < 3600.0 {
        format!("{}m ago", s / 60)
    } else if secs < 86400.0 {
        format!("{}h ago", s / 3600)
    } else if secs < 86400.0 * 30.0 {
        format!("{}d ago", s / 86400)
    } else if secs < 86400.0 * 365.0 {
        format!("{}mo ago", s / (86400 * 30))
    } else {
        format!("{}y ago", s / (86400 * 365))
    }
}

pub fn pick_timestamp(st: &Stat, field: &str) -> f64 {
    match field {
        "accessed" => st.atime,
        "changed" => st.ctime,
        "created" => st.birthtime,
        _ => st.mtime,
    }
}

pub fn classify_indicator(path: &str, st: Option<&Stat>) -> &'static str {
    let owned;
    let st = match st {
        Some(s) => s,
        None => match stat(path, false) {
            Ok(s) => {
                owned = s;
                &owned
            }
            Err(_) => return "",
        },
    };
    match st.mode & S_IFMT {
        S_IFDIR => "/",
        S_IFLNK => "@",
        S_IFIFO => "|",
        S_IFSOCK => "=",
        _ if st.mode & 0o111 != 0 => "*",
        _ => "",
    }
}

pub fn quote_name(name: &str, enabled: bool) -> String {
    const NEED: &str = " \t\n\r\x0b\x0c'\"\\$`|&;<>()*?[]{}!#~";
    if !enabled || !name.chars().any(|c| NEED.contains(c) || c.is_whitespace()) {
        return name.to_string();
    }
    if name.contains('"') {
        format!("'{}'", name)
    } else {
        format!("\"{}\"", name)
    }
}

pub fn short_nix(path: &str, enabled: bool) -> String {
    if !enabled {
        return path.to_string();
    }
    static RX: OnceLock<regex::Regex> = OnceLock::new();
    let rx = RX.get_or_init(|| regex::Regex::new(r"(/nix/store/)([a-z0-9]{32})-").unwrap());
    rx.replace_all(path, "${1}\u{2026}-").into_owned()
}

/// Run a program with captured output and a timeout. None when it cannot start
/// or overruns (the child is killed).
pub fn run_capture(prog: &Path, args: &[&str], input: Option<&str>, timeout: Duration) -> Option<(i32, String, String)> {
    let mut cmd = Command::new(prog);
    cmd.args(args).stdout(Stdio::piped()).stderr(Stdio::piped());
    cmd.stdin(if input.is_some() { Stdio::piped() } else { Stdio::null() });
    let mut child = cmd.spawn().ok()?;
    if let Some(inp) = input {
        let mut si = child.stdin.take()?;
        let data = inp.to_string();
        std::thread::spawn(move || {
            let _ = si.write_all(data.as_bytes());
        });
    }
    let mut so = child.stdout.take()?;
    let mut se = child.stderr.take()?;
    let t1 = std::thread::spawn(move || {
        let mut b = Vec::new();
        let _ = so.read_to_end(&mut b);
        b
    });
    let t2 = std::thread::spawn(move || {
        let mut b = Vec::new();
        let _ = se.read_to_end(&mut b);
        b
    });
    let start = Instant::now();
    let status = loop {
        match child.try_wait() {
            Ok(Some(s)) => break s,
            Ok(None) => {
                if start.elapsed() > timeout {
                    let _ = child.kill();
                    let _ = child.wait();
                    return None;
                }
                std::thread::sleep(Duration::from_millis(5));
            }
            Err(_) => return None,
        }
    };
    let out = t1.join().ok()?;
    let err = t2.join().ok()?;
    Some((status.code().unwrap_or(-1), String::from_utf8_lossy(&out).into_owned(), String::from_utf8_lossy(&err).into_owned()))
}

pub fn git_bin() -> Option<&'static Path> {
    static GIT: OnceLock<Option<std::path::PathBuf>> = OnceLock::new();
    GIT.get_or_init(|| which("git")).as_deref()
}

fn git(args: &[&str], input: Option<&str>, secs: u64) -> Option<(i32, String)> {
    let g = git_bin()?;
    run_capture(g, args, input, Duration::from_secs(secs)).map(|(c, o, _)| (c, o))
}

/// Path(repo) / rel with Python's separator normalisation.
fn join_native(repo: &str, rel: &str) -> String {
    if cfg!(windows) {
        let r = repo.replace('/', "\\");
        format!("{}\\{}", r.trim_end_matches('\\'), rel.replace('/', "\\"))
    } else {
        format!("{}/{}", repo.trim_end_matches('/'), rel)
    }
}

pub struct GitStatusCache {
    map: HashMap<String, String>,
}

impl GitStatusCache {
    pub fn new(roots: &[std::path::PathBuf]) -> GitStatusCache {
        let mut map = HashMap::new();
        if git_bin().is_none() {
            return GitStatusCache { map };
        }
        let mut repos: Vec<String> = Vec::new();
        for r in roots {
            let rs = r.to_string_lossy();
            if let Some((0, out)) = git(&["-C", &rs, "rev-parse", "--show-toplevel"], None, 5) {
                let t = out.trim().to_string();
                if !t.is_empty() && !repos.contains(&t) {
                    repos.push(t);
                }
            }
        }
        for repo in repos {
            if let Some((0, out)) = git(&["-C", &repo, "status", "--porcelain=v1", "-z", "-uall"], None, 10) {
                for entry in out.split('\0') {
                    if entry.chars().count() >= 4 {
                        let st: String = entry.chars().take(2).collect::<String>().trim().to_string();
                        let rel: String = entry.chars().skip(3).collect();
                        map.insert(join_native(&repo, &rel), st);
                    }
                }
            }
        }
        GitStatusCache { map }
    }

    pub fn get(&self, path: &str) -> Option<&str> {
        self.map.get(&util::abspath_str(path)).map(|s| s.as_str())
    }
}

pub fn repo_summary(path: &str, with_status: bool, cache: &mut HashMap<(String, bool), Option<String>>) -> Option<String> {
    if git_bin().is_none() || !util::is_dir(path) {
        return None;
    }
    let key = (util::abspath_str(path), with_status);
    if let Some(v) = cache.get(&key) {
        return v.clone();
    }
    let mut result = None;
    if let Some((0, top)) = git(&["-C", path, "rev-parse", "--show-toplevel"], None, 5) {
        if util::abspath_str(top.trim()) == util::abspath_str(path) {
            let name = match git(&["-C", path, "rev-parse", "--abbrev-ref", "HEAD"], None, 5) {
                Some((0, b)) => b.trim().to_string(),
                _ => "?".to_string(),
            };
            if !with_status {
                result = Some(name);
            } else {
                let state = match git(&["-C", path, "status", "--porcelain"], None, 10) {
                    Some((0, o)) if !o.trim().is_empty() => "dirty",
                    _ => "clean",
                };
                result = Some(format!("{}|{}", name, state));
            }
        }
    }
    cache.insert(key, result.clone());
    result
}

/// Drop what git would ignore (git check-ignore), for the os.walk listings.
pub fn git_ignored_filter(paths: Vec<String>) -> Vec<String> {
    if git_bin().is_none() || paths.is_empty() {
        return paths;
    }
    let mut by_repo: Vec<(String, Vec<String>)> = Vec::new();
    for p in &paths {
        let d = if util::is_dir(p) {
            p.clone()
        } else {
            Path::new(p).parent().map(|x| x.to_string_lossy().into_owned()).filter(|s| !s.is_empty()).unwrap_or(".".into())
        };
        if let Some((0, top)) = git(&["-C", &d, "rev-parse", "--show-toplevel"], None, 5) {
            let t = top.trim().to_string();
            if t.is_empty() {
                continue;
            }
            match by_repo.iter_mut().find(|(r, _)| *r == t) {
                Some((_, g)) => g.push(p.clone()),
                None => by_repo.push((t, vec![p.clone()])),
            }
        }
    }
    let mut ignored = std::collections::HashSet::new();
    for (repo, group) in by_repo {
        if let Some((code, out)) = git(&["-C", &repo, "check-ignore", "--stdin"], Some(&group.join("\n")), 30) {
            if code == 0 || code == 1 {
                for l in out.lines() {
                    if !l.is_empty() {
                        ignored.insert(l.to_string());
                    }
                }
            }
        }
    }
    paths.into_iter().filter(|p| !ignored.contains(p)).collect()
}

#[derive(Default, Clone, Copy)]
pub struct Loc {
    pub lines: i64,
    pub code: i64,
    pub comments: i64,
    pub blanks: i64,
}
impl Loc {
    pub fn add(&mut self, o: &Loc) {
        self.lines += o.lines;
        self.code += o.code;
        self.comments += o.comments;
        self.blanks += o.blanks;
    }
}

/// bytes.decode("utf-8", errors="ignore")
pub fn decode_ignore(b: &[u8]) -> String {
    let mut s = String::with_capacity(b.len());
    for chunk in b.utf8_chunks() {
        s.push_str(chunk.valid());
    }
    s
}

/// Python text-mode readlines(): universal newlines, '\n' kept.
pub fn universal_lines(text: &str) -> Vec<String> {
    let t = text.replace("\r\n", "\n").replace('\r', "\n");
    let mut out: Vec<String> = t.split_inclusive('\n').map(|s| s.to_string()).collect();
    if out.last().map(|s| s.is_empty()).unwrap_or(false) {
        out.pop();
    }
    out
}

pub fn classify_file_loc(path: &str) -> Option<(&'static str, Loc)> {
    let syn = crate::data::syntax_for(path)?;
    let mut f = std::fs::File::open(path).ok()?;
    let mut buf = Vec::new();
    f.read_to_end(&mut buf).ok()?;
    let text = decode_ignore(&buf);
    let lines = universal_lines(&text);
    let mut counts = Loc { lines: lines.len() as i64, ..Default::default() };
    let mut closer: Option<&str> = None;
    for line in &lines {
        let mut rest: &str = line.trim();
        if rest.is_empty() {
            counts.blanks += 1;
            continue;
        }
        let (mut has_code, mut has_comment) = (false, false);
        while !rest.is_empty() {
            if let Some(cl) = closer {
                has_comment = true;
                if let Some(idx) = rest.find(cl) {
                    rest = rest[idx + cl.len()..].trim();
                    closer = None;
                    continue;
                }
                break;
            }
            if syn.line.iter().any(|lc| rest.starts_with(lc)) {
                has_comment = true;
                break;
            }
            if let Some((op, cl)) = syn.block.iter().find(|(op, _)| rest.starts_with(op)) {
                has_comment = true;
                closer = Some(cl);
                rest = rest[op.len()..].trim();
                continue;
            }
            has_code = true;
            let mut it = rest.chars();
            it.next();
            rest = it.as_str().trim();
        }
        if has_code {
            counts.code += 1;
        } else if has_comment {
            counts.comments += 1;
        } else {
            counts.blanks += 1;
        }
    }
    Some((syn.name, counts))
}

pub fn dir_total_size(path: &str, cache: &mut HashMap<String, u64>) -> u64 {
    if let Some(v) = cache.get(path) {
        return *v;
    }
    let mut total = 0u64;
    let mut stack = vec![std::path::PathBuf::from(path)];
    while let Some(d) = stack.pop() {
        if let Ok(rd) = std::fs::read_dir(&d) {
            for e in rd.flatten() {
                let p = e.path();
                let is_link = e.file_type().map(|t| t.is_symlink()).unwrap_or(false);
                // os.walk: a link to a folder is a folder name, never descended or counted
                if p.is_dir() {
                    if !is_link {
                        stack.push(p);
                    }
                } else if let Ok(m) = std::fs::symlink_metadata(&p) {
                    total += m.len();
                }
            }
        }
    }
    cache.insert(path.to_string(), total);
    total
}

#[cfg(unix)]
pub fn user_group(uid: u32, gid: u32) -> (String, String) {
    unsafe {
        let pw = libc::getpwuid(uid);
        let u = if pw.is_null() { uid.to_string() } else { std::ffi::CStr::from_ptr((*pw).pw_name).to_string_lossy().into_owned() };
        let gr = libc::getgrgid(gid);
        let g = if gr.is_null() { gid.to_string() } else { std::ffi::CStr::from_ptr((*gr).gr_name).to_string_lossy().into_owned() };
        (u, g)
    }
}
#[cfg(not(unix))]
pub fn user_group(uid: u32, gid: u32) -> (String, String) {
    // pfind.py on Windows has no pwd/grp module: it printed the numbers (0).
    (uid.to_string(), gid.to_string())
}

#[cfg(target_os = "linux")]
pub fn xattrs(path: &str) -> Vec<(String, usize)> {
    use std::ffi::CString;
    let c = match CString::new(path) {
        Ok(c) => c,
        Err(_) => return vec![],
    };
    unsafe {
        let n = libc::llistxattr(c.as_ptr(), std::ptr::null_mut(), 0);
        if n <= 0 {
            return vec![];
        }
        let mut buf = vec![0u8; n as usize];
        let n = libc::llistxattr(c.as_ptr(), buf.as_mut_ptr() as *mut libc::c_char, buf.len());
        if n <= 0 {
            return vec![];
        }
        buf.truncate(n as usize);
        buf.split(|b| *b == 0)
            .filter(|s| !s.is_empty())
            .map(|name| {
                let cn = CString::new(name).unwrap();
                let sz = libc::lgetxattr(c.as_ptr(), cn.as_ptr(), std::ptr::null_mut(), 0);
                (String::from_utf8_lossy(name).into_owned(), sz.max(0) as usize)
            })
            .collect()
    }
}
#[cfg(not(target_os = "linux"))]
pub fn xattrs(_path: &str) -> Vec<(String, usize)> {
    vec![]
}

pub fn file_flags(path: &str) -> String {
    if let Some(lsattr) = which("lsattr") {
        if let Some((0, out, _)) = run_capture(&lsattr, &["-d", "--", path], None, Duration::from_secs(5)) {
            if let Some(first) = out.split_whitespace().next() {
                let a = first.replace('-', "");
                return if a.is_empty() { "-".into() } else { a };
            }
        }
    }
    "-".into()
}

pub fn mount_info(path: &str) -> Option<(String, String)> {
    static MOUNTS: OnceLock<HashMap<String, (String, String)>> = OnceLock::new();
    let m = MOUNTS.get_or_init(|| {
        let mut m = HashMap::new();
        for src in ["/proc/self/mounts", "/etc/mtab"] {
            if let Ok(t) = std::fs::read_to_string(src) {
                for line in t.lines() {
                    let parts: Vec<&str> = line.split_whitespace().collect();
                    if parts.len() >= 3 {
                        m.insert(parts[1].replace("\\040", " "), (parts[0].to_string(), parts[2].to_string()));
                    }
                }
                break;
            }
        }
        m
    });
    m.get(&util::abspath_str(path)).cloned()
}
