// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Small helpers shared by every module: the output stream, ANSI colour,
// Python-compatible number formatting (so pfind.exe prints what pfind.py
// printed, byte for byte), JSON string escaping, and path normalisation.

use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;

static LINE_BUFFERED: AtomicBool = AtomicBool::new(false);
static OUT: Mutex<Option<io::BufWriter<io::Stdout>>> = Mutex::new(None);

pub fn set_line_buffered(on: bool) {
    LINE_BUFFERED.store(on, Ordering::Relaxed);
}

/// Write raw text to stdout (buffered). A closed pipe ends the program quietly.
pub fn out(s: &str) {
    let mut g = OUT.lock().unwrap();
    if g.is_none() {
        *g = Some(io::BufWriter::with_capacity(1 << 16, io::stdout()));
    }
    let w = g.as_mut().unwrap();
    if w.write_all(s.as_bytes()).is_err() {
        std::process::exit(0);
    }
    if LINE_BUFFERED.load(Ordering::Relaxed) {
        let _ = w.flush();
    }
}

pub fn flush() {
    let mut g = OUT.lock().unwrap();
    if let Some(w) = g.as_mut() {
        let _ = w.flush();
    }
}

#[macro_export]
macro_rules! outln {
    () => { $crate::util::out("\n") };
    ($($t:tt)*) => {{ let mut s = format!($($t)*); s.push('\n'); $crate::util::out(&s) }};
}

#[macro_export]
macro_rules! errln {
    ($($t:tt)*) => {{ $crate::util::flush(); eprintln!($($t)*); }};
}

pub fn c(code: &str, s: &str, use_color: bool) -> String {
    if use_color {
        format!("\x1b[{}m{}\x1b[0m", code, s)
    } else {
        s.to_string()
    }
}

/// Python's f"{n:,}" for integers.
pub fn commas(n: i64) -> String {
    let neg = n < 0;
    let digits = n.unsigned_abs().to_string();
    let mut out = String::new();
    for (i, ch) in digits.chars().enumerate() {
        if i > 0 && (digits.len() - i) % 3 == 0 {
            out.push(',');
        }
        out.push(ch);
    }
    if neg {
        format!("-{}", out)
    } else {
        out
    }
}

/// Python's repr() of a float: shortest round-trip digits, exponent form
/// outside 1e-4 <= |x| < 1e16, always with a decimal point or exponent.
pub fn py_float_repr(x: f64) -> String {
    if x == 0.0 {
        return if x.is_sign_negative() { "-0.0".into() } else { "0.0".into() };
    }
    if !x.is_finite() {
        return if x.is_nan() { "NaN".into() } else if x > 0.0 { "Infinity".into() } else { "-Infinity".into() };
    }
    let e = format!("{:e}", x); // e.g. "3.279e-2", "-1e5"
    let (mant, exp) = e.split_once('e').unwrap();
    let exp: i32 = exp.parse().unwrap();
    let neg = mant.starts_with('-');
    let digits: String = mant.chars().filter(|c| c.is_ascii_digit()).collect();
    let sign = if neg { "-" } else { "" };
    if (-4..16).contains(&exp) {
        let nd = digits.len() as i32;
        let point = exp + 1; // digits before the decimal point
        let s = if point <= 0 {
            format!("0.{}{}", "0".repeat((-point) as usize), digits)
        } else if point >= nd {
            format!("{}{}.0", digits, "0".repeat((point - nd) as usize))
        } else {
            format!("{}.{}", &digits[..point as usize], &digits[point as usize..])
        };
        format!("{}{}", sign, s)
    } else {
        let m = if digits.len() > 1 {
            format!("{}.{}", &digits[..1], &digits[1..])
        } else {
            digits.clone()
        };
        format!("{}{}e{}{:02}", sign, m, if exp < 0 { "-" } else { "+" }, exp.abs())
    }
}

/// Python's round(x, n) for floats: correctly rounded through decimal.
pub fn py_round(x: f64, n: usize) -> f64 {
    format!("{:.*}", n, x).parse().unwrap_or(x)
}

/// json.dumps(..., ensure_ascii=False) string escaping.
pub fn json_str(s: &str) -> String {
    let mut o = String::with_capacity(s.len() + 2);
    o.push('"');
    for ch in s.chars() {
        match ch {
            '"' => o.push_str("\\\""),
            '\\' => o.push_str("\\\\"),
            '\n' => o.push_str("\\n"),
            '\r' => o.push_str("\\r"),
            '\t' => o.push_str("\\t"),
            '\u{08}' => o.push_str("\\b"),
            '\u{0c}' => o.push_str("\\f"),
            c if (c as u32) < 0x20 => o.push_str(&format!("\\u{:04x}", c as u32)),
            c => o.push(c),
        }
    }
    o.push('"');
    o
}

/// Truncate to at most n characters (Python s[:n]).
pub fn take_chars(s: &str, n: usize) -> &str {
    match s.char_indices().nth(n) {
        Some((i, _)) => &s[..i],
        None => s,
    }
}

pub fn char_len(s: &str) -> usize {
    s.chars().count()
}

pub const SEP: char = std::path::MAIN_SEPARATOR;

/// Strip the \\?\ verbatim prefix std::fs::canonicalize adds on Windows, so
/// paths look like Python's Path.resolve() output (C:\Users\...).
pub fn strip_verbatim(p: PathBuf) -> PathBuf {
    let s = p.to_string_lossy();
    if let Some(rest) = s.strip_prefix(r"\\?\UNC\") {
        return PathBuf::from(format!(r"\\{}", rest));
    }
    if let Some(rest) = s.strip_prefix(r"\\?\") {
        return PathBuf::from(rest.to_string());
    }
    p
}

/// Python Path(raw).expanduser().resolve(): absolute, symlinks resolved, and a
/// path that does not exist still made absolute.
pub fn resolve(raw: &str) -> PathBuf {
    let expanded = expanduser(raw);
    match std::fs::canonicalize(&expanded) {
        Ok(p) => strip_verbatim(p),
        Err(_) => abspath(&expanded),
    }
}

pub fn expanduser(raw: &str) -> PathBuf {
    if raw == "~" || raw.starts_with("~/") || raw.starts_with("~\\") {
        if let Some(h) = home_dir() {
            return h.join(&raw[raw.len().min(2)..]);
        }
    }
    PathBuf::from(raw)
}

pub fn home_dir() -> Option<PathBuf> {
    #[cfg(windows)]
    {
        std::env::var_os("USERPROFILE").map(PathBuf::from)
    }
    #[cfg(not(windows))]
    {
        std::env::var_os("HOME").map(PathBuf::from)
    }
}

/// os.path.abspath: join with cwd and normalise . and .. lexically.
pub fn abspath(p: &Path) -> PathBuf {
    let joined = if p.is_absolute() {
        p.to_path_buf()
    } else {
        std::env::current_dir().unwrap_or_default().join(p)
    };
    normpath(&joined)
}

pub fn normpath(p: &Path) -> PathBuf {
    use std::path::Component;
    let mut out = PathBuf::new();
    for comp in p.components() {
        match comp {
            Component::CurDir => {}
            Component::ParentDir => {
                out.pop();
            }
            other => out.push(other.as_os_str()),
        }
    }
    out
}

pub fn abspath_str(p: &str) -> String {
    abspath(Path::new(p)).to_string_lossy().into_owned()
}

pub fn basename(p: &str) -> &str {
    let t = p.trim_end_matches(['/', '\\']);
    let t = if t.is_empty() { p } else { t };
    match t.rfind(['/', '\\']) {
        Some(i) => &p[i + 1..],
        None => p,
    }
}

/// os.path.splitext(name)[1]: the extension with its dot, "" for dotfiles.
pub fn splitext_ext(name: &str) -> &str {
    let base = basename(name);
    let stripped = base.trim_start_matches('.');
    let lead = base.len() - stripped.len();
    match stripped.rfind('.') {
        Some(i) => &base[lead + i..],
        None => "",
    }
}

/// Path(name).suffix
pub fn suffix(name: &str) -> &str {
    let base = basename(name);
    if base.ends_with('.') {
        return "";
    }
    match base.rfind('.') {
        Some(0) | None => "",
        Some(i) => &base[i..],
    }
}

pub fn is_dir(p: &str) -> bool {
    Path::new(p).is_dir()
}
pub fn is_file(p: &str) -> bool {
    Path::new(p).is_file()
}
pub fn is_link(p: &str) -> bool {
    std::fs::symlink_metadata(p).map(|m| m.file_type().is_symlink()).unwrap_or(false)
}
pub fn exists(p: &str) -> bool {
    Path::new(p).exists()
}

/// Locate a program on PATH (with PATHEXT on Windows), never in the current
/// folder: a planted git.exe next to the files being searched must not run.
pub fn which(prog: &str) -> Option<PathBuf> {
    let path = std::env::var_os("PATH")?;
    #[cfg(windows)]
    let exts: Vec<String> = std::env::var("PATHEXT")
        .unwrap_or_else(|_| ".COM;.EXE;.BAT;.CMD".into())
        .split(';')
        .filter(|e| !e.is_empty())
        .map(|e| e.to_ascii_lowercase())
        .collect();
    for dir in std::env::split_paths(&path) {
        if dir.as_os_str().is_empty() || !dir.is_absolute() {
            continue;
        }
        let cand = dir.join(prog);
        #[cfg(windows)]
        {
            for e in &exts {
                let p = dir.join(format!("{}{}", prog, e));
                if p.is_file() {
                    return Some(p);
                }
            }
        }
        #[cfg(not(windows))]
        if cand.is_file() {
            return Some(cand);
        }
        let _ = cand;
    }
    None
}

/// fnmatch.fnmatchcase(name, pat): *, ?, [seq], [!seq]; '*' crosses '/'.
pub fn fnmatchcase(name: &str, pat: &str) -> bool {
    let n: Vec<char> = name.chars().collect();
    let p: Vec<char> = pat.chars().collect();
    fn class(p: &[char], i: usize, ch: char) -> Option<(bool, usize)> {
        // returns (matched, index after ']') or None when '[' is literal
        let mut j = i + 1;
        let mut negate = false;
        if j < p.len() && p[j] == '!' {
            negate = true;
            j += 1;
        }
        let start = j;
        let mut matched = false;
        while j < p.len() && (p[j] != ']' || j == start) {
            if j + 2 < p.len() && p[j + 1] == '-' && p[j + 2] != ']' {
                if p[j] <= ch && ch <= p[j + 2] {
                    matched = true;
                }
                j += 3;
            } else {
                if p[j] == ch {
                    matched = true;
                }
                j += 1;
            }
        }
        if j >= p.len() {
            return None;
        }
        Some((matched != negate, j + 1))
    }
    fn rec(n: &[char], p: &[char], ni: usize, pi: usize) -> bool {
        let (mut ni, mut pi) = (ni, pi);
        while pi < p.len() {
            match p[pi] {
                '*' => {
                    while pi < p.len() && p[pi] == '*' {
                        pi += 1;
                    }
                    if pi == p.len() {
                        return true;
                    }
                    for k in ni..=n.len() {
                        if rec(n, p, k, pi) {
                            return true;
                        }
                    }
                    return false;
                }
                '?' => {
                    if ni >= n.len() {
                        return false;
                    }
                    ni += 1;
                    pi += 1;
                }
                '[' => {
                    if ni >= n.len() {
                        return false;
                    }
                    match class(p, pi, n[ni]) {
                        Some((true, next)) => {
                            ni += 1;
                            pi = next;
                        }
                        Some((false, _)) => return false,
                        None => {
                            if n[ni] != '[' {
                                return false;
                            }
                            ni += 1;
                            pi += 1;
                        }
                    }
                }
                ch => {
                    if ni >= n.len() || n[ni] != ch {
                        return false;
                    }
                    ni += 1;
                    pi += 1;
                }
            }
        }
        ni == n.len()
    }
    rec(&n, &p, 0, 0)
}

/// fnmatch.fnmatch: os.path.normcase on both sides (case-folded and / -> \ on Windows).
pub fn fnmatch(name: &str, pat: &str) -> bool {
    if cfg!(windows) {
        let n = name.to_lowercase().replace('/', "\\");
        let p = pat.to_lowercase().replace('/', "\\");
        fnmatchcase(&n, &p)
    } else {
        fnmatchcase(name, pat)
    }
}

pub fn now_secs() -> f64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs_f64())
        .unwrap_or(0.0)
}

pub fn systime_secs(t: std::io::Result<std::time::SystemTime>) -> Option<f64> {
    let t = t.ok()?;
    Some(match t.duration_since(std::time::UNIX_EPOCH) {
        Ok(d) => d.as_secs_f64(),
        Err(e) => -e.duration().as_secs_f64(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn float_repr_matches_python() {
        assert_eq!(py_float_repr(0.03279), "0.03279");
        assert_eq!(py_float_repr(1e-05), "1e-05");
        assert_eq!(py_float_repr(0.0001), "0.0001");
        assert_eq!(py_float_repr(2.0), "2.0");
        assert_eq!(py_float_repr(0.0), "0.0");
        assert_eq!(py_float_repr(1e16), "1e+16");
        assert_eq!(py_round(0.0327868852459, 5), 0.03279);
    }
    #[test]
    fn commas_like_python() {
        assert_eq!(commas(1234567), "1,234,567");
        assert_eq!(commas(12), "12");
        assert_eq!(commas(-1000), "-1,000");
    }
    #[test]
    fn fnmatch_basics() {
        assert!(fnmatchcase("a.min.js", "*.min.js"));
        assert!(fnmatchcase("obj-x86", "obj-*"));
        assert!(!fnmatchcase("objx", "obj-*"));
        assert!(fnmatchcase("b", "[abc]"));
        assert!(fnmatchcase("d", "[!abc]"));
        assert!(fnmatchcase("x/y.go", "*.go"));
    }
    #[test]
    fn ext_helpers() {
        assert_eq!(splitext_ext("a/b.tar.gz"), ".gz");
        assert_eq!(splitext_ext(".bashrc"), "");
        assert_eq!(suffix("x.py"), ".py");
        assert_eq!(suffix(".bashrc"), "");
    }
}
